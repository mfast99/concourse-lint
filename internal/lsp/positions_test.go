package lsp

import (
	"runtime"
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
)

// TestToRange: our positions (from 1, characters) become LSP ranges (from
// 0, UTF-16 code units). ä and € are one unit, 😀 is two.
func TestToRange(t *testing.T) {
	text := "jobs:\n- {params: {x: ä€😀}, get: zz}\r\n\tkey: value\n"
	span := func(line, column, endLine, endColumn int) model.Position {
		return model.Position{Line: line, Column: column, EndLine: endLine, EndColumn: endColumn}
	}
	tests := []struct {
		name     string
		position model.Position
		want     Range
	}{
		{"ascii", span(1, 1, 1, 5), Range{Position{0, 0}, Position{0, 4}}},
		{"after an emoji", span(2, 27, 2, 29), Range{Position{1, 27}, Position{1, 29}}},
		{"around the emoji", span(2, 16, 2, 19), Range{Position{1, 15}, Position{1, 19}}},
		{"after a tab", span(3, 2, 3, 5), Range{Position{2, 1}, Position{2, 4}}},
		{"no end", span(2, 3, 0, 0), Range{Position{1, 2}, Position{1, 2}}},
		{"beyond the line", span(1, 40, 1, 50), Range{Position{0, 5}, Position{0, 5}}},
		{"beyond the text", span(9, 1, 9, 3), Range{Position{3, 0}, Position{3, 0}}},
		{"zero position", model.Position{}, Range{}},
	}
	for _, test := range tests {
		if got := newLineIndex(text).toRange(test.position); got != test.want {
			t.Errorf("%s: %+v, want %+v", test.name, got, test.want)
		}
	}
}

// TestFromPosition is the way back, used by the tests to compare with
// spec/.
func TestFromPosition(t *testing.T) {
	index := newLineIndex("a: ä€😀b\n")
	if line, column := index.fromPosition(Position{0, 7}); line != 1 || column != 7 {
		t.Errorf("got %d:%d, want 1:7", line, column)
	}
}

func TestURIs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX paths")
	}
	tests := map[string]string{
		"file:///home/me/ci/pipeline.yml":     "/home/me/ci/pipeline.yml",
		"file:///home/me/my%20ci/p%C3%A4.yml": "/home/me/my ci/pä.yml",
		"untitled:Untitled-1":                 "",
		"file:///c%3A/ci/pipeline.yml":        "c:/ci/pipeline.yml",
	}
	for uri, want := range tests {
		if got := uriToPath(uri); got != want {
			t.Errorf("uriToPath(%q) = %q, want %q", uri, got, want)
		}
	}
	if got := pathToURI("/home/me/my ci/pä.yml"); got != "file:///home/me/my%20ci/p%C3%A4.yml" {
		t.Errorf("pathToURI = %q", got)
	}
}

// TestOffset: LSP positions as byte offsets, with UTF-16 characters and
// CRLF line breaks.
func TestOffset(t *testing.T) {
	text := "a: ä€\r\nb: 😀x\n"
	index := newLineIndex(text)
	tests := []struct {
		position Position
		want     int
	}{
		{Position{Line: 0, Character: 0}, 0},
		{Position{Line: 0, Character: 4}, len("a: ä")},
		{Position{Line: 0, Character: 99}, len("a: ä€")},
		{Position{Line: 1, Character: 5}, len("a: ä€\r\nb: 😀")},
		{Position{Line: 2, Character: 0}, len(text)},
		{Position{Line: 7, Character: 0}, len(text)},
	}
	for _, test := range tests {
		if got := index.offset(test.position); got != test.want {
			t.Errorf("offset(%+v) = %d, want %d", test.position, got, test.want)
		}
	}
}
