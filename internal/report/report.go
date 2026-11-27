// Package report writes findings as text, JSON or SARIF. The format is
// defined in spec/README.md under "CLI contract".
//
// All writers have the same signature, so a front end can pick one by name.
package report

import (
	"encoding/json"
	"io"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// Result is everything one run produced, in the order of the command line.
type Result struct {
	Files []File
	// Inputs that could not be read. The other files were still checked.
	Errors []InputError
	// The version of concourse-lint, e.g. "v0.1.0" or "dev". Only SARIF
	// writes it.
	ToolVersion string
}

// File holds the findings of one checked input.
type File struct {
	// As given on the command line; StdinPath for "-".
	Path     string
	Findings []analysis.Finding
}

// InputError is an input that could not be read, e.g. a missing file.
type InputError struct {
	Path    string
	Message string
}

// StdinPath is the name under which input from stdin ("-") is reported.
// The angle brackets show that it is not a real file.
const StdinPath = "<stdin>"

// messageWithHint is the message followed by the hint in parentheses, or
// only the message when there is no hint.
func messageWithHint(finding analysis.Finding) string {
	if finding.Hint == "" {
		return finding.Message
	}
	return finding.Message + " (" + finding.Hint + ")"
}

// writeJSON writes document as JSON indented by two spaces, followed by a
// newline.
func writeJSON(writer io.Writer, document any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	// By default the encoder replaces <, > and & with Unicode escape
	// sequences for embedding in HTML, so "<stdin>" would not appear as
	// written.
	encoder.SetEscapeHTML(false)
	return encoder.Encode(document)
}
