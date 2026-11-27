package parse_test

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestLiteralTabs: a literal tab inside a plain or quoted string is CL021
// (info) at the whole string; the escape \t, block scalars and comments are
// not.
func TestLiteralTabs(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		wantPosition model.Position
	}{
		{name: "double-quoted", source: "a: \"x\ty\"\n",
			wantPosition: model.Position{Line: 1, Column: 4, EndLine: 1, EndColumn: 9}},
		{name: "single-quoted", source: "a: 'x\ty'\n",
			wantPosition: model.Position{Line: 1, Column: 4, EndLine: 1, EndColumn: 9}},
		{name: "plain", source: "a: x\ty  # comment\n",
			wantPosition: model.Position{Line: 1, Column: 4, EndLine: 1, EndColumn: 7}},
		{name: "in a flow list", source: "a: [\"x\ty\", z]\n",
			wantPosition: model.Position{Line: 1, Column: 5, EndLine: 1, EndColumn: 10}},
		{name: "key", source: "\"x\ty\": z\n",
			wantPosition: model.Position{Line: 1, Column: 1, EndLine: 1, EndColumn: 6}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, findings := parse.Parse([]byte(test.source))
			tabFindings := findingsOf(findings, "CL021")
			if len(tabFindings) != 1 {
				t.Fatalf("got %d CL021 findings, want 1: %+v", len(tabFindings), findings)
			}
			finding := tabFindings[0]
			if finding.Severity != analysis.SeverityInfo {
				t.Errorf("severity = %v, want info", finding.Severity)
			}
			if finding.Position != test.wantPosition {
				t.Errorf("position = %+v, want %+v", finding.Position, test.wantPosition)
			}
			if finding.Message != "string contains a literal tab character" {
				t.Errorf("message = %q", finding.Message)
			}
			if finding.Hint != `if the tab is intended, write it as \t in double quotes` {
				t.Errorf("hint = %q", finding.Hint)
			}
		})
	}
}

// findingsOf returns the findings of one rule.
func findingsOf(findings []analysis.Finding, rule string) []analysis.Finding {
	var matching []analysis.Finding
	for _, finding := range findings {
		if finding.Rule == rule {
			matching = append(matching, finding)
		}
	}
	return matching
}

// TestNoLiteralTabs: tabs that are no part of a plain or quoted string.
func TestNoLiteralTabs(t *testing.T) {
	sources := map[string]string{
		"escape":                 "a: \"x\\ty\"\n",
		"literal block":          "a: |\n  x\ty\n",
		"folded block":           "a: >\n  x\ty\n",
		"comment":                "a: b # x\ty\n",
		"after a plain value":    "a: b\t\n",
		"after a quoted value":   "a: \"b\"\t\n",
		"before a comment":       "a: b\t# comment\n",
		"block with indentation": "a: |\n  x\n  \ty\n",
		"block after a comment":  "a: | # script\n  x\ty\n",
	}
	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			_, findings := parse.Parse([]byte(source))
			if tabFindings := findingsOf(findings, "CL021"); len(tabFindings) != 0 {
				t.Errorf("unexpected CL021: %+v", tabFindings)
			}
		})
	}
}
