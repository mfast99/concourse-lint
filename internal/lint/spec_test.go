package lint

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/parse"
	"github.com/mfast99/concourse-lint/internal/rules"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// TestSpec: every file under spec/ produces exactly the expected findings.
func TestSpec(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	for _, file := range spectest.Files(t) {
		t.Run(spectest.Name(file), func(t *testing.T) {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			expectations := spectest.ParseExpectations(source)
			for _, problem := range spectest.Compare(expectations, catalog, Check(file, source)) {
				t.Error(problem)
			}
		})
	}
}

// TestCatalogCoverage: every rule in the catalog has a test case, and every
// test case names only rules from the catalog.
func TestCatalogCoverage(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	covered := map[string]bool{}
	for _, file := range spectest.Files(t) {
		source, _ := os.ReadFile(file)
		for _, rule := range spectest.ParseExpectations(source).Rules() {
			covered[rule] = true
			if _, inCatalog := catalog[rule]; !inCatalog {
				t.Errorf("%s names unknown rule %s", spectest.Name(file), rule)
			}
		}
	}
	for rule := range catalog {
		if !covered[rule] {
			t.Errorf("rule %s has no test case", rule)
		}
	}
}

// minScenarios is the minimum number of scenario files per rule. More are
// welcome: one file per distinct situation, see spec/README.md.
const minScenarios = 1

// TestSpecLayout enforces the structure of spec/rules/ (see spec/README.md):
//
//	spec/rules/<area>/<ID>/<scenario>.yml   must expect <ID>
//	spec/rules/<area>/<ID>/negatives.yml    must NOT expect <ID>
//
// Every rule has at least one scenario file, and every warning or
// info rule has a negatives.yml. CL000 lives in spec/syntax/ instead.
func TestSpecLayout(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	fileNamePattern := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*\.yml$`)
	scenarios := map[string]int{}
	negatives := map[string]bool{}

	for _, file := range spectest.Files(t) {
		name := spectest.Name(file)
		parts := strings.Split(name, "/")
		if parts[0] == "syntax" {
			scenarios["CL000"]++
		}
		if parts[0] != "rules" {
			continue
		}
		if len(parts) != 4 {
			t.Errorf("%s: expected rules/<area>/<ID>/<scenario>.yml", name)
			continue
		}
		area, id, base := parts[1], parts[2], filepath.Base(file)
		if _, inCatalog := catalog[id]; !inCatalog {
			t.Errorf("%s: folder %s is not a rule from the catalog", name, id)
			continue
		}
		if want := spectest.Areas[id[:2]]; area != want {
			t.Errorf("%s: %s belongs in rules/%s/", name, id, want)
		}
		if !fileNamePattern.MatchString(base) {
			t.Errorf("%s: file names are lowercase-kebab-case", name)
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		expects := slices.Contains(spectest.ParseExpectations(source).Rules(), id)
		if base == "negatives.yml" {
			negatives[id] = true
			if expects {
				t.Errorf("%s: negatives.yml must not expect %s", name, id)
			}
			continue
		}
		scenarios[id]++
		if !expects {
			t.Errorf("%s: scenario file does not expect %s", name, id)
		}
	}

	for id, severity := range catalog {
		if count := scenarios[id]; count < minScenarios {
			t.Errorf("rule %s has %d scenario files, want at least %d", id, count, minScenarios)
		}
		if severity != analysis.SeverityError && !negatives[id] {
			t.Errorf("rule %s is a %s and needs a negatives.yml", id, severity)
		}
	}
}

// TestCatalogImplemented: every rule in the catalog is registered in code, with
// the same severity. The parser rules (parse.Rules) come from the parser,
// CL015 from lint itself.
func TestCatalogImplemented(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	registered := map[string]bool{invalidSuppressionRule: true}
	for _, rule := range parse.Rules {
		registered[rule.ID] = true
	}
	for _, rule := range rules.All() {
		registered[rule.ID] = true
		if catalogSeverity, inCatalog := catalog[rule.ID]; !inCatalog {
			t.Errorf("rule %s is not in the catalog", rule.ID)
		} else if rule.Severity != catalogSeverity {
			t.Errorf("rule %s: severity %s in code, %s in catalog", rule.ID, rule.Severity, catalogSeverity)
		}
	}
	for rule := range catalog {
		if !registered[rule] {
			t.Errorf("rule %s is not implemented yet", rule)
		}
	}
}

// TestRuleDocs: every rule in the catalog has a page docs/rules/<ID>.md that
// starts with its ID and title.
func TestRuleDocs(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	docsDir := filepath.Join(spectest.Dir(), "..", "docs", "rules")
	for rule := range catalog {
		content, err := os.ReadFile(filepath.Join(docsDir, rule+".md"))
		if err != nil {
			t.Errorf("rule %s has no page in docs/rules: %v", rule, err)
			continue
		}
		if !strings.HasPrefix(string(content), "# "+rule+": ") {
			t.Errorf("docs/rules/%s.md must start with \"# %s: <title>\"", rule, rule)
		}
	}
}
