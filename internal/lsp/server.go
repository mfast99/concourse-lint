package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/mfast99/concourse-lint/internal/model"
)

// Options configure a server.
type Options struct {
	// Delay between the last change of a document and its check. Zero checks
	// at once, in the handler (for tests).
	Delay time.Duration
	// Version is sent to the editor in serverInfo.
	Version string
	// Log receives messages for the editor's output panel (stderr in the
	// CLI). Nil: no log.
	Log io.Writer
}

// ErrExitWithoutShutdown is returned when the client ends the session with
// exit before shutdown, or closes the stream; the process then exits with 1
// (LSP specification).
var ErrExitWithoutShutdown = errors.New("exit without shutdown")

// Serve runs a server on the input and output streams until the client
// sends exit. It returns nil after shutdown and exit.
func Serve(input io.Reader, output io.Writer, options Options) error {
	server := &server{
		connection: newConnection(input, output),
		options:    options,
		documents:  map[string]*document{},
		timers:     map[string]*time.Timer{},
		configs:    newConfigCache(),
	}
	defer server.stopTimers()
	for {
		message, err := server.connection.read()
		if err == io.EOF {
			return ErrExitWithoutShutdown
		}
		if errors.Is(err, errMalformed) {
			server.log("ignoring a message: %v", err)
			if writeErr := server.connection.respondError(nil, codeParseError, err.Error()); writeErr != nil {
				return writeErr
			}
			continue
		}
		if err != nil {
			return err
		}
		if message.Method == "exit" {
			if server.isShutDown() {
				return nil
			}
			return ErrExitWithoutShutdown
		}
		if err := server.handle(message); err != nil {
			return err
		}
	}
}

type server struct {
	connection *connection
	options    Options

	// mutex guards everything below; the debounce timers run on their own
	// goroutines.
	mutex       sync.Mutex
	initialized bool
	shutDown    bool
	// watchConfigFiles: the client can register file watchers, so the
	// server asks it to watch the config files after initialized.
	watchConfigFiles bool
	documents        map[string]*document
	timers           map[string]*time.Timer
	configs          *configCache
}

// document is an open document.
type document struct {
	uri        string
	path       string
	languageID string
	version    int
	text       string
	// lastModel is the model of the last version without a syntax error,
	// for the names in completion while the YAML is broken.
	lastModel *model.Pipeline
}

func (server *server) log(format string, arguments ...any) {
	if server.options.Log != nil {
		fmt.Fprintf(server.options.Log, "concourse-lint: "+format+"\n", arguments...)
	}
}

func (server *server) isShutDown() bool {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return server.shutDown
}

// handle answers a request or processes a notification. Only an error
// writing to the client ends the server.
func (server *server) handle(message incoming) error {
	server.mutex.Lock()
	initialized, shutDown := server.initialized, server.shutDown
	server.mutex.Unlock()

	// A response of the client to a request of the server.
	if message.Method == "" {
		return nil
	}
	if message.isRequest() {
		switch {
		case message.Method == "initialize":
			return server.initialize(message)
		case !initialized:
			return server.connection.respondError(message.ID, codeServerNotInitialized, "the server is not initialized")
		case shutDown:
			return server.connection.respondError(message.ID, codeInvalidRequest, "the server is shut down")
		case message.Method == "shutdown":
			server.mutex.Lock()
			server.shutDown = true
			server.mutex.Unlock()
			server.stopTimers()
			return server.connection.respond(message.ID, nil)
		case message.Method == "textDocument/definition":
			return server.definition(message)
		case message.Method == "textDocument/references":
			return server.references(message)
		case message.Method == "textDocument/hover":
			return server.hover(message)
		case message.Method == "textDocument/codeAction":
			return server.codeAction(message)
		case message.Method == "textDocument/completion":
			return server.completion(message)
		}
		return server.connection.respondError(message.ID, codeMethodNotFound, "method not supported: "+message.Method)
	}

	// Notifications get no answer; before initialize and after shutdown
	// they are dropped.
	if !initialized || shutDown {
		return nil
	}
	switch message.Method {
	case "initialized":
		return server.registerWatchers()
	case "textDocument/didOpen":
		var params didOpenParams
		if server.decode(message, &params) {
			server.open(params.TextDocument)
		}
	case "textDocument/didChange":
		var params didChangeParams
		if server.decode(message, &params) && len(params.ContentChanges) > 0 {
			server.change(params.TextDocument, params.ContentChanges[len(params.ContentChanges)-1].Text)
		}
	case "textDocument/didClose":
		var params didCloseParams
		if server.decode(message, &params) {
			server.close(params.TextDocument.URI)
		}
	case "workspace/didChangeWatchedFiles":
		server.configFilesChanged()
	}
	return nil
}

