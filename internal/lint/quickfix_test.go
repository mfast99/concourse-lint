package lint_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/lint"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestQuickfixSpec runs the cases of spec/editor/quickfix/: the finding at
// the cursor carries the expected replacement for its range, or none.
func TestQuickfixSpec(t *testing.T) {
	for _, file := range spectest.EditorFiles(t, "quickfix") {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		findings := lint.Check(file, source)
		for _, editorCase := range spectest.ParseEditorCases(source) {
			where := spectest.Name(file) + ":" + strconv.Itoa(editorCase.MarkerLine)
			atCursor := findingsAt(findings, editorCase.Cursor)
			if editorCase.Kind != "fix" || len(atCursor) == 0 {
				t.Errorf("%s: want a fix case on a finding, got kind %q and %d findings", where, editorCase.Kind, len(atCursor))
				continue
			}
			want := ""
			if editorCase.Arguments != "none" {
				want, _ = strconv.Unquote(editorCase.Arguments)
			}
			if got := atCursor[0].Suggestion; got != want {
				t.Errorf("%s: suggestion of %s = %q, want %q", where, atCursor[0].Rule, got, want)
			}
		}
	}
}

// findingsAt returns the findings whose range contains the cursor.
func findingsAt(findings []analysis.Finding, cursor model.Position) []analysis.Finding {
	var found []analysis.Finding
	for _, finding := range findings {
		position := finding.Position
		if position.Line == cursor.Line && position.Column <= cursor.Column && cursor.Column <= position.EndColumn {
			found = append(found, finding)
		}
	}
	return found
}

// TestSuggestionKeepsQuotes: the suggestion is the exact text for the
// finding's range, so a quoted typo is replaced inside its quotes (spec
// README, CLI contract). Columns count characters (ä).
func TestSuggestionKeepsQuotes(t *testing.T) {
	tests := []struct{ resource, written, want string }{
		{"repo", "rep", "repo"},
		{"repo", `"rep"`, `"repo"`},
		{"repo", "'rep'", "'repo'"},
		{"däta", `"dät"`, `"däta"`},
	}
	for _, test := range tests {
		source := "resources:\n- name: " + test.resource + "\n  type: git\n  source: {uri: https://github.com/example/app.git}\n" +
			"jobs:\n- name: build\n  plan:\n  - get: " + test.written + "\n"
		got := ""
		for _, finding := range lint.Check("pipeline.yml", []byte(source)) {
			if finding.Rule == "CL002" {
				got = finding.Suggestion
			}
		}
		if got != test.want {
			t.Errorf("get: %s: suggestion = %q, want %q", test.written, got, test.want)
		}
	}
}
