package parse

import (
	"strings"

	"github.com/mfast99/go-yaml/token"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// literalTabs reports CL021 for every plain or quoted string that contains a
// literal tab. YAML and Concourse accept it, but it is almost never meant.
// Block scalars (| and >) are left out,
// because tabs in scripts are normal there. The tab must be in the string as
// written: the escape \t in double quotes is the way to write an intended tab.
func literalTabs(sourceTokens token.Tokens) []analysis.Finding {
	var findings []analysis.Finding
	for index, sourceToken := range sourceTokens {
		if !isFlowString(sourceToken) || isBlockScalarContent(sourceTokens, index) {
			continue
		}
		if !containsWrittenTab(sourceToken.Origin) {
			continue
		}
		findings = append(findings, analysis.Finding{
			Rule:     "CL021",
			Severity: analysis.SeverityInfo,
			Position: tokenPosition(sourceToken),
			Message:  "string contains a literal tab character",
			Hint:     `if the tab is intended, write it as \t in double quotes`,
		})
	}
	return findings
}

// isFlowString tells whether the token is a plain, single-quoted or
// double-quoted string.
func isFlowString(sourceToken *token.Token) bool {
	switch sourceToken.Type {
	case token.StringType, token.SingleQuoteType, token.DoubleQuoteType:
		return true
	}
	return false
}

// isBlockScalarContent tells whether the token is the text of a block
// scalar: goccy gives it the string type, right after the | or > token, or
// after a comment behind it (`run: | # script`).
func isBlockScalarContent(sourceTokens token.Tokens, index int) bool {
	previous := index - 1
	for previous >= 0 && sourceTokens[previous].Type == token.CommentType {
		previous--
	}
	if previous < 0 {
		return false
	}
	previousType := sourceTokens[previous].Type
	return previousType == token.LiteralType || previousType == token.FoldedType
}

// containsWrittenTab tells whether the token text has a tab inside the
// string. Tabs at the start or end of a line are whitespace around the
// string (or folded away in a multi-line string), not part of it.
func containsWrittenTab(origin string) bool {
	for line := range strings.SplitSeq(origin, "\n") {
		if strings.Contains(strings.TrimSpace(line), "\t") {
			return true
		}
	}
	return false
}
