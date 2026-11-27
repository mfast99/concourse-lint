package lsp

import (
	"slices"
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/lint"
)

const codeActionQuickFix = "quickfix"

// codeAction answers textDocument/codeAction with quick fixes for the
// findings in the range: the suggestion of a
// finding ("did you mean"), and a suppression comment on the line or for
// the file. The findings are computed again, not taken from the client.
func (server *server) codeAction(message incoming) error {
	var params codeActionParams
	if !server.decode(message, &params) {
		return server.connection.respondError(message.ID, codeInvalidParams, "invalid params")
	}
	open, found := server.pipelineDocument(params.TextDocument.URI)
	if !found {
		return server.connection.respond(message.ID, nil)
	}
	findings, err := server.findings(open)
	if err != nil {
		return server.connection.respond(message.ID, nil)
	}
	index := newLineIndex(open.text)
	var actions []codeAction
	add := func(action codeAction) {
		if !slices.ContainsFunc(actions, func(existing codeAction) bool { return existing.Title == action.Title }) {
			actions = append(actions, action)
		}
	}
	for _, finding := range findings {
		findingRange := index.toRange(finding.Position)
		if !rangesTouch(findingRange, params.Range) {
			continue
		}
		diagnostic := toDiagnostic(finding, index)
		if finding.Suggestion != "" {
			add(changeAction(open, finding, findingRange, diagnostic, index))
		}
		if !lint.Suppressible(finding.Rule) {
			continue
		}
		if action, works := server.lineSuppression(open, finding, diagnostic, index); works {
			add(action)
		}
		add(fileSuppression(open, finding, diagnostic))
	}
	return server.connection.respond(message.ID, actions)
}

// changeAction replaces the finding's range with its suggestion, which is
// already the exact text for the range (with quotes where needed).
func changeAction(open document, finding analysis.Finding, findingRange Range, diagnostic Diagnostic, index lineIndex) codeAction {
	return codeAction{
		Title:       finding.SuggestionTitle(),
		Kind:        codeActionQuickFix,
		Diagnostics: []Diagnostic{diagnostic},
		IsPreferred: true,
		Edit:        singleEdit(open.uri, TextEdit{Range: findingRange, NewText: finding.Suggestion}),
	}
}

// lineSuppression adds the rule to an ignore comment on the line above, or
// inserts one there with the indentation of the finding's line. It is only
// offered when it really removes the finding: inside a block scalar (|, >)
// the new line would be text, not a comment.
func (server *server) lineSuppression(open document, finding analysis.Finding, diagnostic Diagnostic, index lineIndex) (codeAction, bool) {
	lineNumber := finding.Position.Line - 1
	if lineNumber < 0 || lineNumber >= len(index.lines) {
		return codeAction{}, false
	}
	var edit TextEdit
	findingLineAfter := finding.Position.Line
	if extended, found := extendIgnoreComment(index, lineNumber-1, finding.Rule); found {
		edit = extended
	} else {
		line := index.lines[lineNumber]
		indentation := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		start := Position{Line: lineNumber, Character: 0}
		edit = TextEdit{Range: Range{Start: start, End: start}, NewText: indentation + "# concourse-lint:ignore " + finding.Rule + lineBreakOf(open.text)}
		findingLineAfter++
	}
	fixed := open
	fixed.text = applyEdit(open.text, edit, index)
	remaining, err := server.findings(fixed)
	if err != nil || slices.ContainsFunc(remaining, func(after analysis.Finding) bool {
		return after.Rule == finding.Rule && after.Position.Line == findingLineAfter && after.Message == finding.Message
	}) {
		return codeAction{}, false
	}
	return codeAction{
		Title:       "Suppress " + finding.Rule + " on this line",
		Kind:        codeActionQuickFix,
		Diagnostics: []Diagnostic{diagnostic},
		Edit:        singleEdit(open.uri, edit),
	}, true
}

// extendIgnoreComment returns the edit that adds the rule to the ID list of
// an ignore comment that stands alone on the line, if there is one.
func extendIgnoreComment(index lineIndex, lineNumber int, rule string) (TextEdit, bool) {
	if lineNumber < 0 {
		return TextEdit{}, false
	}
	line := index.lines[lineNumber]
	const prefix = "# concourse-lint:ignore "
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, prefix) {
		return TextEdit{}, false
	}
	listStart := len(line) - len(trimmed) + len(prefix)
	listEnd := listStart + strings.IndexAny(line[listStart:]+" ", " \t")
	if slices.Contains(strings.Split(line[listStart:listEnd], ","), rule) {
		return TextEdit{}, false
	}
	insertAt := Position{Line: lineNumber, Character: utf16Length(line[:listEnd], -1)}
	return TextEdit{Range: Range{Start: insertAt, End: insertAt}, NewText: "," + rule}, true
}

// fileSuppression inserts an ignore-file comment as the first line.
func fileSuppression(open document, finding analysis.Finding, diagnostic Diagnostic) codeAction {
	start := Position{Line: 0, Character: 0}
	return codeAction{
		Title:       "Suppress " + finding.Rule + " in this file",
		Kind:        codeActionQuickFix,
		Diagnostics: []Diagnostic{diagnostic},
		Edit:        singleEdit(open.uri, TextEdit{Range: Range{Start: start, End: start}, NewText: "# concourse-lint:ignore-file " + finding.Rule + lineBreakOf(open.text)}),
	}
}

func singleEdit(uri string, edit TextEdit) workspaceEdit {
	return workspaceEdit{Changes: map[string][]TextEdit{uri: {edit}}}
}

func applyEdit(text string, edit TextEdit, index lineIndex) string {
	return text[:index.offset(edit.Range.Start)] + edit.NewText + text[index.offset(edit.Range.End):]
}

// lineBreakOf returns the line break the document uses.
func lineBreakOf(text string) string {
	if strings.Contains(text, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

// rangesTouch reports whether two ranges overlap or touch, e.g. a cursor
// (an empty range) at the end of a finding.
func rangesTouch(first, second Range) bool {
	return !positionBefore(first.End, second.Start) && !positionBefore(second.End, first.Start)
}

func positionBefore(first, second Position) bool {
	return first.Line < second.Line || (first.Line == second.Line && first.Character < second.Character)
}
