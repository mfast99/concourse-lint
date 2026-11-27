package lsp

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mfast99/concourse-lint/internal/model"
)

// lineIndex converts between our positions (line and column from 1, the
// column in characters) and LSP positions (from 0, the character in UTF-16
// code units, the only encoding VS Code supports).
type lineIndex struct {
	lines []string
	// lineStarts[n] is the byte offset in the text where line n (from 0)
	// starts, for applying edits.
	lineStarts []int
}

// lineBreaks are the line breaks of YAML and LSP; lint treats them the same.
var lineBreaks = regexp.MustCompile(`\r\n|\r|\n`)

func newLineIndex(text string) lineIndex {
	lineStarts := []int{0}
	for _, lineBreak := range lineBreaks.FindAllStringIndex(text, -1) {
		lineStarts = append(lineStarts, lineBreak[1])
	}
	return lineIndex{lines: lineBreaks.Split(text, -1), lineStarts: lineStarts}
}

// offset returns the byte offset of an LSP position in the text; positions
// beyond a line or the text are moved to its end.
func (index lineIndex) offset(position Position) int {
	if position.Line >= len(index.lines) {
		last := len(index.lines) - 1
		return index.lineStarts[last] + len(index.lines[last])
	}
	line := index.lines[max(position.Line, 0)]
	units := 0
	for byteOffset, character := range line {
		if units >= position.Character {
			return index.lineStarts[max(position.Line, 0)] + byteOffset
		}
		units += utf16Units(character)
	}
	return index.lineStarts[max(position.Line, 0)] + len(line)
}

// toRange converts a finding's position. Without an end the range has
// length zero; positions beyond a line or the text are moved to its end.
func (index lineIndex) toRange(position model.Position) Range {
	start := index.toPosition(position.Line, position.Column)
	if position.EndLine == 0 {
		return Range{Start: start, End: start}
	}
	end := index.toPosition(position.EndLine, position.EndColumn)
	if end.Line < start.Line || (end.Line == start.Line && end.Character < start.Character) {
		end = start
	}
	return Range{Start: start, End: end}
}

func (index lineIndex) toPosition(line, column int) Position {
	lineNumber := min(max(line-1, 0), len(index.lines)-1)
	if line-1 > lineNumber {
		return Position{Line: lineNumber, Character: utf16Length(index.lines[lineNumber], -1)}
	}
	return Position{Line: lineNumber, Character: utf16Length(index.lines[lineNumber], max(column-1, 0))}
}

// fromPosition is the way back: an LSP position as our line and column.
func (index lineIndex) fromPosition(position Position) (line, column int) {
	if position.Line < 0 || position.Line >= len(index.lines) {
		return position.Line + 1, position.Character + 1
	}
	units, characters := 0, 0
	for _, character := range index.lines[position.Line] {
		if units >= position.Character {
			break
		}
		units += utf16Units(character)
		characters++
	}
	return position.Line + 1, characters + 1
}

// utf16Length returns the UTF-16 length of the first count characters of
// the line (all of them for a negative count or one beyond its end).
func utf16Length(line string, count int) int {
	units, characters := 0, 0
	for _, character := range line {
		if count >= 0 && characters >= count {
			break
		}
		units += utf16Units(character)
		characters++
	}
	return units
}

func utf16Units(character rune) int {
	if character >= 0x10000 && character <= utf8.MaxRune {
		return 2
	}
	return 1
}

// windowsDrive matches the start of a Windows path in a file URI, /c:/.
var windowsDrive = regexp.MustCompile(`^/[A-Za-z]:`)

// uriToPath returns the file path of a file URI, "" for other schemes
// (e.g. untitled: for a new, unsaved file).
func uriToPath(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" {
		return ""
	}
	path := parsed.Path
	if windowsDrive.MatchString(path) {
		path = path[1:]
	}
	return filepath.FromSlash(path)
}

// pathToURI returns the file URI of a path.
func pathToURI(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	slashPath := filepath.ToSlash(path)
	if !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	return (&url.URL{Scheme: "file", Path: slashPath}).String()
}
