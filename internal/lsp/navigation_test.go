package lsp

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestNavigationSpec runs the cases of spec/editor/navigation/ through the
// protocol: textDocument/definition and textDocument/references.
func TestNavigationSpec(t *testing.T) {
	client := startServer(t, Options{})
	response := client.initialize()
	capabilities := response["result"].(map[string]any)["capabilities"].(map[string]any)
	if capabilities["definitionProvider"] != true || capabilities["referencesProvider"] != true {
		t.Errorf("capabilities = %v, want definitionProvider and referencesProvider", capabilities)
	}
	for _, file := range spectest.EditorFiles(t, "navigation") {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		uri := pathToURI(file)
		client.open(uri, languageIDPipeline, string(source))
		index := newLineIndex(string(source))
		for _, editorCase := range spectest.ParseEditorCases(source) {
			want, err := spectest.ParsePositions(editorCase.Arguments)
			if err != nil {
				t.Fatalf("%s:%d: %v", spectest.Name(file), editorCase.MarkerLine, err)
			}
			params := map[string]any{
				"textDocument": map[string]any{"uri": uri},
				"position":     index.toPosition(editorCase.Cursor.Line, editorCase.Cursor.Column),
			}
			method := "textDocument/" + editorCase.Kind
			if editorCase.Kind == "references" {
				params["context"] = map[string]any{"includeDeclaration": false}
			}
			got := locationStarts(t, client.request(method, params, nil), uri, index)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s:%d: %s = %v, want %v", spectest.Name(file), editorCase.MarkerLine, method, got, want)
			}
		}
		client.notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": uri}})
	}
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

// TestNavigationUnknownDocument: a document that is not open has no
// definitions (null), not an error.
func TestNavigationUnknownDocument(t *testing.T) {
	client := startServer(t, Options{})
	client.initialize()
	response := client.request("textDocument/definition", map[string]any{
		"textDocument": map[string]any{"uri": "file:///nowhere.yml"},
		"position":     map[string]any{"line": 0, "character": 0},
	}, nil)
	if response["error"] != nil || response["result"] != nil {
		t.Errorf("response = %v, want result null", response)
	}
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

// locationStarts turns the Location list of a response into our start
// positions; every location must be in the document itself.
func locationStarts(t *testing.T, response map[string]any, uri string, index lineIndex) []model.Position {
	t.Helper()
	if response["error"] != nil {
		t.Fatalf("error response: %v", response["error"])
	}
	encoded, _ := json.Marshal(response["result"])
	var locations []Location
	if err := json.Unmarshal(encoded, &locations); err != nil {
		t.Fatalf("result is no Location list: %s", encoded)
	}
	starts := []model.Position{}
	for _, location := range locations {
		if location.URI != uri {
			t.Errorf("location in %s, want %s", location.URI, uri)
		}
		line, column := index.fromPosition(location.Range.Start)
		starts = append(starts, model.Position{Line: line, Column: column})
	}
	return starts
}
