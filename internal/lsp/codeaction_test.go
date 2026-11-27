package lsp

import (
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/lint"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestQuickfixSpec runs the cases of spec/editor/quickfix/ through the
// protocol: a finding with a suggestion has the preferred quick fix
// 'Change to "x"' that replaces its range.
func TestQuickfixSpec(t *testing.T) {
	client := startServer(t, Options{})
	response := client.initialize()
	capabilities := response["result"].(map[string]any)["capabilities"].(map[string]any)
	if provider, _ := capabilities["codeActionProvider"].(map[string]any); provider == nil || !slices.Contains(provider["codeActionKinds"].([]any), any("quickfix")) {
		t.Errorf("capabilities = %v, want codeActionProvider with quickfix", capabilities)
	}
	for _, file := range spectest.EditorFiles(t, "quickfix") {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		uri := pathToURI(file)
		client.open(uri, languageIDPipeline, string(source))
		index := newLineIndex(string(source))
		for _, editorCase := range spectest.ParseEditorCases(source) {
			where := spectest.Name(file) + ":" + strconv.Itoa(editorCase.MarkerLine)
			cursor := index.toPosition(editorCase.Cursor.Line, editorCase.Cursor.Column)
			actions := codeActions(t, client, uri, Range{Start: cursor, End: cursor})
			change := findAction(actions, "Change to ")
			if editorCase.Arguments == "none" {
				if change != nil {
					t.Errorf("%s: got %q, want no change", where, change.Title)
				}
				continue
			}
			want, _ := strconv.Unquote(editorCase.Arguments)
			if change == nil {
				t.Errorf("%s: no 'Change to' action in %v", where, titles(actions))
				continue
			}
			if change.Title != "Change to "+strconv.Quote(want) || !change.IsPreferred || change.Kind != "quickfix" {
				t.Errorf("%s: action = %+v", where, change)
			}
			edits := change.Edit.Changes[uri]
			if len(edits) != 1 || edits[0].NewText != want {
				t.Errorf("%s: edits = %+v, want one edit to %q", where, edits, want)
			}
		}
	}
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

// TestQuickfixKeepsQuotes: a quoted typo is replaced inside its quotes.
func TestQuickfixKeepsQuotes(t *testing.T) {
	text := "resources:\n- name: repo\n  type: git\n  source: {uri: https://github.com/example/app.git}\njobs:\n- name: build\n  plan:\n  - get: \"rep\"\n"
	client, uri := openText(t, text)
	actions := codeActions(t, client, uri, Range{Start: Position{Line: 7, Character: 10}, End: Position{Line: 7, Character: 10}})
	change := findAction(actions, "Change to ")
	if change == nil {
		t.Fatalf("no change in %v", titles(actions))
	}
	if got := applyEdits(t, text, change.Edit.Changes[uri]); !strings.Contains(got, "  - get: \"repo\"\n") {
		t.Errorf("after the fix:\n%s", got)
	}
}

// TestSuppressionActions: every suppressible finding offers a suppression
// comment on the line above (with the line's indentation) and one for the
// whole file; after either one the finding is gone.
func TestSuppressionActions(t *testing.T) {
	text := "resources:\n- name: repo\n  type: git\n  source: {uri: https://github.com/example/app.git}\njobs:\n- name: build\n  plan:\n  - get: zzz\n  - get: repo\n"
	client, uri := openText(t, text)
	actions := codeActions(t, client, uri, Range{Start: Position{Line: 7, Character: 9}, End: Position{Line: 7, Character: 9}})
	line := findAction(actions, "Suppress CL002 on this line")
	file := findAction(actions, "Suppress CL002 in this file")
	if line == nil || file == nil {
		t.Fatalf("actions = %v, want both suppressions", titles(actions))
	}
	afterLine := applyEdits(t, text, line.Edit.Changes[uri])
	if !strings.Contains(afterLine, "  plan:\n  # concourse-lint:ignore CL002\n  - get: zzz\n") {
		t.Errorf("after the line suppression:\n%s", afterLine)
	}
	afterFile := applyEdits(t, text, file.Edit.Changes[uri])
	if !strings.HasPrefix(afterFile, "# concourse-lint:ignore-file CL002\nresources:") {
		t.Errorf("after the file suppression:\n%s", afterFile)
	}
	for _, fixed := range []string{afterLine, afterFile} {
		for _, finding := range lint.Check("fixed.yml", []byte(fixed)) {
			if finding.Rule == "CL002" {
				t.Errorf("CL002 still reported after the fix:\n%s", fixed)
			}
		}
	}
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

// TestSuppressionExtendsComment: an ignore comment on the line above gets
// the rule added to its list instead of a second comment.
func TestSuppressionExtendsComment(t *testing.T) {
	text := "resources:\n- name: repo\n  type: git\n  source: {uri: https://github.com/example/app.git}\njobs:\n- name: build\n  plan:\n  # concourse-lint:ignore CS001 -- reason\n  - get: zzz\n  - get: repo\n"
	client, uri := openText(t, text)
	actions := codeActions(t, client, uri, Range{Start: Position{Line: 8, Character: 9}, End: Position{Line: 8, Character: 9}})
	line := findAction(actions, "Suppress CL002 on this line")
	if line == nil {
		t.Fatalf("actions = %v", titles(actions))
	}
	if got := applyEdits(t, text, line.Edit.Changes[uri]); !strings.Contains(got, "  # concourse-lint:ignore CS001,CL002 -- reason\n  - get: zzz\n") {
		t.Errorf("after the fix:\n%s", got)
	}
}

// TestLineSuppressionForBlockScalar: a finding in a | block is reported at
// the block's key, so the comment goes above the key, not into the block
// (where a "#" line would be text). Every line suppression is tried before
// it is offered, so a comment that would not work is never suggested.
func TestLineSuppressionForBlockScalar(t *testing.T) {
	text := "resources:\n- name: repo\n  type: git\n  source: {uri: https://github.com/example/app.git}\njobs:\n- name: build\n  plan:\n  - get: repo\n  - task: test\n    file: repo/ci/test.yml\n    params:\n      SCRIPT: |\n        echo start\n        export TOKEN=ghp_abcdefghijklmnopqrstuvwxyz0123456789\n"
	client, uri := openText(t, text)
	findings := lint.Check("block.yml", []byte(text))
	if !slices.ContainsFunc(findings, func(finding analysis.Finding) bool { return finding.Rule == "CS002" && finding.Position.Line == 12 }) {
		t.Fatalf("test needs a CS002 on line 12, got %+v", findings)
	}
	actions := codeActions(t, client, uri, Range{Start: Position{Line: 11, Character: 14}, End: Position{Line: 11, Character: 14}})
	line := findAction(actions, "Suppress CS002 on this line")
	if line == nil {
		t.Fatalf("actions = %v, want the line suppression", titles(actions))
	}
	fixed := applyEdits(t, text, line.Edit.Changes[uri])
	if !strings.Contains(fixed, "    params:\n      # concourse-lint:ignore CS002\n      SCRIPT: |\n") {
		t.Errorf("after the fix:\n%s", fixed)
	}
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

// TestNoSuppressionForCL000: a syntax error cannot be suppressed.
func TestNoSuppressionForCL000(t *testing.T) {
	text := "jobs:\n- name: build\n  plan: [\n"
	client, uri := openText(t, text)
	actions := codeActions(t, client, uri, Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 3, Character: 0}})
	if action := findAction(actions, "Suppress CL000"); action != nil {
		t.Errorf("got %q", action.Title)
	}
}

func openText(t *testing.T, text string) (*testClient, string) {
	t.Helper()
	client := startServer(t, Options{})
	client.initialize()
	uri := "untitled:pipeline.yml"
	client.open(uri, languageIDPipeline, text)
	return client, uri
}

func codeActions(t *testing.T, client *testClient, uri string, actionRange Range) []codeAction {
	t.Helper()
	response := client.request("textDocument/codeAction", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"range":        actionRange,
		"context":      map[string]any{"diagnostics": []any{}},
	}, nil)
	if response["error"] != nil {
		t.Fatalf("error response: %v", response["error"])
	}
	encoded, _ := json.Marshal(response["result"])
	var actions []codeAction
	if err := json.Unmarshal(encoded, &actions); err != nil {
		t.Fatalf("result is no CodeAction list: %s", encoded)
	}
	return actions
}

func findAction(actions []codeAction, titlePrefix string) *codeAction {
	for index := range actions {
		if strings.HasPrefix(actions[index].Title, titlePrefix) {
			return &actions[index]
		}
	}
	return nil
}

func titles(actions []codeAction) []string {
	var all []string
	for _, action := range actions {
		all = append(all, action.Title)
	}
	return all
}

// applyEdits applies non-overlapping edits to the text, the last one first.
func applyEdits(t *testing.T, text string, edits []TextEdit) string {
	t.Helper()
	index := newLineIndex(text)
	sorted := slices.Clone(edits)
	slices.SortFunc(sorted, func(first, second TextEdit) int {
		return index.offset(second.Range.Start) - index.offset(first.Range.Start)
	})
	for _, edit := range sorted {
		start, end := index.offset(edit.Range.Start), index.offset(edit.Range.End)
		text = text[:start] + edit.NewText + text[end:]
	}
	return text
}
