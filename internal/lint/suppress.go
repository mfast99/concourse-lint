package lint

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/parse"
	"github.com/mfast99/concourse-lint/internal/rules"
)

// CL015 is reported by lint itself: a suppression comment without a rule ID
// or with an unknown one. Cases: spec/rules/correctness/CL015/
const invalidSuppressionRule = "CL015"

// invalidSuppression describes CL015 for Rules.
var invalidSuppression = analysis.Rule{ID: invalidSuppressionRule, Severity: analysis.SeverityWarning, Title: "Invalid suppression comment"}

// unsuppressible rules: CL000 (comments of broken YAML are not reliable) and
// CL015 (a broken suppression comment must not switch itself off).
var unsuppressible = []string{"CL000", invalidSuppressionRule}

const (
	ignorePrefix     = "concourse-lint:ignore"
	ignoreFilePrefix = "concourse-lint:ignore-file"
)

// suppressions are the parsed suppression comments of one file.
type suppressions struct {
	// byLine[line] are the rule IDs ignored on that line.
	byLine map[int][]string
	// wholeFile are the rule IDs ignored in the whole file.
	wholeFile []string
	// problems are the CL015 findings.
	problems []analysis.Finding
}

// Suppressible reports whether a suppression comment can switch the rule
// off (all except CL000 and CL015).
func Suppressible(ruleID string) bool {
	return !slices.Contains(unsuppressible, ruleID)
}

// RuleIDs returns every rule ID the linter knows, sorted.
func RuleIDs() []string {
	var ids []string
	for _, rule := range Rules() {
		ids = append(ids, rule.ID)
	}
	return ids
}

// Rules describes every rule the linter knows, sorted by ID: the parser
// rules, the rules under internal/rules and CL015. Run is not set; the
// list only says what exists (list_rules of the MCP server).
func Rules() []analysis.Rule {
	described := slices.Clone(parse.Rules)
	for _, rule := range rules.All() {
		described = append(described, analysis.Rule{ID: rule.ID, Severity: rule.Severity, Title: rule.Title})
	}
	described = append(described, invalidSuppression)
	// A rule both the parser and internal/rules report (CL017) is listed once.
	slices.SortStableFunc(described, func(first, second analysis.Rule) int { return strings.Compare(first.ID, second.ID) })
	return slices.CompactFunc(described, func(first, second analysis.Rule) bool { return first.ID == second.ID })
}

// readSuppressions parses the suppression comments (spec/README.md,
// "Suppression").
func readSuppressions(comments []model.Comment, source []byte) suppressions {
	result := suppressions{byLine: map[int][]string{}}
	lines := strings.Split(string(source), "\n")
	knownIDs := RuleIDs()
	for _, comment := range comments {
		text := strings.TrimSpace(comment.Text)
		var rest string
		wholeFile := false
		switch {
		case strings.HasPrefix(text, ignoreFilePrefix):
			rest, wholeFile = text[len(ignoreFilePrefix):], true
		case strings.HasPrefix(text, ignorePrefix):
			rest = text[len(ignorePrefix):]
		default:
			continue
		}
		// "concourse-lint:ignored" or similar is no suppression comment.
		if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
			continue
		}
		ids := result.validIDs(comment, rest, knownIDs)
		if wholeFile {
			result.wholeFile = append(result.wholeFile, ids...)
			continue
		}
		line := comment.Position.Line
		if standsAlone(lines, comment.Position) {
			line++
		}
		result.byLine[line] = append(result.byLine[line], ids...)
	}
	return result
}

// validIDs returns the known rule IDs of the list after the prefix and adds a
// CL015 for a missing list and for every unknown ID.
func (result *suppressions) validIDs(comment model.Comment, rest string, knownIDs []string) []string {
	fields := strings.Fields(rest)
	if len(fields) == 0 || fields[0] == "--" || fields[0] == "#" {
		result.problem(comment, "suppression comment without a rule ID suppresses nothing", "name the rules, e.g. concourse-lint:ignore CS006")
		return nil
	}
	var valid []string
	for _, id := range strings.Split(fields[0], ",") {
		if slices.Contains(knownIDs, id) {
			valid = append(valid, id)
			continue
		}
		result.problem(comment, fmt.Sprintf("unknown rule ID %q in suppression comment", id), analysis.DidYouMean(id, knownIDs))
	}
	return valid
}

func (result *suppressions) problem(comment model.Comment, message, hint string) {
	result.problems = append(result.problems, analysis.Finding{
		Rule:     invalidSuppressionRule,
		Severity: analysis.SeverityWarning,
		Position: comment.Position,
		Message:  message,
		Hint:     hint,
	})
}

// standsAlone reports whether the comment is the only thing on its line.
func standsAlone(lines []string, position model.Position) bool {
	if position.Line < 1 || position.Line > len(lines) {
		return false
	}
	line := lines[position.Line-1]
	before := line[:min(max(position.Column-1, 0), len(line))]
	return strings.TrimSpace(before) == ""
}

// apply drops the suppressed findings and adds the CL015 findings.
func (result suppressions) apply(findings []analysis.Finding) []analysis.Finding {
	var kept []analysis.Finding
	for _, finding := range findings {
		if result.suppresses(finding) {
			continue
		}
		kept = append(kept, finding)
	}
	return append(kept, result.problems...)
}

func (result suppressions) suppresses(finding analysis.Finding) bool {
	if slices.Contains(unsuppressible, finding.Rule) {
		return false
	}
	return slices.Contains(result.wholeFile, finding.Rule) || slices.Contains(result.byLine[finding.Position.Line], finding.Rule)
}
