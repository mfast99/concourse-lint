// Package archtest guards the dependency direction between packages.
//
// The core (model, analysis, parse, rules, lint, report) never imports a
// front end, and front ends (cmd, lsp, mcp) never call parse or rules
// directly: they go through lint, and the editor features through
// navigate. See CLAUDE.md, "Layout".
package archtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/mfast99/concourse-lint/"

// allowed lists, for every package of this module, which other packages of
// this module it may import (non-test files only). Standard library and
// third-party imports are not checked here.
//
// A new package must get an entry here; that forces a conscious decision
// about where it sits in the layers.
var allowed = map[string][]string{
	// core
	"internal/model":    {},
	"internal/analysis": {"internal/model"},
	"internal/flow":     {"internal/model"},
	"internal/navigate": {"internal/model"},
	"internal/complete": {"internal/model", "internal/parse", "internal/navigate", "internal/flow"},
	// the rule pages, embedded for the editor (M6)
	"docs":            {},
	"internal/config": {"internal/model", "internal/analysis"},
	"internal/parse":  {"internal/model", "internal/analysis"},
	"internal/rules": {
		"internal/model", "internal/analysis",
		"internal/rules/correctness", "internal/rules/security", "internal/rules/bestpractice",
	},
	"internal/rules/correctness":  {"internal/model", "internal/analysis", "internal/flow", "internal/navigate"},
	"internal/rules/security":     {"internal/model", "internal/analysis"},
	"internal/rules/bestpractice": {"internal/model", "internal/analysis"},
	"internal/lint":               {"internal/model", "internal/analysis", "internal/config", "internal/parse", "internal/rules", "docs"},
	"internal/report":             {"internal/model", "internal/analysis"},

	// front ends: only lint.Check plus the shared types and output formats
	"internal/lsp":       {"internal/model", "internal/analysis", "internal/config", "internal/lint", "internal/navigate", "internal/complete", "internal/report", "docs"},
	"internal/mcp":       {"internal/model", "internal/analysis", "internal/lint", "internal/report", "docs"},
	"cmd/concourse-lint": {"internal/model", "internal/analysis", "internal/lint", "internal/report", "internal/lsp", "internal/mcp"},

	// test helpers
	"internal/spectest": {"internal/model", "internal/analysis"},
	"internal/archtest": {},
}

// packageInfo is the part of the `go list -json` output this test needs.
type packageInfo struct {
	ImportPath string
	Imports    []string
}

func TestDependencyDirection(t *testing.T) {
	packages := listPackages(t)

	seen := map[string]bool{}
	for _, info := range packages {
		name := strings.TrimPrefix(info.ImportPath, module)
		seen[name] = true

		allowedImports, isListed := allowed[name]
		if !isListed {
			t.Errorf("%s: package is missing in the allowed table of arch_test.go; add it and decide which packages it may import", name)
			continue
		}
		for _, importPath := range info.Imports {
			// Standard library or third party.
			if !strings.HasPrefix(importPath, module) {
				continue
			}
			dependency := strings.TrimPrefix(importPath, module)
			if !slices.Contains(allowedImports, dependency) {
				t.Errorf("%s must not import %s (allowed: %s)", name, dependency, strings.Join(allowedImports, ", "))
			}
		}
	}

	// A stale entry usually means a package was renamed and the table was
	// not updated, so its rules silently stopped applying.
	for name := range allowed {
		if !seen[name] {
			t.Errorf("%s: entry in the allowed table, but no such package exists", name)
		}
	}
}

// listPackages asks the go tool for every package of the module and its
// direct imports.
func listPackages(t *testing.T) []packageInfo {
	t.Helper()
	output, err := exec.Command("go", "list", "-json=ImportPath,Imports", module+"...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	// go list -json prints one JSON object per package, one after another
	// (not a JSON array), so they are decoded in a loop until io.EOF.
	var packages []packageInfo
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var info packageInfo
		err := decoder.Decode(&info)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		packages = append(packages, info)
	}
	if len(packages) == 0 {
		t.Fatal("go list returned no packages")
	}
	return packages
}
