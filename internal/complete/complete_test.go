package complete_test

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/complete"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/parse"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestCompletionSpec runs the cases of spec/editor/completion/:
// "complete" expects exactly these labels (sorted), "complete-has" at least
// these, "complete-not" none of these.
func TestCompletionSpec(t *testing.T) {
	for _, file := range spectest.EditorFiles(t, "completion") {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		pipeline, _ := parse.Parse(source)
		for _, editorCase := range spectest.ParseEditorCases(source) {
			where := spectest.Name(file) + ":" + strconv.Itoa(editorCase.MarkerLine)
			labels := labelsOf(complete.At(source, editorCase.Cursor, pipeline))
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
			default:
				t.Errorf("%s: unknown kind %q", where, editorCase.Kind)
			}
		}
	}
}

func labelsOf(result complete.Result) []string {
	labels := []string{}
	for _, item := range result.Items {
		labels = append(labels, item.Label)
	}
	slices.Sort(labels)
	return labels
}

// TestReplaceRange: the items replace the word at the cursor, from its
// start, also with a "-" in it.
func TestReplaceRange(t *testing.T) {
	source := []byte("resources:\n- name: deploy-repo\n  type: git\njobs:\n- name: build\n  plan:\n  - get: deploy-re\n")
	pipeline, _ := parse.Parse(source)
	result := complete.At(source, model.Position{Line: 7, Column: 18}, pipeline)
	if result.Start != (model.Position{Line: 7, Column: 10}) {
		t.Errorf("start = %+v, want 7:10", result.Start)
	}
	if len(result.Items) != 1 || result.Items[0].Insert != "deploy-repo" {
		t.Errorf("items = %+v", result.Items)
	}
}

// TestKeyInsertsColon: a key is inserted with ": ".
func TestKeyInsertsColon(t *testing.T) {
	source := []byte("jobs:\n- name: build\n  plan:\n  - get: repo\n    trig\n")
	result := complete.At(source, model.Position{Line: 5, Column: 9}, nil)
	index := slices.IndexFunc(result.Items, func(item complete.Item) bool { return item.Label == "trigger" })
	if index < 0 || result.Items[index].Insert != "trigger: " || result.Items[index].Kind != complete.KindField {
		t.Errorf("items = %+v, want trigger with insert \"trigger: \"", result.Items)
	}
}

// TestBrokenYAML: the place at the cursor comes from the text, so keys are
// offered also when the YAML cannot be read; names come from the model
// passed in (in the editor: the last one that could be read).
func TestBrokenYAML(t *testing.T) {
	good := []byte("resources:\n- name: repo\n  type: git\njobs:\n- name: build\n  plan:\n  - get: repo\n")
	lastGood, _ := parse.Parse(good)
	broken := []byte("resources:\n- name: repo\n  type: git\njobs:\n- name: build\n  plan:\n  - get: \n  - task: [\n")
	result := complete.At(broken, model.Position{Line: 7, Column: 10}, lastGood)
	if labels := labelsOf(result); !slices.Equal(labels, []string{"repo"}) {
		t.Errorf("labels = %v, want [repo]", labels)
	}
}

// TestNothingInFreeFormBlocks: source:, params: and the like have no
// fixed keys.
func TestNothingInFreeFormBlocks(t *testing.T) {
	source := []byte("resources:\n- name: repo\n  type: git\n  source:\n    uri: x\n")
	if result := complete.At(source, model.Position{Line: 5, Column: 6}, nil); len(result.Items) != 0 {
		t.Errorf("items = %+v, want none", result.Items)
	}
}
