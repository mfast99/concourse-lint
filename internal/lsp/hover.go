package lsp

import (
	"fmt"
	"strings"

	"github.com/mfast99/concourse-lint/docs"
	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/lint"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// hover answers textDocument/hover: the rules of the findings at the
// position (from their page in docs/rules/), then the summary of the name
// there; null when there is neither.
func (server *server) hover(message incoming) error {
	var params textDocumentPositionParams
	if !server.decode(message, &params) {
		return server.connection.respondError(message.ID, codeInvalidParams, "invalid params")
	}
	open, found := server.pipelineDocument(params.TextDocument.URI)
	if !found {
		return server.connection.respond(message.ID, nil)
	}
	index := newLineIndex(open.text)
	cursor := cursorOf(index, params.Position)
	var parts []string
	var hoverRange *Range
	// A broken config file is shown as a diagnostic; the hover then only
	// describes names.
	findings, _ := server.findings(open)
	for _, finding := range findings {
		if !findingContains(finding.Position, cursor) {
			continue
		}
		parts = append(parts, ruleMarkdown(finding))
		if hoverRange == nil {
			findingRange := index.toRange(finding.Position)
			hoverRange = &findingRange
		}
	}
	pipeline, _ := lint.Parse([]byte(open.text))
	if summary, found := navigate.Hover(pipeline, cursor); found {
		parts = append(parts, summaryMarkdown(summary))
		summaryRange := index.toRange(summary.Position)
		hoverRange = &summaryRange
	}
	if len(parts) == 0 {
		return server.connection.respond(message.ID, nil)
	}
	return server.connection.respond(message.ID, hover{
		Contents: markupContent{Kind: "markdown", Value: strings.Join(parts, "\n\n---\n\n")},
		Range:    hoverRange,
	})
}

// findingContains reports whether the cursor is on the finding: from its
// start to its end (inclusive, where the cursor stands after the last
// character), or exactly at its start when it has no end.
func findingContains(position model.Position, cursor model.Position) bool {
	start, end := position.Start(), model.Position{Line: position.EndLine, Column: position.EndColumn}
	if position.EndLine == 0 {
		return cursor == start
	}
	afterStart := cursor.Line > start.Line || (cursor.Line == start.Line && cursor.Column >= start.Column)
	beforeEnd := cursor.Line < end.Line || (cursor.Line == end.Line && cursor.Column <= end.Column)
	return afterStart && beforeEnd
}

// ruleMarkdown explains the rule of a finding: title, what it checks, the
// fix and a link to its page.
func ruleMarkdown(finding analysis.Finding) string {
	page, found := docs.Rule(finding.Rule)
	if !found {
		return fmt.Sprintf("**%s**\n\n[Rule page](%s)", finding.Rule, analysis.DocsURL(finding.Rule))
	}
	return fmt.Sprintf("**%s** %s\n\n%s\n\n**Fix:** %s\n\n[Rule page](%s)",
		finding.Rule, page.Title, page.Checked, page.Fix, analysis.DocsURL(finding.Rule))
}

// summaryMarkdown shows a summary as a title and one line per fact.
func summaryMarkdown(summary navigate.Summary) string {
	lines := []string{fmt.Sprintf("**%s** `%s`", summary.Kind, summary.Name)}
	for _, fact := range summary.Facts {
		lines = append(lines, "- "+fact)
	}
	return strings.Join(lines, "\n")
}
