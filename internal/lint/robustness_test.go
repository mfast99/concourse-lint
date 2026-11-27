package lint

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/complete"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/navigate"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// Metamorphic tests: a change to the input that does not change its meaning
// may only shift the findings in a predictable way.

// TestShiftedLines: inserting comment lines at the top shifts every line by
// the same amount. A byte order mark stays first: anywhere else it would be
// content.
func TestShiftedLines(t *testing.T) {
	const insertedLines = 3
	const byteOrderMark = "\ufeff"
	for _, file := range spectest.Files(t) {
		t.Run(spectest.Name(file), func(t *testing.T) {
			source, _ := os.ReadFile(file)
			withoutMark, hasMark := bytes.CutPrefix(source, []byte(byteOrderMark))
			shifted := []byte(strings.Repeat("# header\n", insertedLines))
			if hasMark {
				shifted = append([]byte(byteOrderMark), shifted...)
			}
			shifted = append(shifted, withoutMark...)
			if want, got := findingKeys(Check(file, source), insertedLines), findingKeys(Check(file, shifted), 0); want != got {
				t.Errorf("shifted:\n  want %s\n  got  %s", want, got)
			}
		})
	}
}

// TestCRLF: Windows line endings do not change the findings.
func TestCRLF(t *testing.T) {
	for _, file := range spectest.Files(t) {
		t.Run(spectest.Name(file), func(t *testing.T) {
			source, _ := os.ReadFile(file)
			crlf := bytes.ReplaceAll(source, []byte("\n"), []byte("\r\n"))
			if want, got := findingKeys(Check(file, source), 0), findingKeys(Check(file, crlf), 0); want != got {
				t.Errorf("CRLF:\n  want %s\n  got  %s", want, got)
			}
		})
	}
}

// findingKeys renders findings as sorted "line:column:rule" keys, with every
// line moved by lineShift.
func findingKeys(findings []analysis.Finding, lineShift int) string {
	var keys []string
	for _, finding := range findings {
		keys = append(keys, fmt.Sprintf("%d:%d:%s", finding.Position.Line+lineShift, finding.Position.Column, finding.Rule))
	}
	sort.Strings(keys)
	return strings.Join(keys, " ")
}

// FuzzCheck: Check and the navigation must not crash on any input, and every
// position must lie inside the file. Run with: make fuzz
func FuzzCheck(fuzz *testing.F) {
	for _, file := range spectest.Files(fuzz) {
		source, _ := os.ReadFile(file)
		fuzz.Add(source)
	}
	fuzz.Fuzz(func(t *testing.T, input []byte) {
		// Count lines like YAML and editors (LSP): CRLF, LF and a lone CR
		// are one line break each.
		withoutCRLF := bytes.ReplaceAll(input, []byte("\r\n"), []byte("\n"))
		lineCount := bytes.Count(withoutCRLF, []byte("\n")) + bytes.Count(withoutCRLF, []byte("\r")) + 1
		for _, finding := range Check("fuzz.yml", input) {
			if finding.Position.Line < 1 || finding.Position.Line > lineCount || finding.Position.Column < 1 {
				t.Errorf("position outside the file: %+v (lines: %d)", finding.Position, lineCount)
			}
			if finding.Rule == "" || finding.Message == "" {
				t.Errorf("finding without rule or message: %+v", finding)
			}
		}
		// The editor features must not crash either.
		pipeline, _ := Parse(input)
		for _, reference := range navigate.Index(pipeline) {
			navigate.Definition(pipeline, reference.Position)
			navigate.References(pipeline, reference.Position, true)
			navigate.Hover(pipeline, reference.Position)
		}
		for line := 1; line <= min(lineCount, 40); line++ {
			complete.At(input, model.Position{Line: line, Column: 1}, pipeline)
			complete.At(input, model.Position{Line: line, Column: 200}, pipeline)
		}
	})
}
