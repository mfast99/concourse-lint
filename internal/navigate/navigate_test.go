package navigate_test

import (
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/navigate"
	"github.com/mfast99/concourse-lint/internal/parse"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestNavigationSpec runs the definition and references cases of
// spec/editor/navigation/.
func TestNavigationSpec(t *testing.T) {
	for _, file := range spectest.EditorFiles(t, "navigation") {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		pipeline, _ := parse.Parse(source)
		cases := spectest.ParseEditorCases(source)
		if len(cases) == 0 {
			t.Errorf("%s: no marker lines", spectest.Name(file))
		}
		for _, editorCase := range cases {
			want, err := spectest.ParsePositions(editorCase.Arguments)
			if err != nil {
				t.Errorf("%s:%d: %v", spectest.Name(file), editorCase.MarkerLine, err)
				continue
			}
			var got []model.Position
			switch editorCase.Kind {
			case "definition":
				got = navigate.Definition(pipeline, editorCase.Cursor)
			case "references":
				got = navigate.References(pipeline, editorCase.Cursor, false)
			default:
				t.Errorf("%s:%d: unknown kind %q", spectest.Name(file), editorCase.MarkerLine, editorCase.Kind)
				continue
			}
			if starts := startsOf(got); !reflect.DeepEqual(starts, want) {
				t.Errorf("%s:%d: %s at %d:%d = %v, want %v", spectest.Name(file), editorCase.MarkerLine,
					editorCase.Kind, editorCase.Cursor.Line, editorCase.Cursor.Column, starts, want)
			}
		}
	}
}

// startsOf returns the start of every position, without the end, because
// the cases only name the start.
func startsOf(positions []model.Position) []model.Position {
	starts := []model.Position{}
	for _, position := range positions {
		starts = append(starts, position.Start())
	}
	return starts
}

// TestReferencesWithDeclaration: with the declaration, the definition is
// listed too, everything in the order of the file.
func TestReferencesWithDeclaration(t *testing.T) {
	source := "resources:\n" +
		"- name: repo\n" +
		"  type: git\n" +
		"jobs:\n" +
		"- name: build\n" +
		"  plan:\n" +
		"  - get: repo\n"
	pipeline, _ := parse.Parse([]byte(source))
	got := startsOf(navigate.References(pipeline, model.Position{Line: 7, Column: 10}, true))
	want := []model.Position{{Line: 2, Column: 9}, {Line: 7, Column: 10}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("References with declaration = %v, want %v", got, want)
	}
}

// TestCursorAtEndOfName: the cursor right after the last character of a
// name still counts, like in an editor where it stands behind the word.
func TestCursorAtEndOfName(t *testing.T) {
	source := "resources:\n- name: repo\n  type: git\njobs:\n- name: build\n  plan:\n  - get: repo\n"
	pipeline, _ := parse.Parse([]byte(source))
	for _, column := range []int{10, 13, 14} {
		if got := navigate.Definition(pipeline, model.Position{Line: 7, Column: column}); len(got) != 1 {
			t.Errorf("Definition at 7:%d = %v, want the resource", column, got)
		}
	}
	if got := navigate.Definition(pipeline, model.Position{Line: 7, Column: 9}); len(got) != 0 {
		t.Errorf("Definition at 7:9 (the space) = %v, want none", got)
	}
}

// TestDuplicateDefinitions: a name defined twice (CL005) has two
// definitions.
func TestDuplicateDefinitions(t *testing.T) {
	source := "resources:\n- name: repo\n  type: git\n- name: repo\n  type: git\njobs:\n- name: build\n  plan:\n  - get: repo\n"
	pipeline, _ := parse.Parse([]byte(source))
	got := startsOf(navigate.Definition(pipeline, model.Position{Line: 9, Column: 10}))
	want := []model.Position{{Line: 2, Column: 9}, {Line: 4, Column: 9}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Definition = %v, want %v", got, want)
	}
}

// TestVariablesAreNoReferences: a name with ((var)) cannot be resolved.
func TestVariablesAreNoReferences(t *testing.T) {
	source := "resources:\n- name: ((name))\n  type: git\njobs:\n- name: build\n  plan:\n  - get: ((name))\n"
	pipeline, _ := parse.Parse([]byte(source))
	references := navigate.Index(pipeline)
	if slices.ContainsFunc(references, func(reference navigate.Reference) bool { return reference.Kind == navigate.KindResource }) {
		t.Errorf("Index = %+v, want no resource references", references)
	}
}

// TestGlobReferencesOnce: on a glob that matches several jobs, every
// position comes once, although it refers to each of the jobs.
func TestGlobReferencesOnce(t *testing.T) {
	source := "jobs:\n- name: deploy-a\n  plan: []\n- name: deploy-b\n  plan: []\ngroups:\n- name: all\n  jobs: [\"deploy-*\"]\n"
	pipeline, _ := parse.Parse([]byte(source))
	got := startsOf(navigate.References(pipeline, model.Position{Line: 8, Column: 11}, true))
	want := []model.Position{{Line: 2, Column: 9}, {Line: 4, Column: 9}, {Line: 8, Column: 10}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("References = %v, want %v", got, want)
	}
}
