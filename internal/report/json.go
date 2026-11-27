package report

import (
	"io"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// The types below are the JSON contract from spec/README.md. They are
// separate from analysis.Finding on purpose (like an API DTO), so that a
// change in the core cannot change the output for AI agents by accident.
// Lowercase names keep them private to this package.

type jsonDocument struct {
	Findings []jsonFinding `json:"findings"`
	Errors   []jsonError   `json:"errors"`
}

type jsonFinding struct {
	File     string       `json:"file"`
	Rule     string       `json:"rule"`
	Severity string       `json:"severity"`
	Position jsonPosition `json:"pos"`
	Message  string       `json:"message"`
	// Always present, "" when there is no hint (no omitempty).
	Hint string `json:"hint"`
	// The exact text that replaces pos (a "did you mean" fix), always
	// present, "" when there is none.
	Suggestion string `json:"suggestion"`
	// Link to the rule page in docs/rules/.
	Docs string `json:"docs"`
}

// jsonPosition is where a finding starts and ends (right after its last
// character). A finding without an end has length zero: end = start.
type jsonPosition struct {
	Line      int `json:"line"`
	Column    int `json:"col"`
	EndLine   int `json:"end_line"`
	EndColumn int `json:"end_col"`
}

func toJSONPosition(position model.Position) jsonPosition {
	end := endOf(position)
	return jsonPosition{Line: position.Line, Column: position.Column, EndLine: end.Line, EndColumn: end.Column}
}

// endOf returns the end of a position, or its start when it has none.
func endOf(position model.Position) model.Position {
	if position.EndLine == 0 {
		return position.Start()
	}
	return model.Position{Line: position.EndLine, Column: position.EndColumn}
}

type jsonError struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

// JSON writes the result as the JSON document from the CLI contract: one
// flat list of findings over all files, plus the inputs that could not be
// read. Both lists are [] when empty, never null.
func JSON(writer io.Writer, result Result) error {
	// Starting from empty (non-nil) slices makes the encoder write [] instead
	// of null when nothing is appended.
	document := jsonDocument{Findings: []jsonFinding{}, Errors: []jsonError{}}
	for _, file := range result.Files {
		for _, finding := range file.Findings {
			document.Findings = append(document.Findings, jsonFinding{
				File:       file.Path,
				Rule:       finding.Rule,
				Severity:   string(finding.Severity),
				Position:   toJSONPosition(finding.Position),
				Message:    finding.Message,
				Hint:       finding.Hint,
				Suggestion: finding.Suggestion,
				Docs:       analysis.DocsURL(finding.Rule),
			})
		}
	}
	for _, inputError := range result.Errors {
		document.Errors = append(document.Errors, jsonError{File: inputError.Path, Message: inputError.Message})
	}
	return writeJSON(writer, document)
}
