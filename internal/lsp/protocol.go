package lsp

// The LSP types the server uses (LSP 3.17), only with the fields it needs.

// Position is a position in LSP: line and character from 0, the character
// counted in UTF-16 code units.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Diagnostic severities.
const (
	severityError       = 1
	severityWarning     = 2
	severityInformation = 3
)

type CodeDescription struct {
	Href string `json:"href"`
}

type Diagnostic struct {
	Range           Range            `json:"range"`
	Severity        int              `json:"severity"`
	Code            string           `json:"code,omitempty"`
	CodeDescription *CodeDescription `json:"codeDescription,omitempty"`
	Source          string           `json:"source"`
	Message         string           `json:"message"`
}

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// initializeParams holds the only client capability the server reads.
type initializeParams struct {
	Capabilities struct {
		Workspace struct {
			DidChangeWatchedFiles struct {
				DynamicRegistration bool `json:"dynamicRegistration"`
			} `json:"didChangeWatchedFiles"`
		} `json:"workspace"`
	} `json:"capabilities"`
}

type registrationParams struct {
	Registrations []registration `json:"registrations"`
}

type registration struct {
	ID              string                   `json:"id"`
	Method          string                   `json:"method"`
	RegisterOptions watchedFilesRegistration `json:"registerOptions"`
}

type watchedFilesRegistration struct {
	Watchers []fileSystemWatcher `json:"watchers"`
}

type fileSystemWatcher struct {
	GlobPattern string `json:"globPattern"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   serverInfo         `json:"serverInfo"`
}

type serverCapabilities struct {
	PositionEncoding   string            `json:"positionEncoding"`
	TextDocumentSync   textDocumentSync  `json:"textDocumentSync"`
	DefinitionProvider bool              `json:"definitionProvider"`
	ReferencesProvider bool              `json:"referencesProvider"`
	HoverProvider      bool              `json:"hoverProvider"`
	CodeActionProvider codeActionOptions `json:"codeActionProvider"`
	CompletionProvider completionOptions `json:"completionProvider"`
}

type completionOptions struct {
	// Typing one of these opens the completion list.
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// CompletionItemKind values of LSP.
const (
	completionKindProperty  = 10
	completionKindKeyword   = 14
	completionKindReference = 18
)

type completionItem struct {
	Label    string   `json:"label"`
	Kind     int      `json:"kind"`
	TextEdit TextEdit `json:"textEdit"`
}

type codeActionOptions struct {
	CodeActionKinds []string `json:"codeActionKinds"`
}

type codeActionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
}

// TextEdit replaces a range of a document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type workspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"`
}

type codeAction struct {
	Title       string        `json:"title"`
	Kind        string        `json:"kind"`
	Diagnostics []Diagnostic  `json:"diagnostics,omitempty"`
	IsPreferred bool          `json:"isPreferred,omitempty"`
	Edit        workspaceEdit `json:"edit"`
}

// markupContent is Markdown for the editor (hover).
type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type hover struct {
	Contents markupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// textDocumentSync: open and close notifications, and the full text on
// every change.
type textDocumentSync struct {
	OpenClose bool `json:"openClose"`
	// 1 = full text.
	Change int `json:"change"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// Location is a range in a document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// textDocumentPositionParams: a document and a position in it (definition,
// hover, completion).
type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type referenceParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type contentChange struct {
	Text string `json:"text"`
}

type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []contentChange                 `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}
