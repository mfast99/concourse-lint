package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

// latestProtocolVersion is the newest MCP version the server speaks; it is
// the answer when the client asks for one the server does not know.
const latestProtocolVersion = "2025-11-25"

// supportedProtocolVersions are the MCP versions the server speaks.
var supportedProtocolVersions = []string{latestProtocolVersion, "2025-06-18", "2025-03-26", "2024-11-05"}

// instructions tell the agent when and how to use the tools (initialize
// result, spec/README.md "MCP server contract").
const instructions = "concourse-lint checks Concourse CI pipelines (YAML) for errors, security issues and bad practice, offline. " +
	"Call check_pipeline after writing or changing a pipeline: pass the YAML as text, or paths of files or folders inside the working directory. " +
	"Fix findings with severity error first. " +
	"When the suggestion of a finding is not empty, it is the exact text that replaces the finding's range. " +
	"Ask explain_rule what a rule checks and how to fix it; list_rules lists all rules."

// Options configure a server.
type Options struct {
	// Version is sent to the client in serverInfo.
	Version string
	// Log receives messages about the session (stderr in the CLI). Nil: no
	// log.
	Log io.Writer
}

// Serve runs a server on the input and output streams until the input
// ends. It returns nil then, or the error of a failed read or write. Paths
// are relative to the working directory of the process, and only paths
// inside it are read.
func Serve(input io.Reader, output io.Writer, options Options) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return err
	}
	// Resolved, so that it compares with resolved paths (e.g. /tmp as a
	// symlink on macOS).
	if resolved, err := filepath.EvalSymlinks(workingDirectory); err == nil {
		workingDirectory = resolved
	}
	server := &server{connection: newConnection(input, output, maxLineSize), options: options, workingDirectory: workingDirectory}
	for {
		line, err := server.connection.read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if errors.Is(err, errLineTooLong) {
			server.log("dropping a message: %v", err)
			if err := server.connection.respondError(nil, &responseError{Code: codeParseError, Message: err.Error()}); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := server.handleLine(line); err != nil {
			return err
		}
	}
}

type server struct {
	connection *connection
	options    Options
	// protocolVersion is the version agreed in initialize; "" before.
	protocolVersion string
	// workingDirectory is absolute, with symlinks resolved.
	workingDirectory string
}

func (server *server) log(format string, arguments ...any) {
	if server.options.Log != nil {
		fmt.Fprintf(server.options.Log, "concourse-lint mcp: "+format+"\n", arguments...)
	}
}

func (server *server) initialized() bool {
	return server.protocolVersion != ""
}

// handleLine answers one line: requests get exactly one answer,
// notifications and responses from the client none.
func (server *server) handleLine(line []byte) error {
	if len(bytes.TrimSpace(line)) == 0 {
		return nil
	}
	message, failure := decode(line)
	if failure != nil {
		server.log("ignoring a message: %s", failure.Message)
		return server.connection.respondError(message.ID, failure)
	}
	if message.isResponse() || (!message.isRequest() && message.Method != "") {
		return nil
	}
	if message.Method == "" {
		return server.connection.respondError(message.ID, &responseError{Code: codeInvalidRequest, Message: "the message has no method"})
	}
	result, failure := server.handleRequest(message)
	if failure != nil {
		return server.connection.respondError(message.ID, failure)
	}
	return server.connection.respond(message.ID, result)
}

// handleRequest returns the result of a request or its error.
func (server *server) handleRequest(message incoming) (any, *responseError) {
	switch {
	case message.Method == "ping":
		return struct{}{}, nil
	case message.Method == "initialize":
		return server.initialize(message.Params)
	case !server.initialized():
		return nil, &responseError{Code: codeServerNotInitialized, Message: "the server is not initialized"}
	case message.Method == "tools/list":
		return server.listTools(), nil
	case message.Method == "tools/call":
		return server.callTool(message.Params)
	default:
		return nil, &responseError{Code: codeMethodNotFound, Message: "unknown method " + message.Method}
	}
}

type initializeParams struct {
	// A pointer, so that a missing version differs from an empty one.
	ProtocolVersion *string `json:"protocolVersion"`
}

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions"`
}

type serverCapabilities struct {
	Tools toolsCapability `json:"tools"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// initialize agrees on the protocol version: the client's when the server
// speaks it, else the latest.
func (server *server) initialize(rawParams json.RawMessage) (any, *responseError) {
	if server.initialized() {
		return nil, &responseError{Code: codeInvalidRequest, Message: "initialize was already called"}
	}
	var params initializeParams
	if err := json.Unmarshal(rawParams, &params); err != nil || params.ProtocolVersion == nil {
		return nil, &responseError{Code: codeInvalidParams, Message: "initialize needs params with a protocolVersion"}
	}
	server.protocolVersion = latestProtocolVersion
	if slices.Contains(supportedProtocolVersions, *params.ProtocolVersion) {
		server.protocolVersion = *params.ProtocolVersion
	}
	return initializeResult{
		ProtocolVersion: server.protocolVersion,
		Capabilities:    serverCapabilities{Tools: toolsCapability{ListChanged: false}},
		ServerInfo:      implementation{Name: "concourse-lint", Version: server.options.Version},
		Instructions:    instructions,
	}, nil
}
