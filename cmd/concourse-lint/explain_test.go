package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExplainText: the rule page for the terminal, without colors in a
// pipe; the ID in any case (spec/README.md, "Rules and explain").
func TestExplainText(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "explain_CL002.golden"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"CL002", "cl002"} {
		result := runIn(t, projectWith(t, map[string]string{}), "", "explain", id)
		if result.exitCode != 0 || result.stderr != "" {
			t.Fatalf("%s: exit code %d, stderr %q", id, result.exitCode, result.stderr)
		}
		if result.stdout != string(want) {
			t.Errorf("%s: output differs from testdata/explain_CL002.golden\ngot:\n%s\nwant:\n%s", id, result.stdout, want)
		}
	}
}

// TestExplainEveryRule: every rule has its title, the four sections and
// the link.
func TestExplainEveryRule(t *testing.T) {
	folder := projectWith(t, map[string]string{})
	for _, id := range catalogIDs(t) {
		result := runIn(t, folder, "", "explain", id)
		if result.exitCode != 0 {
			t.Errorf("%s: exit code %d, stderr %q", id, result.exitCode, result.stderr)
			continue
		}
		for _, want := range []string{id + ": ", "\nSeverity: ", "\nWhat is checked?\n", "\nExample\n", "\nFix\n", "\nSuppress\n", "/docs/rules/" + id + ".md\n"} {
			if !strings.Contains(result.stdout, want) {
				t.Errorf("%s: no %q in\n%s", id, want, result.stdout)
			}
		}
	}
}

func TestExplainWithConfig(t *testing.T) {
	root := projectWith(t, map[string]string{".concourse-lint.yml": "rules:\n  CS005: error\n  CL018: off\n"})
	cases := map[string]string{
		"CS005": "\nSeverity: error (default: warning)\n",
		"CL018": "\nSeverity: off (default: warning)\n",
		"CL002": "\nSeverity: error\n",
	}
	for id, want := range cases {
		result := runIn(t, root, "", "explain", id)
		if !strings.Contains(result.stdout, want) {
			t.Errorf("%s: no %q in\n%s", id, want, result.stdout)
		}
	}
	explicit := runCLI(t, "", "explain", "--config", testdata+"config/rule-off/.concourse-lint.yml", "CS008")
	if !strings.Contains(explicit.stdout, "\nSeverity: off (default: warning)\n") {
		t.Errorf("--config: CS008 is not off:\n%s", explicit.stdout)
	}
}

// TestExplainJSONLikeMCP: the JSON is byte for byte the text block of the
// MCP tool explain_rule in the same folder; flags may follow the ID.
func TestExplainJSONLikeMCP(t *testing.T) {
	root := projectWith(t, map[string]string{".concourse-lint.yml": "rules:\n  CS005: error\n"})
	cli := runIn(t, root, "", "explain", "cs005", "--format", "json")
	if cli.exitCode != 0 {
		t.Fatalf("exit code %d, stderr %q", cli.exitCode, cli.stderr)
	}
	answers := mcpSessionIn(t, root, map[string]any{"name": "explain_rule", "arguments": map[string]any{"id": "cs005"}})
	text, _ := textAndStructured(t, answers[1])
	if cli.stdout != text {
		t.Errorf("CLI and MCP differ\nCLI:\n%s\nMCP:\n%s", cli.stdout, text)
	}
	for _, want := range []string{`"severity": "error"`, `"default_severity": "warning"`, `"example": "`, `"suppress": "`} {
		if !strings.Contains(cli.stdout, want) {
			t.Errorf("no %s in\n%s", want, cli.stdout)
		}
	}
}

func TestExplainErrors(t *testing.T) {
	broken := projectWith(t, map[string]string{".concourse-lint.yml": "rules:\n  CL999: off\n"})
	cases := []struct {
		name   string
		folder string
		args   []string
		// wantStderr are parts of stderr.
		wantStderr []string
	}{
		{"unknown ID", ".", []string{"explain", "CL02"}, []string{`unknown rule "CL02" (did you mean "CL002"?)`, "concourse-lint rules"}},
		{"unknown ID without a close one", ".", []string{"explain", "nothing"}, []string{`unknown rule "nothing"`, "concourse-lint rules"}},
		{"no ID", ".", []string{"explain"}, []string{"rule ID", "Usage"}},
		{"two IDs", ".", []string{"explain", "CL002", "CL003"}, []string{`"CL003"`}},
		{"unknown format", ".", []string{"explain", "--format", "sarif", "CL002"}, []string{`"sarif"`}},
		{"broken config", broken, []string{"explain", "CL002"}, []string{".concourse-lint.yml:2:3: unknown rule \"CL999\""}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			result := runIn(t, testCase.folder, "", testCase.args...)
			if result.exitCode != 2 || result.stdout != "" {
				t.Errorf("exit code %d, stdout %q; want 2 and nothing", result.exitCode, result.stdout)
			}
			for _, want := range testCase.wantStderr {
				if !strings.Contains(result.stderr, want) {
					t.Errorf("stderr does not contain %q:\n%s", want, result.stderr)
				}
			}
		})
	}
}

func TestExplainInHelp(t *testing.T) {
	result := runCLI(t, "", "help")
	if !strings.Contains(result.stdout, "concourse-lint explain") {
		t.Errorf("help does not mention explain:\n%s", result.stdout)
	}
}

// catalogIDs reads the rule IDs from spec/catalog.txt.
func catalogIDs(t *testing.T) []string {
	t.Helper()
	content, err := os.ReadFile(testdata + "catalog.txt")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && !strings.HasPrefix(fields[0], "#") {
			ids = append(ids, fields[0])
		}
	}
	return ids
}
