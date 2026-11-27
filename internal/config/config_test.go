package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mfast99/concourse-lint/internal/config"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestFindStopsAtGitRoot: a config above the Git root does not count.
func TestFindStopsAtGitRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".concourse-lint.yml"), "rules: {}\n")
	repository := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repository, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repository, "ci"), 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := config.Find(filepath.Join(repository, "ci"))
	if err != nil || found != "" {
		t.Errorf("Find = %q, %v; want no config (it is above the Git root)", found, err)
	}
	writeFile(t, filepath.Join(repository, ".concourse-lint.yaml"), "rules: {}\n")
	found, err = config.Find(filepath.Join(repository, "ci"))
	if err != nil || found != filepath.Join(repository, ".concourse-lint.yaml") {
		t.Errorf("Find = %q, %v; want the .yaml file at the Git root", found, err)
	}
}

// TestFindBothNames: .yml and .yaml in one folder is an error.
func TestFindBothNames(t *testing.T) {
	folder := t.TempDir()
	writeFile(t, filepath.Join(folder, ".concourse-lint.yml"), "rules: {}\n")
	writeFile(t, filepath.Join(folder, ".concourse-lint.yaml"), "rules: {}\n")
	if _, err := config.Find(folder); err == nil {
		t.Error("Find must fail when both file names exist")
	}
}

// TestNilConfig: a nil config is the default configuration.
func TestNilConfig(t *testing.T) {
	var defaults *config.Config
	if defaults.IsOff("CS008") || defaults.Severity("CS008", "warning") != "warning" || defaults.Excluded("x.yml") {
		t.Error("a nil config must change nothing")
	}
}

// TestParseErrors: the messages name the problem and the line and column
// of the offending key or value.
func TestParseErrors(t *testing.T) {
	known := []string{"CL000", "CS006", "CB002"}
	tests := map[string]string{
		"rules: {CS060: off}\n":                  `1:9: unknown rule "CS060" in rules: (did you mean "CS006"?)`,
		"rules: {CL000: off}\n":                  "1:16: rules: CL000 cannot be switched off",
		"rules: {CS006: fatal}\n":                `1:16: rules: CS006: unknown severity "fatal" (use error, warning, info or off)`,
		"rules: {CB002: {min_interval: soon}}\n": "1:31: rules: CB002: min_interval: must be a Go duration like 30s or 2m, not soon",
		"rules:\n  CB002:\n    speed: 1\n":       "3:5: rules: CB002: unknown option \"speed\" (known: severity, min_interval)",
		"exclude: [a, \"[\"]\n":                  `1:14: exclude: "[" is no valid glob`,
		"# comment\nrulez: {}\n":                 `2:1: unknown field "rulez"`,
		"%YAML 1.1\n---\nrules: {CS060: off}\n":  `3:9: unknown rule "CS060" in rules: (did you mean "CS006"?)`,
	}
	for source, want := range tests {
		_, err := config.Parse([]byte(source), known)
		if err == nil || err.Error() != want {
			t.Errorf("Parse(%q) error = %v, want %q", source, err, want)
		}
	}
}

// TestLoadErrorNamesFile: a config error starts with file:line:col, like a
// finding.
func TestLoadErrorNamesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".concourse-lint.yml")
	writeFile(t, path, "rules:\n  CS006: fatal\n")
	_, err := config.Load(path, []string{"CS006"})
	want := path + `:2:10: rules: CS006: unknown severity "fatal" (use error, warning, info or off)`
	if err == nil || err.Error() != want {
		t.Errorf("Load error = %v, want %q", err, want)
	}
}

// TestExcludeGlobThatCrashes: the glob library panics for "ci{" on the path
// "ci" (an empty alternative at the end). Such a pattern excludes nothing
// instead of crashing the CLI or the language server.
func TestExcludeGlobThatCrashes(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, ".concourse-lint.yml")
	writeFile(t, path, "exclude: [\"ci{\", generated]\n")
	loaded, err := config.Load(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Excluded(filepath.Join(folder, "ci", "pipeline.yml")) {
		t.Error("ci/pipeline.yml is excluded, want not excluded")
	}
	if !loaded.Excluded(filepath.Join(folder, "generated", "pipeline.yml")) {
		t.Error("generated/pipeline.yml is not excluded, want excluded")
	}
}
