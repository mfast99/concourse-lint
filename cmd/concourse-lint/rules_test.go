package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// projectWith creates a Git project with the files and returns its folder.
func projectWith(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files[".git/HEAD"] = ""
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestRulesText: one aligned line per rule, then the pointer to explain
// (spec/README.md, "Rules and explain").
func TestRulesText(t *testing.T) {
	result := runIn(t, projectWith(t, map[string]string{}), "", "rules")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("exit code %d, stderr %q", result.exitCode, result.stderr)
	}
	lines := strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n")
	if len(lines) != 37+2 {
		t.Fatalf("%d lines, want 37 rules, a blank line and the pointer:\n%s", len(lines), result.stdout)
	}
	wanted := map[int]string{
		0:  "CB001  info     Resource type is not used",
		5:  "CL002  error    get/put or a group refers to a missing resource",
		36: "CS009  info     load_var reveals the value in the log",
		37: "",
		38: "Details: concourse-lint explain <ID>",
	}
	for index, want := range wanted {
		if lines[index] != want {
			t.Errorf("line %d = %q, want %q", index+1, lines[index], want)
		}
	}
	if strings.Contains(result.stdout, "\x1b") {
		t.Error("colors in a pipe")
	}
}

// TestRulesWithConfig: the config file found from the current folder sets
// the severity; a changed one names its default.
func TestRulesWithConfig(t *testing.T) {
	root := projectWith(t, map[string]string{
		".concourse-lint.yml": "rules:\n  CS005: error\n  CL018: off\n",
		"ci/pipelines/x":      "",
	})
	result := runIn(t, filepath.Join(root, "ci", "pipelines"), "", "rules")
	if result.exitCode != 0 {
		t.Fatalf("exit code %d, stderr %q", result.exitCode, result.stderr)
	}
	for _, want := range []string{
		"\nCL002  error                     get/put or a group refers to a missing resource\n",
		"\nCL018  off (default: warning)    Unknown field that Concourse ignores\n",
		"\nCS005  error (default: warning)  Image without a fixed tag or digest\n",
		"\n\nSeverities from " + filepath.Join("..", "..", ".concourse-lint.yml") + ".\nDetails: concourse-lint explain <ID>\n",
	} {
		if !strings.Contains(result.stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, result.stdout)
		}
	}
}

// TestRulesConfigFlag: --config uses that file and names it as given.
func TestRulesConfigFlag(t *testing.T) {
	result := runCLI(t, "", "rules", "--config", testdata+"config/rule-off/.concourse-lint.yml")
	if result.exitCode != 0 {
		t.Fatalf("exit code %d, stderr %q", result.exitCode, result.stderr)
	}
	if !regexp.MustCompile(`\nCS008  off \(default: warning\) +Public build logs\n`).MatchString(result.stdout) {
		t.Errorf("CS008 is not off:\n%s", result.stdout)
	}
	if !strings.Contains(result.stdout, "Severities from "+testdata+"config/rule-off/.concourse-lint.yml.\n") {
		t.Errorf("the config file is not named as given:\n%s", result.stdout)
	}
}

// TestRulesJSONLikeMCP: the JSON is byte for byte the text block of the
// MCP tool list_rules in the same folder, also with a config file.
func TestRulesJSONLikeMCP(t *testing.T) {
	root := projectWith(t, map[string]string{".concourse-lint.yml": "rules:\n  CS005: error\n  CL018: off\n"})
	cli := runIn(t, root, "", "rules", "--format", "json")
	if cli.exitCode != 0 {
		t.Fatalf("exit code %d, stderr %q", cli.exitCode, cli.stderr)
	}
	answers := mcpSessionIn(t, root, map[string]any{"name": "list_rules"})
	text, _ := textAndStructured(t, answers[1])
	if cli.stdout != text {
		t.Errorf("CLI and MCP differ\nCLI:\n%s\nMCP:\n%s", cli.stdout, text)
	}
	var document struct {
		Rules []struct {
			ID, Severity    string
			DefaultSeverity string `json:"default_severity"`
		}
	}
	if err := json.Unmarshal([]byte(cli.stdout), &document); err != nil || len(document.Rules) != 37 {
		t.Fatalf("%d rules (%v), want 37", len(document.Rules), err)
	}
	if rule := document.Rules[len(document.Rules)-1]; rule.ID != "CS009" || rule.Severity != "info" || rule.DefaultSeverity != "info" {
		t.Errorf("last rule = %+v", rule)
	}
}

func TestRulesErrors(t *testing.T) {
	broken := projectWith(t, map[string]string{".concourse-lint.yml": "rules:\n  CL999: off\n"})
	cases := []struct {
		name   string
		folder string
		args   []string
		// wantStderr is a part of stderr.
		wantStderr string
	}{
		{"broken config", broken, []string{"rules"}, ".concourse-lint.yml:2:3: unknown rule \"CL999\""},
		{"broken config with JSON", broken, []string{"rules", "--format", "json"}, ".concourse-lint.yml:2:3:"},
		{"missing --config file", ".", []string{"rules", "--config", "does-not-exist.yml"}, "does-not-exist.yml"},
		{"an argument", ".", []string{"rules", "CL002"}, `"CL002"`},
		{"unknown format", ".", []string{"rules", "--format", "sarif"}, `"sarif"`},
		{"unknown flag", ".", []string{"rules", "--colour"}, "colour"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			result := runIn(t, testCase.folder, "", testCase.args...)
			if result.exitCode != 2 || result.stdout != "" || !strings.Contains(result.stderr, testCase.wantStderr) {
				t.Errorf("got %+v, want exit 2, no stdout, stderr containing %q", result, testCase.wantStderr)
			}
		})
	}
}

func TestRulesInHelp(t *testing.T) {
	result := runCLI(t, "", "help")
	if !strings.Contains(result.stdout, "concourse-lint rules") {
		t.Errorf("help does not mention rules:\n%s", result.stdout)
	}
}
