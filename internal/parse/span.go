package parse

import (
	"strings"
	"unicode/utf8"

	"github.com/mfast99/go-yaml/token"

	"github.com/mfast99/concourse-lint/internal/model"
)

// tokenPosition returns where a token starts and ends. The end comes from
// the token text as written (with quotes, without the spaces and line
// breaks around it), counted in characters like the start.
func tokenPosition(sourceToken *token.Token) model.Position {
	if sourceToken == nil || sourceToken.Position == nil {
		return model.Position{Line: 1, Column: 1}
	}
	start := model.Position{Line: sourceToken.Position.Line, Column: sourceToken.Position.Column}
	written := strings.TrimSpace(strings.ReplaceAll(sourceToken.Origin, "\r", ""))
	if written == "" {
		written = sourceToken.Value
	}
	if written == "" {
		return start
	}
	position := start
	lines := strings.Split(written, "\n")
	if len(lines) == 1 {
		position.EndLine, position.EndColumn = start.Line, start.Column+utf8.RuneCountInString(written)
		return position
	}
	lastLine := lines[len(lines)-1]
	position.EndLine, position.EndColumn = start.Line+len(lines)-1, utf8.RuneCountInString(lastLine)+1
	return position
}
