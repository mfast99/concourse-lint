package lint_test

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/lint"
)

// TestParse: the model for the editor features, read like Check reads it
// (CRLF counts as one line break), and whether the text could be read at
// all (no syntax error).
func TestParse(t *testing.T) {
	pipeline, readable := lint.Parse([]byte("resources:\r\n- name: repo\r\n  type: git\r\n"))
	if !readable || len(pipeline.Resources) != 1 || pipeline.Resources[0].Name.Position.Line != 2 {
		t.Errorf("Parse = %+v, %v; want one resource on line 2, readable", pipeline.Resources, readable)
	}
	if _, readable := lint.Parse([]byte("jobs: [\n")); readable {
		t.Error("Parse of broken YAML is readable, want not")
	}
}
