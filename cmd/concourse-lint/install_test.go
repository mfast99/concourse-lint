package main

import (
	"os"
	"strings"
	"testing"
)

// TestGoModAllowsGoInstall guards "go install …@version": it refuses a module
// whose go.mod has a replace or exclude directive. A fix that is not released
// upstream goes into a fork with its own module path instead (the YAML parser
// github.com/mfast99/go-yaml is one).
func TestGoModAllowsGoInstall(t *testing.T) {
	goMod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for lineIndex, line := range strings.Split(string(goMod), "\n") {
		directive, _, _ := strings.Cut(strings.TrimSpace(line), " ")
		if directive == "replace" || directive == "exclude" {
			t.Errorf("go.mod:%d: %q breaks go install", lineIndex+1, strings.TrimSpace(line))
		}
	}
}
