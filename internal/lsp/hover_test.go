package lsp

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestHoverSpec runs the cases of spec/editor/hover/ through the protocol:
// a hover where the case expects one, with every fact of the summary in its
// Markdown, and null where it expects none.
func TestHoverSpec(t *testing.T) {
	client := startServer(t, Options{})
	response := client.initialize()
	if capabilities := response["result"].(map[string]any)["capabilities"].(map[string]any); capabilities["hoverProvider"] != true {
		t.Errorf("capabilities = %v, want hoverProvider", capabilities)
	}
	for _, file := range spectest.EditorFiles(t, "hover") {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		uri := pathToURI(file)
		client.open(uri, languageIDPipeline, string(source))
		index := newLineIndex(string(source))
		for _, editorCase := range spectest.ParseEditorCases(source) {
			where := spectest.Name(file) + ":" + strconv.Itoa(editorCase.MarkerLine)
			markdown := hoverText(t, client, uri, index.toPosition(editorCase.Cursor.Line, editorCase.Cursor.Column))
			if editorCase.Arguments == "none" {
				if markdown != "" {
					t.Errorf("%s: hover = %q, want none", where, markdown)
				}
				continue
			}
			want, _ := strconv.Unquote(editorCase.Arguments)
			parts := strings.Split(want, " | ")
			kind, name := splitTitle(parts[0])
			if !strings.Contains(markdown, "**"+kind+"** `"+name+"`") {
				t.Errorf("%s: hover = %q, want the title **%s** `%s`", where, markdown, kind, name)
			}
			for _, fact := range parts[1:] {
				if !strings.Contains(markdown, fact) {
					t.Errorf("%s: hover = %q, want %q in it", where, markdown, fact)
				}
			}
		}
	}
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

// TestHoverOnFinding: on a finding, the hover explains the rule from its
// page in docs/rules/: title, what is checked, the fix and a link; the
// summary of the name follows.
func TestHoverOnFinding(t *testing.T) {
	client := startServer(t, Options{})
	client.initialize()
	file := filepath.Join(spectest.Dir(), "rules", "correctness", "CL002", "get-put.yml")
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	uri := pathToURI(file)
	client.open(uri, languageIDPipeline, string(source))
	// "rep" in "  - get: rep" (line 13).
	markdown := hoverText(t, client, uri, Position{Line: 12, Character: 10})
	for _, want := range []string{
		"**CL002** get/put or a group refers to a missing resource",
		"Every `get:` and `put:` step needs a resource",
		"**Fix:** Correct the name",
		"(https://github.com/mfast99/concourse-lint/blob/main/docs/rules/CL002.md)",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("hover = %q, want %q in it", markdown, want)
		}
	}
	// A line without a finding or a name.
	if markdown := hoverText(t, client, uri, Position{Line: 13, Character: 6}); markdown != "" {
		t.Errorf("hover on trigger: = %q, want none", markdown)
	}
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

// splitTitle splits "resource type git" into the kind and the name.
func splitTitle(title string) (kind, name string) {
	for _, known := range []string{"resource type", "resource", "job", "glob"} {
		if rest, found := strings.CutPrefix(title, known+" "); found {
			return known, rest
		}
	}
	return "", title
}

// hoverText sends textDocument/hover and returns the Markdown, or "" for
// a null result.
func hoverText(t *testing.T, client *testClient, uri string, position Position) string {
	t.Helper()
	response := client.request("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": uri}, "position": position,
	}, nil)
	if response["error"] != nil {
		t.Fatalf("error response: %v", response["error"])
	}
	result, isHover := response["result"].(map[string]any)
	if !isHover {
		return ""
	}
	contents := result["contents"].(map[string]any)
	if contents["kind"] != "markdown" {
		t.Errorf("contents kind = %v, want markdown", contents["kind"])
	}
	return contents["value"].(string)
}
