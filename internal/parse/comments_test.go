package parse_test

import (
	"reflect"
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestComments: every comment with the position of its "#"; a "#" inside a
// string or a block scalar is no comment.
func TestComments(t *testing.T) {
	source := "# top\n" +
		"jobs: # trailing\n" +
		"- name: \"# not a comment\"\n" +
		"  plan:\n" +
		"  - task: t\n" +
		"    config:\n" +
		"      run:\n" +
		"        path: |\n" +
		"          # in a block scalar\n" +
		"  # indented\n"
	pipeline, _ := parse.Parse([]byte(source))
	clearEnds(pipeline.Comments)
	want := []model.Comment{
		{Position: model.Position{Line: 1, Column: 1}, Text: " top"},
		{Position: model.Position{Line: 2, Column: 7}, Text: " trailing"},
		{Position: model.Position{Line: 10, Column: 3}, Text: " indented"},
	}
	if !reflect.DeepEqual(pipeline.Comments, want) {
		t.Errorf("comments = %+v, want %+v", pipeline.Comments, want)
	}
}
