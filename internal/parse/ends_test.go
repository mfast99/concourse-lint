package parse_test

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestEnds: every position ends right after the text it stands for, counted
// in characters like the start; quotes belong to the text, a trailing
// comment does not.
func TestEnds(t *testing.T) {
	source := "# top\n" +
		"resources:\n" +
		"- name: plain value  # trailing\n" +
		"  type: \"quo ted\"\n" +
		"  icon: 'it''s'\n" +
		"- {name: ä€😀, type: git}\n" +
		"- name: multi\n" +
		"    line\n" +
		"  type: git\n" +
		"jobs: []\n"
	pipeline, _ := parse.Parse([]byte(source))
	span := func(line, column, endLine, endColumn int) model.Position {
		return model.Position{Line: line, Column: column, EndLine: endLine, EndColumn: endColumn}
	}
	tests := []struct {
		name      string
		got, want model.Position
	}{
		{"comment", pipeline.Comments[0].Position, span(1, 1, 1, 6)},
		{"key", pipeline.Resources[0].Name.KeyPosition, span(3, 3, 3, 7)},
		{"plain value", pipeline.Resources[0].Name.Position, span(3, 9, 3, 20)},
		{"double quoted", pipeline.Resources[0].Type.Position, span(4, 9, 4, 18)},
		{"single quoted", pipeline.Resources[0].Icon.Position, span(5, 9, 5, 16)},
		{"unicode in flow", pipeline.Resources[1].Name.Position, span(6, 10, 6, 13)},
		{"after unicode", pipeline.Resources[1].Type.Position, span(6, 21, 6, 24)},
		{"multi-line plain", pipeline.Resources[2].Name.Position, span(7, 9, 8, 9)},
	}
	for _, test := range tests {
		if test.got != test.want {
			t.Errorf("%s: %+v, want %+v", test.name, test.got, test.want)
		}
	}
}

// TestEndsOfFindings: a syntax error covers its token; a finding without a
// token has length zero.
func TestEndsOfFindings(t *testing.T) {
	_, findings := parse.Parse([]byte("resources: []\n"))
	if len(findings) != 1 || findings[0].Position != (model.Position{Line: 1, Column: 1}) {
		t.Errorf("no jobs: %+v, want one finding at 1:1 without an end", findings)
	}
	_, findings = parse.Parse([]byte("jobs:\n- name: [unclosed\n"))
	if len(findings) != 1 || findings[0].Position.EndLine == 0 {
		t.Errorf("syntax error: %+v, want one finding with an end", findings)
	}
}
