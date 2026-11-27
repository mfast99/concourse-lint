package parse

import (
	"regexp"
	"strings"

	"github.com/mfast99/go-yaml/lexer"
	"github.com/mfast99/go-yaml/token"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// TemplateRule is CL022: the file is a ytt, spruce, ERB or Jinja template, not a
// pipeline. lint reports it instead of all other findings, because the
// template is no pipeline before it is rendered.
const TemplateRule = "CL022"

// spruceOperators are the operators of spruce (doc/operators.md), including
// the ones for merging arrays.
var spruceOperators = []string{
	"append", "awsparam", "awssecret", "base64", "base64-decode", "calc",
	"cartesian-product", "concat", "defer", "delete", "empty", "file", "grab",
	"inject", "inline", "insert", "ips", "join", "keys", "load", "merge",
	"negate", "param", "prepend", "prune", "raw_env", "replace", "shuffle",
	"sort", "static_ips", "stringify", "vault",
}

// spruceExpression matches a whole value that is a spruce operator call,
// e.g. "(( grab meta.repo ))" or "(( append ))".
var spruceExpression = regexp.MustCompile(`^\(\(\s*(?:` + strings.Join(quoteAll(spruceOperators), "|") + `)(?:\s.*)?\)\)$`)

// Template reports whether the source is a ytt, spruce, ERB or Jinja
// template and returns the CL022 finding at its first marker:
//
//   - ytt: a comment that starts with "#@" (the lexer tells a comment from
//     a "#@" inside a string).
//   - spruce: a plain or quoted value that is a whole spruce operator call
//     with a space inside. A Concourse variable never has a space inside
//     the parentheses, so ((grab)) stays a variable.
//   - ERB and Jinja: a tag outside comments and block scalars, see
//     markupTemplateFinding.
//
// It works on the tokens, so a template with broken YAML is recognized too.
func Template(source []byte) (analysis.Finding, bool) {
	sourceTokens := lexer.Tokenize(string(source))
	for index, sourceToken := range sourceTokens {
		if sourceToken.Position == nil {
			continue
		}
		switch {
		case sourceToken.Type == token.CommentType && strings.HasPrefix(sourceToken.Value, "@"):
			return templateFinding(sourceToken, "ytt", "ytt -f <files>"), true
		case isFlowString(sourceToken) && !isBlockScalarContent(sourceTokens, index) && isSpruceExpression(sourceToken.Value):
			return templateFinding(sourceToken, "spruce", "spruce merge <files>"), true
		}
	}
	return markupTemplateFinding(source)
}

// isSpruceExpression tells whether a value is a whole spruce operator call
// with a space inside.
func isSpruceExpression(value string) bool {
	value = strings.TrimSpace(value)
	return strings.ContainsAny(value, " \t") && spruceExpression.MatchString(value)
}

// templateFinding is the CL022 finding at a template marker.
func templateFinding(marker *token.Token, tool, renderCommand string) analysis.Finding {
	return analysis.Finding{
		Rule:     TemplateRule,
		Severity: analysis.SeverityInfo,
		Position: tokenPosition(marker),
		Message:  "file is a " + tool + " template, not a pipeline; it is not checked",
		Hint:     "check the rendered pipeline: " + renderCommand + " | concourse-lint check -",
	}
}

// quoteAll escapes every word for a regular expression.
func quoteAll(words []string) []string {
	quoted := make([]string, len(words))
	for index, word := range words {
		quoted[index] = regexp.QuoteMeta(word)
	}
	return quoted
}
