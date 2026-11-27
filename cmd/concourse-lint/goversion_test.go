package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoVersionsMatch guards the Go version of the builds. go.mod names the
// lowest Go release the module needs; the workflows install the newest patch
// release of the same minor version ("1.27.x" with check-latest), so that
// govulncheck and the release binaries see the standard library with all its
// security fixes. go-version-file would install exactly the version in
// go.mod. Every module of the repository uses the same minor version.
func TestGoVersionsMatch(t *testing.T) {
	minorVersion := goMinorVersion(t, "../../go.mod")

	for _, modulePath := range []string{
		"../../tools/go.mod",
		"../../tools/release/go.mod",
		"../../conformance/concourse/go.mod",
	} {
		if moduleMinor := goMinorVersion(t, modulePath); moduleMinor != minorVersion {
			t.Errorf("%s: go %s, want the minor version of go.mod (%s)", modulePath, moduleMinor, minorVersion)
		}
	}

	workflowPaths, err := filepath.Glob("../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	wantGoVersion := `go-version: "` + minorVersion + `.x"`
	for _, workflowPath := range workflowPaths {
		workflow, err := os.ReadFile(workflowPath)
		if err != nil {
			t.Fatal(err)
		}
		workflowName := filepath.Base(workflowPath)
		lines := strings.Split(string(workflow), "\n")
		for lineIndex, line := range lines {
			if !strings.Contains(line, "uses: actions/setup-go@") {
				continue
			}
			settings := setupStepSettings(lines, lineIndex)
			if !containsLine(settings, wantGoVersion) {
				t.Errorf("%s:%d: setup-go without %s", workflowName, lineIndex+1, wantGoVersion)
			}
			if !containsLine(settings, "check-latest: true") {
				t.Errorf("%s:%d: setup-go without check-latest: true", workflowName, lineIndex+1)
			}
			for _, setting := range settings {
				if strings.HasPrefix(setting, "go-version-file:") {
					t.Errorf("%s:%d: setup-go with %s installs exactly that version", workflowName, lineIndex+1, setting)
				}
			}
		}
	}
}

// goMinorVersion returns the minor version ("1.27") of the go line of a
// go.mod file.
func goMinorVersion(t *testing.T, goModPath string) string {
	t.Helper()
	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(goMod), "\n") {
		version, found := strings.CutPrefix(strings.TrimSpace(line), "go ")
		if !found {
			continue
		}
		versionParts := strings.Split(version, ".")
		if len(versionParts) < 2 {
			t.Fatalf("%s: go %s has no minor version", goModPath, version)
		}
		return versionParts[0] + "." + versionParts[1]
	}
	t.Fatalf("%s: no go line", goModPath)
	return ""
}

// setupStepSettings returns the trimmed lines of the step that starts at
// usesIndex, up to the next step or the end of the job, without comments.
func setupStepSettings(lines []string, usesIndex int) []string {
	stepIndent := len(lines[usesIndex]) - len(strings.TrimLeft(lines[usesIndex], " "))
	var settings []string
	for _, line := range lines[usesIndex+1:] {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}
		lineIndent := len(line) - len(strings.TrimLeft(line, " "))
		if lineIndent <= stepIndent {
			break
		}
		settings = append(settings, trimmedLine)
	}
	return settings
}

func containsLine(lines []string, wantLine string) bool {
	for _, line := range lines {
		if line == wantLine {
			return true
		}
	}
	return false
}
