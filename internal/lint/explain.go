package lint

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/mfast99/concourse-lint/docs"
	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/config"
)

// severityOff is the effective severity of a rule the config switches off.
const severityOff = "off"

// RuleInfo describes a rule under a config file: the JSON of
// concourse-lint rules and the MCP tool list_rules (spec/README.md, "Rules
// and explain").
type RuleInfo struct {
	ID string `json:"id"`
	// Severity is the effective severity: the configured one, "off", or
	// the default.
	Severity        string `json:"severity"`
	DefaultSeverity string `json:"default_severity"`
	Title           string `json:"title"`
	Docs            string `json:"docs"`
}

// RuleExplanation is a rule with the sections of its page: the JSON of
// concourse-lint explain and the MCP tool explain_rule.
type RuleExplanation struct {
	ID              string `json:"id"`
	Severity        string `json:"severity"`
	DefaultSeverity string `json:"default_severity"`
	Title           string `json:"title"`
	// The sections of the rule page as Markdown, with absolute links.
	Checked  string `json:"checked"`
	Example  string `json:"example"`
	Fix      string `json:"fix"`
	Suppress string `json:"suppress"`
	Docs     string `json:"docs"`
}

// RuleList describes every rule, sorted by ID, with the severity under the
// config. A nil config means the defaults.
func RuleList(ruleConfig *config.Config) []RuleInfo {
	var list []RuleInfo
	for _, rule := range Rules() {
		list = append(list, describe(rule, ruleConfig))
	}
	return list
}

// ExplainRule describes the rule with the ID (case-insensitive) and the
// sections of its page. An unknown ID is an error with a "did you mean"
// hint when one is close.
func ExplainRule(id string, ruleConfig *config.Config) (RuleExplanation, error) {
	wanted := strings.ToUpper(strings.TrimSpace(id))
	for _, rule := range Rules() {
		if rule.ID != wanted {
			continue
		}
		info := describe(rule, ruleConfig)
		page, _ := docs.Rule(rule.ID)
		return RuleExplanation{
			ID:              info.ID,
			Severity:        info.Severity,
			DefaultSeverity: info.DefaultSeverity,
			Title:           info.Title,
			Checked:         absoluteLinks(page.Checked),
			Example:         absoluteLinks(page.Example),
			Fix:             absoluteLinks(page.Fix),
			Suppress:        absoluteLinks(page.Suppress),
			Docs:            info.Docs,
		}, nil
	}
	message := fmt.Sprintf("unknown rule %q", id)
	if hint := analysis.DidYouMean(wanted, RuleIDs()); hint != "" {
		message += " (" + hint + ")"
	}
	return RuleExplanation{}, fmt.Errorf("%s", message)
}

// FindConfig finds and loads the config file for the folder, as for stdin
// in the CLI: the search starts in the folder and ends at the Git root. It
// returns the path of the file, or "" and nil when there is none.
func FindConfig(folder string) (string, *config.Config, error) {
	path, err := config.Find(folder)
	if err != nil || path == "" {
		return "", nil, err
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		return "", nil, err
	}
	return path, loaded, nil
}

// LoadOrFindConfig loads the config file at explicitPath (--config), or
// searches from the folder when it is "". It returns the path of the file
// it used, or "" and nil when the search finds none.
func LoadOrFindConfig(explicitPath, folder string) (string, *config.Config, error) {
	if explicitPath == "" {
		return FindConfig(folder)
	}
	loaded, err := LoadConfig(explicitPath)
	if err != nil {
		return "", nil, err
	}
	return explicitPath, loaded, nil
}

func describe(rule analysis.Rule, ruleConfig *config.Config) RuleInfo {
	severity := string(ruleConfig.Severity(rule.ID, rule.Severity))
	if ruleConfig.IsOff(rule.ID) {
		severity = severityOff
	}
	return RuleInfo{
		ID:              rule.ID,
		Severity:        severity,
		DefaultSeverity: string(rule.Severity),
		Title:           rule.Title,
		Docs:            analysis.DocsURL(rule.ID),
	}
}

// markdownLink matches the target of a Markdown link: "](target)".
var markdownLink = regexp.MustCompile(`\]\(([^)\s]+)\)`)

// absoluteLinks resolves the relative link targets of a rule page against
// the address of the published pages, e.g. ../../spec/README.md#suppression.
func absoluteLinks(markdown string) string {
	base, err := url.Parse(analysis.DocsBaseURL)
	if err != nil {
		return markdown
	}
	return markdownLink.ReplaceAllStringFunc(markdown, func(match string) string {
		target := markdownLink.FindStringSubmatch(match)[1]
		reference, err := url.Parse(target)
		if err != nil || reference.IsAbs() {
			return match
		}
		return "](" + base.ResolveReference(reference).String() + ")"
	})
}
