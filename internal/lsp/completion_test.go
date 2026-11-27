package lsp

import (
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestCompletionSpec runs the cases of spec/editor/completion/ through the
// protocol, comparing the labels like the core test does.
func TestCompletionSpec(t *testing.T) {
	client := startServer(t, Options{})
	response := client.initialize()
	if capabilities := response["result"].(map[string]any)["capabilities"].(map[string]any); capabilities["completionProvider"] == nil {
		t.Errorf("capabilities = %v, want completionProvider", capabilities)
	}
	for _, file := range spectest.EditorFiles(t, "completion") {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		uri := pathToURI(file)
		client.open(uri, languageIDPipeline, string(source))
		index := newLineIndex(string(source))
		for _, editorCase := range spectest.ParseEditorCases(source) {
			where := spectest.Name(file) + ":" + strconv.Itoa(editorCase.MarkerLine)
			items := completionItems(t, client, uri, index.toPosition(editorCase.Cursor.Line, editorCase.Cursor.Column))
			labels := itemLabels(items)
			want := strings.Fields(editorCase.Arguments)
			switch editorCase.Kind {
			case "complete":
				if !slices.Equal(labels, want) {
					t.Errorf("%s: labels = %v, want %v", where, labels, want)
				}
			case "complete-has":
				for _, label := range want {
					if !slices.Contains(labels, label) {
						t.Errorf("%s: %q missing in %v", where, label, labels)
					}
				}
			case "complete-not":
				for _, label := range want {
					if slices.Contains(labels, label) {
						t.Errorf("%s: %q must not be in %v", where, label, labels)
					}
				}
			}
		}
	}
	if err := client.shutdown(); err != nil {
		t.Errorf("Serve = %v", err)
	}
}

// TestCompletionTextEdit: an item replaces the word from its start to the
// cursor; a key gets ": ".
func TestCompletionTextEdit(t *testing.T) {
	text := "resources:\n- name: deploy-repo\n  type: git\n  source: {uri: https://github.com/example/app.git}\njobs:\n- name: build\n  plan:\n  - get: deploy-re\n    trigger: true\n"
	client, uri := openText(t, text)
	items := completionItems(t, client, uri, Position{Line: 7, Character: 17})
	if len(items) != 1 || items[0].TextEdit.NewText != "deploy-repo" ||
		items[0].TextEdit.Range != (Range{Start: Position{Line: 7, Character: 9}, End: Position{Line: 7, Character: 17}}) {
		t.Errorf("items = %+v", items)
	}
	keys := completionItems(t, client, uri, Position{Line: 8, Character: 8})
	index := slices.IndexFunc(keys, func(item completionItem) bool { return item.Label == "trigger" })
	if index < 0 || keys[index].TextEdit.NewText != "trigger: " || keys[index].Kind != completionKindProperty {
		t.Errorf("keys = %+v", keys)
	}
}

// TestCompletionKeepsLastModel: while the YAML is broken, the names come
// from the last version that could be read.
func TestCompletionKeepsLastModel(t *testing.T) {
	good := "resources:\n- name: repo\n  type: git\n  source: {uri: https://github.com/example/app.git}\njobs:\n- name: build\n  plan:\n  - get: repo\n"
	client, uri := openText(t, good)
	completionItems(t, client, uri, Position{Line: 7, Character: 11})
	client.change(uri, 2, good+"  - get: \n  - task: [\n")
	labels := itemLabels(completionItems(t, client, uri, Position{Line: 8, Character: 9}))
	if !slices.Equal(labels, []string{"repo"}) {
		t.Errorf("labels = %v, want [repo] from the last readable version", labels)
	}
}

func completionItems(t *testing.T, client *testClient, uri string, position Position) []completionItem {
	t.Helper()
	response := client.request("textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": uri}, "position": position,
	}, nil)
	if response["error"] != nil {
		t.Fatalf("error response: %v", response["error"])
	}
	encoded, _ := json.Marshal(response["result"])
	var items []completionItem
	if err := json.Unmarshal(encoded, &items); err != nil {
		t.Fatalf("result is no CompletionItem list: %s", encoded)
	}
	return items
}

func itemLabels(items []completionItem) []string {
	labels := []string{}
	for _, item := range items {
		labels = append(labels, item.Label)
	}
	slices.Sort(labels)
	return labels
}
