// Package analysis holds the shared types that all rules use:
// finding, position, severity and the description of a rule.
//
// This package imports nothing from the project except model. That way
// rules, lint, report, lsp and mcp can all use it without import cycles.
package analysis

import (
	"strconv"
	"strings"

	"github.com/mfast99/concourse-lint/internal/model"
)

// Severity states how serious a finding is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Finding is a single finding reported by a rule.
type Finding struct {
	// E.g. "CL002".
	Rule     string         `json:"rule"`
	Severity Severity       `json:"severity"`
	Position model.Position `json:"pos"`
	Message  string         `json:"message"`
	// E.g. "did you mean 'repo'?".
	Hint string `json:"hint,omitempty"`
	// Suggestion replaces the text of the finding's range, e.g. "repo" for
	// the typo "rep"; the editor offers it as a quick fix.
	Suggestion string `json:"suggestion,omitempty"`
}

// SuggestionTitle names the fix of the suggestion, e.g. `Change to "repo"`
// (also for "\"repo\"", the text with its quotes), for the editor's quick
// fix and SARIF's fixes.
func (finding Finding) SuggestionTitle() string {
	return "Change to " + strconv.Quote(strings.Trim(finding.Suggestion, `"'`))
}

// Rule describes a rule. Every rule lives in its own file under
// internal/rules/<area>/ and is registered there in All.
type Rule struct {
	ID       string
	Severity Severity
	Title    string
	Run      func(pass *Pass)
}

// Pass is what a rule receives for one run: the pipeline and a way to
// report findings.
type Pass struct {
	Pipeline *model.Pipeline
	// Settings are the rule options from the config file.
	Settings Settings
	rule     *Rule
	findings *[]Finding
}

// NewPass is used by lint to run a rule.
func NewPass(rule *Rule, pipeline *model.Pipeline, findings *[]Finding) *Pass {
	return &Pass{Pipeline: pipeline, rule: rule, findings: findings}
}

// ReportTypo reports a finding on a word that is probably a typo of one of
// the candidates: with the hint "did you mean" and the closest candidate as
// the suggestion. The position must cover exactly the word.
func (pass *Pass) ReportTypo(position model.Position, message, word string, candidates []string) {
	pass.Report(position, message, DidYouMean(word, candidates))
	if closest, found := ClosestName(word, candidates); found {
		(*pass.findings)[len(*pass.findings)-1].Suggestion = closest
	}
}

// Report reports a finding. ID and severity are taken from the rule.
func (pass *Pass) Report(position model.Position, message, hint string) {
	*pass.findings = append(*pass.findings, Finding{
		Rule:     pass.rule.ID,
		Severity: pass.rule.Severity,
		Position: position,
		Message:  message,
		Hint:     hint,
	})
}
