package spectest

import (
	"reflect"
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
)

// TestParseEditorCases: a "^" in a comment line marks the cursor on the
// nearest line above that is no marker line; its column is the column of
// the "^", counted in characters.
func TestParseEditorCases(t *testing.T) {
	source := "jobs:\n" +
		"- name: büld\n" +
		"#        ^ definition 2:9\n" +
		"#          ^ references none\n" +
		"  plan: []\n" +
		"#  ^ hover \"plan of the job\"\n"
	want := []EditorCase{
		{MarkerLine: 3, Cursor: model.Position{Line: 2, Column: 10}, Kind: "definition", Arguments: "2:9"},
		{MarkerLine: 4, Cursor: model.Position{Line: 2, Column: 12}, Kind: "references", Arguments: "none"},
		{MarkerLine: 6, Cursor: model.Position{Line: 5, Column: 4}, Kind: "hover", Arguments: `"plan of the job"`},
	}
	if got := ParseEditorCases([]byte(source)); !reflect.DeepEqual(got, want) {
		t.Errorf("ParseEditorCases =\n%+v\nwant\n%+v", got, want)
	}
}

// TestParsePositions: positions are "line:column", separated by spaces;
// "none" is the empty list.
func TestParsePositions(t *testing.T) {
	got, err := ParsePositions("4:9 12:11")
	want := []model.Position{{Line: 4, Column: 9}, {Line: 12, Column: 11}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("ParsePositions = %+v, %v; want %+v", got, err, want)
	}
	if got, err := ParsePositions("none"); err != nil || len(got) != 0 {
		t.Errorf("ParsePositions(none) = %+v, %v; want empty", got, err)
	}
	if _, err := ParsePositions("4"); err == nil {
		t.Error("ParsePositions(4) = nil error, want an error")
	}
}
