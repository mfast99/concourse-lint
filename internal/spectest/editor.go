package spectest

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mfast99/concourse-lint/internal/model"
)

// The editor cases in spec/editor/<feature>/ are pipelines with marker
// lines: a comment whose "^" points at the cursor on the nearest line above
// that is no marker line, followed by what the editor feature must answer
// there:
//
//	  - get: repo
//	#        ^ definition 2:9
//
// The files are clean pipelines, so TestSpec checks them like any other
// case.

// markerPattern matches a marker line: "#", spaces, "^", the kind of the
// case and its arguments.
var markerPattern = regexp.MustCompile(`^\s*#\s*\^\s+([a-z-]+)\s*(.*)$`)

// EditorCase is one marker line.
type EditorCase struct {
	// The line of the marker itself, for messages.
	MarkerLine int
	// Where the "^" points.
	Cursor model.Position
	// E.g. "definition", "references", "hover".
	Kind string
	// The rest of the line, e.g. "2:9 14:11".
	Arguments string
}

// EditorDir is spec/editor/: one folder per editor feature.
func EditorDir() string {
	return filepath.Join(Dir(), "editor")
}

// EditorFiles returns the cases of one feature, e.g. "definition".
func EditorFiles(t testing.TB, feature string) []string {
	files, err := filepath.Glob(filepath.Join(EditorDir(), feature, "*.yml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no editor cases for %s: %v", feature, err)
	}
	slices.Sort(files)
	return files
}

// ParseEditorCases reads the marker lines of a file.
func ParseEditorCases(source []byte) []EditorCase {
	var cases []EditorCase
	targetLine := 0
	scanner := bufio.NewScanner(bytes.NewReader(source))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		text := scanner.Text()
		match := markerPattern.FindStringSubmatch(text)
		if match == nil {
			targetLine = lineNumber
			continue
		}
		caretIndex := strings.Index(text, "^")
		cases = append(cases, EditorCase{
			MarkerLine: lineNumber,
			Cursor:     model.Position{Line: targetLine, Column: utf8.RuneCountInString(text[:caretIndex]) + 1},
			Kind:       match[1],
			Arguments:  strings.TrimSpace(match[2]),
		})
	}
	return cases
}

// ParsePositions reads positions written as "line:column", separated by
// spaces; "none" is the empty list.
func ParsePositions(arguments string) ([]model.Position, error) {
	positions := []model.Position{}
	if arguments == "none" {
		return positions, nil
	}
	for field := range strings.FieldsSeq(arguments) {
		lineText, columnText, found := strings.Cut(field, ":")
		line, lineErr := strconv.Atoi(lineText)
		column, columnErr := strconv.Atoi(columnText)
		if !found || lineErr != nil || columnErr != nil {
			return nil, fmt.Errorf("%q is no position (want line:column)", field)
		}
		positions = append(positions, model.Position{Line: line, Column: column})
	}
	return positions, nil
}
