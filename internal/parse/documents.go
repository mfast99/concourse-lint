package parse

import (
	"regexp"
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// LaterDocumentRule is CL024: content after the first YAML document, which
// Concourse never reads.
const LaterDocumentRule = "CL024"

// documentMarker matches a line with a document start (---) or end (...)
// marker; the group is the rest of the line.
var documentMarker = regexp.MustCompile(`^(?:---|\.\.\.)(\s.*)?$`)

// firstDocument returns the part of the source that Concourse reads: the
// first document, up to the --- or ... line after its content (comments
// and a --- before the content do not end it). When content follows, it
// also returns the CL024 finding at that line. The marker line stays in the
// first part when nothing but a comment follows it on that line, so a
// suppression comment there works.
func firstDocument(source []byte) ([]byte, *analysis.Finding) {
	lines := strings.SplitAfter(string(source), "\n")
	seenContent := false
	offset := 0
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		marker := documentMarker.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
		switch {
		case marker != nil && seenContent:
			rest := strings.TrimSpace(marker[1])
			cut := offset
			if rest == "" || strings.HasPrefix(rest, "#") {
				cut += len(line)
			}
			if !hasContent(lines[index+1:]) && (rest == "" || strings.HasPrefix(rest, "#")) {
				return source[:cut], nil
			}
			return source[:cut], &analysis.Finding{
				Rule:     LaterDocumentRule,
				Severity: analysis.SeverityWarning,
				Position: model.Position{Line: index + 1, Column: 1, EndLine: index + 1, EndColumn: 4},
				Message:  "Concourse reads only the first YAML document; everything after this line is ignored",
				Hint:     "move it into a file of its own, or remove the line if it belongs to the pipeline",
			}
		case marker == nil && trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "%"):
			seenContent = true
		}
		offset += len(line)
	}
	return source, nil
}

// hasContent reports whether the lines hold more than blank lines and
// comments.
func hasContent(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return true
		}
	}
	return false
}
