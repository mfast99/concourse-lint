package navigate_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/mfast99/concourse-lint/internal/navigate"
	"github.com/mfast99/concourse-lint/internal/parse"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestHoverSpec runs the cases of spec/editor/hover/: the summary of the
// name at the cursor in its one-line form, or none.
func TestHoverSpec(t *testing.T) {
	for _, file := range spectest.EditorFiles(t, "hover") {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		pipeline, _ := parse.Parse(source)
		for _, editorCase := range spectest.ParseEditorCases(source) {
			where := spectest.Name(file) + ":" + strconv.Itoa(editorCase.MarkerLine)
			if editorCase.Kind != "hover" {
				t.Errorf("%s: unknown kind %q", where, editorCase.Kind)
				continue
			}
			summary, found := navigate.Hover(pipeline, editorCase.Cursor)
			if editorCase.Arguments == "none" {
				if found {
					t.Errorf("%s: hover = %q, want none", where, summary.String())
				}
				continue
			}
			want, err := strconv.Unquote(editorCase.Arguments)
			if err != nil {
				t.Errorf("%s: want a quoted text or none, got %s", where, editorCase.Arguments)
				continue
			}
			if !found || summary.String() != want {
				t.Errorf("%s: hover = %q (found %v), want %q", where, summary.String(), found, want)
			}
		}
	}
}
