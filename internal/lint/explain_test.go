package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/config"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestRuleListDefaults: without a config file every rule has its default
// severity (spec/README.md, "Rules and explain").
func TestRuleListDefaults(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	list := RuleList(nil)
	if len(list) != len(catalog) {
		t.Errorf("%d rules, catalog %d", len(list), len(catalog))
	}
	for index, rule := range list {
		if index > 0 && list[index-1].ID >= rule.ID {
			t.Errorf("not sorted by ID at %s", rule.ID)
		}
		want := string(catalog[rule.ID])
		if rule.Severity != want || rule.DefaultSeverity != want || rule.Title == "" || rule.Docs != analysis.DocsURL(rule.ID) {
			t.Errorf("%+v does not fit the catalog (%s)", rule, want)
		}
	}
}

// TestRuleListWithConfig: a configured severity and "off" become the
// severity; the default stays in default_severity.
func TestRuleListWithConfig(t *testing.T) {
	loaded := parseConfig(t, "rules:\n  CS005: error\n  CL018: off\n  CB002: {min_interval: 30s}\n")
	severities := map[string][2]string{}
	for _, rule := range RuleList(loaded) {
		severities[rule.ID] = [2]string{rule.Severity, rule.DefaultSeverity}
	}
	wanted := map[string][2]string{
		"CS005": {"error", "warning"},
		"CL018": {"off", "warning"},
		"CB002": {"warning", "warning"},
		"CL002": {"error", "error"},
	}
	for id, want := range wanted {
		if severities[id] != want {
			t.Errorf("%s: severity, default = %v, want %v", id, severities[id], want)
		}
	}
}

// TestExplainRule: every rule of the catalog, the ID in any case, with all
// sections of its page.
func TestExplainRule(t *testing.T) {
	for id, severity := range spectest.LoadCatalog(t) {
		explanation, err := ExplainRule(strings.ToLower(id), nil)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		if explanation.ID != id || explanation.Severity != string(severity) || explanation.DefaultSeverity != string(severity) || explanation.Docs != analysis.DocsURL(id) {
			t.Errorf("%s: %+v", id, explanation)
		}
		if explanation.Title == "" || explanation.Checked == "" || explanation.Example == "" || explanation.Fix == "" || explanation.Suppress == "" {
			t.Errorf("%s: title and every section must be set: %+v", id, explanation)
		}
	}
}

// TestExplainRuleAbsoluteLinks: relative links of a rule page point to the
// published repository, because the reader of the JSON or the terminal has
// no repository to resolve them in.
func TestExplainRuleAbsoluteLinks(t *testing.T) {
	explanation, err := ExplainRule("CL002", nil)
	if err != nil {
		t.Fatal(err)
	}
	wanted := "[suppression](https://github.com/mfast99/concourse-lint/blob/main/spec/README.md#suppression)"
	if !strings.Contains(explanation.Suppress, wanted) {
		t.Errorf("suppress = %q, want it to contain %q", explanation.Suppress, wanted)
	}
	for id := range spectest.LoadCatalog(t) {
		explanation, _ := ExplainRule(id, nil)
		for _, section := range []string{explanation.Checked, explanation.Example, explanation.Fix, explanation.Suppress} {
			if strings.Contains(section, "](../") || strings.Contains(section, "](./") {
				t.Errorf("%s: relative link left in %q", id, section)
			}
		}
	}
}

func TestExplainRuleWithConfig(t *testing.T) {
	explanation, err := ExplainRule("CS005", parseConfig(t, "rules:\n  CS005: error\n"))
	if err != nil {
		t.Fatal(err)
	}
	if explanation.Severity != "error" || explanation.DefaultSeverity != "warning" {
		t.Errorf("severity %q, default %q, want error and warning", explanation.Severity, explanation.DefaultSeverity)
	}
}

func TestExplainRuleUnknown(t *testing.T) {
	cases := map[string]string{
		"CL02":    `unknown rule "CL02" (did you mean "CL002"?)`,
		"nothing": `unknown rule "nothing"`,
		"":        `unknown rule ""`,
	}
	for id, want := range cases {
		_, err := ExplainRule(id, nil)
		if err == nil || err.Error() != want {
			t.Errorf("ExplainRule(%q) error = %v, want %q", id, err, want)
		}
	}
}

// TestFindConfig: the search starts in the folder and ends at the Git root.
func TestFindConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "ci", "pipelines")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	path, found, err := FindConfig(nested)
	if path != "" || found != nil || err != nil {
		t.Errorf("without a config file: %q, %v, %v; want nothing", path, found, err)
	}

	configPath := filepath.Join(root, ".concourse-lint.yml")
	if err := os.WriteFile(configPath, []byte("rules:\n  CL018: off\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, found, err = FindConfig(nested)
	if path != configPath || err != nil || !found.IsOff("CL018") {
		t.Errorf("got %q, %v; want %q with CL018 off", path, err, configPath)
	}

	if err := os.WriteFile(configPath, []byte("rules:\n  CL999: off\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := FindConfig(nested); err == nil || !strings.Contains(err.Error(), "CL999") {
		t.Errorf("broken config: error = %v, want one naming CL999", err)
	}
}

func parseConfig(t *testing.T, text string) *config.Config {
	t.Helper()
	parsed, err := config.Parse([]byte(text), RuleIDs())
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

// TestLoadOrFindConfig: an explicit path is loaded, else the search runs.
func TestLoadOrFindConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	found := filepath.Join(root, ".concourse-lint.yml")
	explicit := filepath.Join(root, "strict.yml")
	for path, content := range map[string]string{found: "rules:\n  CL018: off\n", explicit: "rules:\n  CL018: info\n"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path, loaded, err := LoadOrFindConfig("", root)
	if path != found || err != nil || !loaded.IsOff("CL018") {
		t.Errorf("search: %q, %v; want %q with CL018 off", path, err, found)
	}
	path, loaded, err = LoadOrFindConfig(explicit, root)
	if path != explicit || err != nil || loaded.IsOff("CL018") {
		t.Errorf("explicit: %q, %v; want %q with CL018 on", path, err, explicit)
	}
	if _, _, err := LoadOrFindConfig(filepath.Join(root, "missing.yml"), root); err == nil {
		t.Error("a missing explicit file is no error")
	}
}