func (server *server) decode(message incoming, params any) bool {
	if err := json.Unmarshal(message.Params, params); err != nil {
		server.log("%s: invalid params: %v", message.Method, err)
		return false
	}
	return true
}

func (server *server) initialize(message incoming) error {
	// Broken capabilities only mean that none are offered.
	var params initializeParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		server.log("initialize: ignoring the client capabilities: %v", err)
	}
	server.mutex.Lock()
	already := server.initialized
	server.initialized = true
	if !already {
		server.watchConfigFiles = params.Capabilities.Workspace.DidChangeWatchedFiles.DynamicRegistration
	}
	server.mutex.Unlock()
	if already {
		return server.connection.respondError(message.ID, codeInvalidRequest, "initialize was already called")
	}
	return server.connection.respond(message.ID, initializeResult{
		Capabilities: serverCapabilities{
			PositionEncoding:   "utf-16",
			TextDocumentSync:   textDocumentSync{OpenClose: true, Change: 1},
			DefinitionProvider: true,
			ReferencesProvider: true,
			HoverProvider:      true,
			CodeActionProvider: codeActionOptions{CodeActionKinds: []string{codeActionQuickFix}},
			CompletionProvider: completionOptions{},
		},
		ServerInfo: serverInfo{Name: "concourse-lint", Version: server.options.Version},
	})
}

// registerWatchers asks the client to report changes of the config files
// with workspace/didChangeWatchedFiles, if it can register file watchers.
// Two patterns instead of {yml,yaml}: not every client knows braces.
func (server *server) registerWatchers() error {
	server.mutex.Lock()
	watch := server.watchConfigFiles
	server.watchConfigFiles = false
	server.mutex.Unlock()
	if !watch {
		return nil
	}
	return server.connection.request("concourse-lint/config-files", "client/registerCapability", registrationParams{
		Registrations: []registration{{
			ID:     "concourse-lint/config-files",
			Method: "workspace/didChangeWatchedFiles",
			RegisterOptions: watchedFilesRegistration{Watchers: []fileSystemWatcher{
				{GlobPattern: "**/.concourse-lint.yml"},
				{GlobPattern: "**/.concourse-lint.yaml"},
			}},
		}},
	})
}

func (server *server) open(item textDocumentItem) {
	server.mutex.Lock()
	server.documents[item.URI] = &document{
		uri: item.URI, path: uriToPath(item.URI), languageID: item.LanguageID,
		version: item.Version, text: item.Text,
	}
	server.mutex.Unlock()
	server.schedule(item.URI)
}

func (server *server) change(identifier versionedTextDocumentIdentifier, text string) {
	server.mutex.Lock()
	open, isOpen := server.documents[identifier.URI]
	if isOpen {
		open.version, open.text = identifier.Version, text
	}
	server.mutex.Unlock()
	if isOpen {
		server.schedule(identifier.URI)
	}
}

func (server *server) close(uri string) {
	server.mutex.Lock()
	delete(server.documents, uri)
	if timer, found := server.timers[uri]; found {
		timer.Stop()
		delete(server.timers, uri)
	}
	server.mutex.Unlock()
	// A closed document shows no findings any more.
	server.publish(uri, nil, nil)
	server.recheckAfterConfigEdit(uri)
}

// schedule checks the document after the delay; a new change in between
// starts the delay again.
func (server *server) schedule(uri string) {
	if server.options.Delay <= 0 {
		server.check(uri)
		return
	}
	server.mutex.Lock()
	defer server.mutex.Unlock()
	if timer, found := server.timers[uri]; found {
		timer.Stop()
	}
	server.timers[uri] = time.AfterFunc(server.options.Delay, func() { server.check(uri) })
}

func (server *server) stopTimers() {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	for uri, timer := range server.timers {
		timer.Stop()
		delete(server.timers, uri)
	}
}

// publish sends the diagnostics of a document. version is nil for a
// document that is not open (e.g. a config file on disk).
func (server *server) publish(uri string, version *int, diagnostics []Diagnostic) {
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	err := server.connection.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI: uri, Version: version, Diagnostics: diagnostics,
	})
	if err != nil {
		server.log("publishing diagnostics: %v", err)
	}
}
