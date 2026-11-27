// Package spectest runs the specification test cases from spec/. It only knows
// the format of the test cases (comments "# want ...") and the rule catalog,
// nothing about how the rules are implemented.
//
// Used by internal/lint (core) and later by lsp and mcp, so that all
// front ends are checked against the same cases.
package spectest

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

var (
	wantPattern         = regexp.MustCompile(`#\s*want\s+((?:[A-Z]{2}\d{3}\s*)+)$`)
	wantAnywherePattern = regexp.MustCompile(`^#\s*want-anywhere\s+((?:[A-Z]{2}\d{3}\s*)+)$`)
)

// Dir is the absolute path to spec/, no matter which package the test runs in.
func Dir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "spec")
}

// Files returns all test cases: every *.yml below spec/, at any depth
// (e.g. spec/valid/01-minimal.yml, spec/rules/correctness/CL002/alias.yml),
// except the config scenarios in spec/config/ (see ConfigScenarios).
func Files(t testing.TB) []string {
	var files []string
	err := filepath.WalkDir(Dir(), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path == ConfigDir() {
			return filepath.SkipDir
		}
		if !entry.IsDir() && filepath.Ext(path) == ".yml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		t.Fatalf("no test cases found: %v", err)
	}
	return files
}

// ConfigDir is spec/config/: one folder per config scenario.
func ConfigDir() string {
	return filepath.Join(Dir(), "config")
}

// ConfigScenarios returns the scenario folders in spec/config/.
func ConfigScenarios(t testing.TB) []string {
	entries, err := os.ReadDir(ConfigDir())
	if err != nil {
		t.Fatal(err)
	}
	var scenarios []string
	for _, entry := range entries {
		if entry.IsDir() {
			scenarios = append(scenarios, filepath.Join(ConfigDir(), entry.Name()))
		}
	}
	if len(scenarios) == 0 {
		t.Fatal("no config scenarios found")
	}
	return scenarios
}

// ConfigScenarioFiles returns the pipelines of a config scenario (every
// *.yml at any depth that is not a config file) and its config files.
func ConfigScenarioFiles(t testing.TB, scenario string) (pipelines, configs []string) {
	err := filepath.WalkDir(scenario, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		switch {
		case entry.Name() == ".concourse-lint.yml" || entry.Name() == ".concourse-lint.yaml":
			configs = append(configs, path)
		case filepath.Ext(path) == ".yml":
			pipelines = append(pipelines, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return pipelines, configs
}

// Marker reports whether the text has a comment line "# <marker>", e.g.
// "# want-excluded" or "# want-config-error".
func Marker(source []byte, marker string) bool {
	for _, line := range strings.Split(string(source), "\n") {
		if strings.TrimSpace(line) == "# "+marker {
			return true
		}
	}
	return false
}

// Areas maps a rule ID prefix to its folder under spec/rules/ (and under
// internal/rules/).
var Areas = map[string]string{
	"CL": "correctness",
	"CS": "security",
	"CB": "bestpractice",
}

// Name turns a path into a readable test name, e.g. "rules/correctness/CL002/alias".
func Name(file string) string {
	relativePath, _ := filepath.Rel(Dir(), file)
	return strings.TrimSuffix(filepath.ToSlash(relativePath), ".yml")
}

// Expectations are the expectations taken from the comments of a file. The
// values are counts: every rule ID stands for exactly one finding, so
// "# want CL002 CL002" expects two findings on that line.
type Expectations struct {
	// "12:CL002" -> number of findings on line 12.
	AtLine map[string]int
	// "CL000" -> number of findings on any line.
	Anywhere map[string]int
}

// Rules returns all rule IDs that appear in the expectations.
func (expectations Expectations) Rules() []string {
	ruleSet := map[string]bool{}
	for key := range expectations.AtLine {
		ruleSet[strings.SplitN(key, ":", 2)[1]] = true
	}
	for rule := range expectations.Anywhere {
		ruleSet[rule] = true
	}
	var rules []string
	for rule := range ruleSet {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	return rules
}

// ParseExpectations reads "# want ..." and "# want-anywhere ..." from the text.
func ParseExpectations(source []byte) Expectations {
	expectations := Expectations{AtLine: map[string]int{}, Anywhere: map[string]int{}}
	scanner := bufio.NewScanner(bytes.NewReader(source))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		text := strings.TrimSpace(scanner.Text())
		if match := wantAnywherePattern.FindStringSubmatch(text); match != nil {
			for _, rule := range strings.Fields(match[1]) {
				expectations.Anywhere[rule]++
			}
			continue
		}
		if match := wantPattern.FindStringSubmatch(text); match != nil {
			for _, rule := range strings.Fields(match[1]) {
				expectations.AtLine[fmt.Sprintf("%d:%s", lineNumber, rule)]++
			}
		}
	}
	return expectations
}

// Catalog maps every rule ID to its severity (from spec/catalog.txt).
type Catalog map[string]analysis.Severity

// LoadCatalog reads spec/catalog.txt.
func LoadCatalog(t testing.TB) Catalog {
	content, err := os.ReadFile(filepath.Join(Dir(), "catalog.txt"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := Catalog{}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		catalog[fields[0]] = analysis.Severity(fields[1])
	}
	return catalog
}

// Compare compares findings with expectations and catalog. An empty result
// means: everything matches.
//
// Findings are counted, not just ticked off: each expectation is used up by
// exactly one finding. A finding is first matched against the "# want" of its
// line, only then against "# want-anywhere". So a duplicate finding (e.g. one
// per use of an anchor) is reported as unexpected.
func Compare(expectations Expectations, catalog Catalog, got []analysis.Finding) []string {
	// Expectations that are still open; matching findings use them up.
	openAtLine := maps.Clone(expectations.AtLine)
	openAnywhere := maps.Clone(expectations.Anywhere)

	var problems []string
	var unmatched []analysis.Finding
	for _, finding := range got {
		key := fmt.Sprintf("%d:%s", finding.Position.Line, finding.Rule)
		if openAtLine[key] > 0 {
			openAtLine[key]--
		} else {
			unmatched = append(unmatched, finding)
		}
	}
	for _, finding := range unmatched {
		if openAnywhere[finding.Rule] > 0 {
			openAnywhere[finding.Rule]--
			continue
		}
		key := fmt.Sprintf("%d:%s", finding.Position.Line, finding.Rule)
		problems = append(problems, fmt.Sprintf("unexpected: %s (%s)", key, finding.Message))
	}
	for key, count := range openAtLine {
		for range count {
			problems = append(problems, "missing:    "+key)
		}
	}
	for rule, count := range openAnywhere {
		for range count {
			problems = append(problems, "missing:    anywhere:"+rule)
		}
	}

	for _, finding := range got {
		if catalogSeverity, inCatalog := catalog[finding.Rule]; !inCatalog {
			problems = append(problems, "unknown rule: "+finding.Rule)
		} else if finding.Severity != catalogSeverity {
			problems = append(problems, fmt.Sprintf("severity %s: %s instead of %s", finding.Rule, finding.Severity, catalogSeverity))
		}
	}
	sort.Strings(problems)
	return problems
}
