package lint

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestRules: Rules lists every rule of spec/catalog.txt once, sorted by ID,
// with the severity and title of the catalog (list_rules of the MCP server).
func TestRules(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(spectest.Dir(), "catalog.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var wanted []analysis.Rule
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		wanted = append(wanted, analysis.Rule{ID: fields[0], Severity: analysis.Severity(fields[1]), Title: strings.Join(fields[2:], " ")})
	}
	slices.SortFunc(wanted, func(first, second analysis.Rule) int { return strings.Compare(first.ID, second.ID) })

	got := Rules()
	if len(got) != len(wanted) {
		t.Errorf("Rules() has %d rules, catalog %d", len(got), len(wanted))
	}
	for index := range min(len(got), len(wanted)) {
		rule, want := got[index], wanted[index]
		if rule.ID != want.ID || rule.Severity != want.Severity || rule.Title != want.Title {
			t.Errorf("rule %d = %s %s %q, want %s %s %q", index, rule.ID, rule.Severity, rule.Title, want.ID, want.Severity, want.Title)
		}
		if rule.Run != nil {
			t.Errorf("%s: Run is set; Rules only describes the rules", rule.ID)
		}
	}
}

// TestRuleIDs: RuleIDs are the IDs of Rules.
func TestRuleIDs(t *testing.T) {
	var fromRules []string
	for _, rule := range Rules() {
		fromRules = append(fromRules, rule.ID)
	}
	if !slices.Equal(RuleIDs(), fromRules) {
		t.Errorf("RuleIDs() = %v, want %v", RuleIDs(), fromRules)
	}
}
