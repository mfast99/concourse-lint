package parse

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// markupTemplate is a template language that works on the text, not on the
// YAML: ERB or Jinja.
type markupTemplate struct {
	// tool is the language with its article, e.g. "an ERB".
	tool          string
	renderCommand string
	// marker matches a tag of the language in a line without its comment.
	marker *regexp.Regexp
}

var markupTemplates = []markupTemplate{
	// ERB: <% … %> and <%= … %>, also a tag over several lines (a line that
	// starts with <% or %>).
	{"an ERB", "erb <file>", regexp.MustCompile(`<%.*?%>|^\s*<%|^\s*-?%>`)},
	// Jinja: {% … %} and {# … #}, also a {% tag over several lines.
	{"a Jinja", "jinja2 <file> <values>", regexp.MustCompile(`\{%.*?%\}|\{#.*?#\}|^\s*\{%`)},
}

var (
	// blockScalarStart matches a line that starts a block scalar (| or >,
	// with indicators, before an optional comment).
	blockScalarStart = regexp.MustCompile(`[:\-]\s*[|>][-+0-9]*\s*(?:#.*)?$`)
	// trailingComment matches a YAML comment behind a value; the # of a
	// Jinja comment end (#}) is no comment.
	trailingComment = regexp.MustCompile(`\s#[^}].*$|\s#$`)
)

// markupTemplateFinding returns the CL022 finding at the first ERB or Jinja
// tag. Tags in comments and in block scalars (scripts) do not count. It
// works on the lines, because such a template is often no YAML at all.
func markupTemplateFinding(source []byte) (analysis.Finding, bool) {
	blockIndent := -1
	for index, line := range strings.Split(string(source), "\n") {
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		trimmed := strings.TrimSpace(line)
		if blockIndent >= 0 {
			if trimmed == "" || indent > blockIndent {
				continue
			}
			blockIndent = -1
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		text := trailingComment.ReplaceAllString(line, "")
		if blockScalarStart.MatchString(text) {
			blockIndent = indent
		}
		for _, template := range markupTemplates {
			location := template.marker.FindStringIndex(text)
			if location == nil {
				continue
			}
			start := location[0] + len(text[location[0]:location[1]]) - len(strings.TrimLeft(text[location[0]:location[1]], " \t"))
			column := utf8.RuneCountInString(text[:start]) + 1
			return analysis.Finding{
				Rule:     TemplateRule,
				Severity: analysis.SeverityInfo,
				Position: model.Position{
					Line:      index + 1,
					Column:    column,
					EndLine:   index + 1,
					EndColumn: column + utf8.RuneCountInString(text[start:location[1]]),
				},
				Message: "file is " + template.tool + " template, not a pipeline; it is not checked",
				Hint:    "check the rendered pipeline: " + template.renderCommand + " | concourse-lint check -",
			}, true
		}
	}
	return analysis.Finding{}, false
}
