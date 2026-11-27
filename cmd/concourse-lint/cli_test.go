package main

// Black-box tests: the real binary is built and invoked the way a human,
// a CI system or an AI agent would. Only exit code, stdout and stderr are
// checked, nothing internal.

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var binary string

// executableName adds ".exe" on Windows, which runs only files with that
// extension.
func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func TestMain(suite *testing.M) {
	// Paths that the CLI found in a folder use the separator of the system
	// in its output, so the expectations below use filepath.FromSlash.
	tempDir, err := os.MkdirTemp("", "concourse-lint-test")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(tempDir, executableName("concourse-lint"))
	if buildOutput, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		panic("build failed: " + string(buildOutput))
	}
	exitCode := suite.Run()
	os.RemoveAll(tempDir)
	os.Exit(exitCode)
}

const testdata = "../../spec/"

type cliResult struct {
	exitCode       int
	stdout, stderr string
}

func runCLI(t *testing.T, stdin string, args ...string) cliResult {
	t.Helper()
	return runCLIWithEnvironment(t, nil, stdin, args...)
}

// githubVariables are the variables of a GitHub Actions run that the CLI
// reads. They are removed from the environment of every test run, so that
// the tests behave the same in our own CI (and never write to its step
// summary); a test sets them on purpose with runCLIWithEnvironment.
var githubVariables = []string{"GITHUB_WORKSPACE", "GITHUB_STEP_SUMMARY", "GITHUB_SERVER_URL", "GITHUB_REPOSITORY", "GITHUB_SHA"}

// runCLIWithEnvironment is runCLI with extra variables ("NAME=value").
func runCLIWithEnvironment(t *testing.T, variables []string, stdin string, args ...string) cliResult {
	t.Helper()
	command := exec.Command(binary, args...)
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if !slices.Contains(githubVariables, name) {
			command.Env = append(command.Env, variable)
		}
	}
	command.Env = append(command.Env, variables...)
	command.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	exitCode := 0
	if exitError, exited := err.(*exec.ExitError); exited {
		exitCode = exitError.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return cliResult{exitCode, stdout.String(), stderr.String()}
}

func TestCLI(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		stdin    string
		wantCode int
		// Must appear in stdout.
		contains []string
		// Stdout must be empty.
		empty bool
		// Must appear in stderr.
		stderrContains []string
	}{
		{name: "valid pipeline", args: []string{"check", testdata + "valid/01-minimal.yml"}, wantCode: 0, empty: true},
		{name: "error gives exit 1", args: []string{"check", testdata + "rules/correctness/CL002/get-put.yml"}, wantCode: 1, contains: []string{"CL002", "CL002/get-put.yml:13:"}},
		{name: "warning only gives exit 0", args: []string{"check", testdata + "rules/security/CS008/basic.yml"}, wantCode: 0, contains: []string{"CS008"}},
		{name: "fail-on warning", args: []string{"check", "--fail-on", "warning", testdata + "rules/security/CS008/basic.yml"}, wantCode: 1, contains: []string{"CS008"}},
		{name: "info gives exit 0 even with fail-on warning", args: []string{"check", "--fail-on", "warning", testdata + "rules/security/CS009/basic.yml"}, wantCode: 0, contains: []string{"CS009"}},
		{name: "stdin", args: []string{"check", "-"}, stdin: "jobs:\n- name: a\n  plan:\n  - get: nope\nresources: []\n", wantCode: 1, contains: []string{"CL002"}},
		{name: "missing file", args: []string{"check", "does-not-exist.yml"}, wantCode: 2},
		{name: "unknown command", args: []string{"lint-all-the-things"}, wantCode: 2},
		{name: "syntax error", args: []string{"check", testdata + "syntax/unclosed-flow.yml"}, wantCode: 1, contains: []string{"CL000"}},
		{name: "SARIF for GitHub Code Scanning", args: []string{"check", "--format", "sarif", testdata + "rules/security/CS002/github.yml"}, wantCode: 1, contains: []string{`"version": "2.1.0"`, "CS002"}},

		// M3 cases that need only CL000 from the parser, no rules.
		{name: "stdin with syntax error", args: []string{"check", "-"}, stdin: "jobs: [\n", wantCode: 1, contains: []string{"<stdin>:", "CL000"}},
		{name: "fail-on info with syntax error", args: []string{"check", "--fail-on", "info", testdata + "syntax/unclosed-flow.yml"}, wantCode: 1, contains: []string{"CL000"}},
		{name: "JSON of a valid pipeline", args: []string{"check", "--format", "json", testdata + "valid/01-minimal.yml"}, wantCode: 0, contains: []string{`"findings": []`, `"errors": []`}},
		{name: "flag with equals sign", args: []string{"check", "--format=json", testdata + "valid/01-minimal.yml"}, wantCode: 0, contains: []string{`"findings": []`}},
		{name: "SARIF with syntax error", args: []string{"check", "--format", "sarif", testdata + "syntax/unclosed-flow.yml"}, wantCode: 1, contains: []string{`"version": "2.1.0"`, `"ruleId": "CL000"`, `"level": "error"`}},
		{name: "several files keep their order", args: []string{"check", testdata + "syntax/unclosed-flow.yml", testdata + "valid/01-minimal.yml", "-"}, stdin: "jobs: [\n", wantCode: 1, contains: []string{"syntax/unclosed-flow.yml:", "<stdin>:"}},
		{name: "missing file next to a readable one", args: []string{"check", "does-not-exist.yml", testdata + "syntax/unclosed-flow.yml"}, wantCode: 2, contains: []string{"CL000"}, stderrContains: []string{"does-not-exist.yml"}},
		{name: "directory is checked recursively", args: []string{"check", testdata + "rules/security/CS008"}, wantCode: 1, contains: []string{filepath.FromSlash("CS008/basic.yml:"), filepath.FromSlash("CS008/yaml11.yml:")}},

		// Usage errors: exit 2, nothing on stdout, a message on stderr.
		{name: "no arguments", args: []string{}, wantCode: 2, empty: true, stderrContains: []string{"Usage"}},
		{name: "check without a file", args: []string{"check"}, wantCode: 2, empty: true, stderrContains: []string{"no file"}},
		{name: "unknown format", args: []string{"check", "--format", "xml", testdata + "valid/01-minimal.yml"}, wantCode: 2, empty: true, stderrContains: []string{`"xml"`}},
		{name: "unknown fail-on", args: []string{"check", "--fail-on", "fatal", testdata + "valid/01-minimal.yml"}, wantCode: 2, empty: true, stderrContains: []string{`"fatal"`}},
		{name: "unknown flag", args: []string{"check", "--colour", testdata + "valid/01-minimal.yml"}, wantCode: 2, empty: true, stderrContains: []string{"colour"}},
		{name: "flag after a file", args: []string{"check", testdata + "valid/01-minimal.yml", "--format", "json"}, wantCode: 0, contains: []string{`"findings": []`}},
		{name: "flags between files", args: []string{"check", testdata + "valid/01-minimal.yml", "--format", "json", testdata + "valid/02-full-featured.yml"}, wantCode: 0, contains: []string{`"findings": []`}},
		{name: "-- ends the flags", args: []string{"check", "--format", "json", "--", testdata + "valid/01-minimal.yml"}, wantCode: 0, contains: []string{`"findings": []`}},
		{name: "after -- a flag is a file name", args: []string{"check", "--", testdata + "valid/01-minimal.yml", "--format"}, wantCode: 2, stderrContains: []string{"--format"}},
		{name: "stdin twice", args: []string{"check", "-", "-"}, wantCode: 2, empty: true, stderrContains: []string{"once"}},
		{name: "unknown command shows usage", args: []string{"lint"}, wantCode: 2, empty: true, stderrContains: []string{`"lint"`, "Usage"}},

		// Version and help.
		{name: "version short flag", args: []string{"-v"}, wantCode: 0, contains: []string{"concourse-lint "}},
		{name: "version long flag", args: []string{"--version"}, wantCode: 0, contains: []string{"concourse-lint "}},
		{name: "version command", args: []string{"version"}, wantCode: 0, contains: []string{"concourse-lint "}},
		{name: "help", args: []string{"help"}, wantCode: 0, contains: []string{"Usage", "check", "--format", "--fail-on"}},
		{name: "help flag", args: []string{"--help"}, wantCode: 0, contains: []string{"Usage"}},

		// Config file (spec/README.md, "Config file").
		{name: "config is found next to the pipeline", args: []string{"check", testdata + "config/severity/pipeline.yml"}, wantCode: 1, contains: []string{"error CS005", "info CB003"}},
		{name: "--config applies to every input", args: []string{"check", "--config", testdata + "config/rule-off/.concourse-lint.yml", testdata + "rules/security/CS008/basic.yml"}, wantCode: 0, empty: true},
		{name: "broken config checks nothing", args: []string{"check", "--config", testdata + "config/unknown-rule/.concourse-lint.yml", testdata + "rules/correctness/CL002/get-put.yml"}, wantCode: 2, empty: true, stderrContains: []string{"unknown-rule/.concourse-lint.yml:3:3: unknown rule \"CS060\""}},
		{name: "broken config found by the search", args: []string{"check", testdata + "config/unknown-key/.concourse-lint.yml"}, wantCode: 2, empty: true, stderrContains: []string{filepath.FromSlash("unknown-key/.concourse-lint.yml") + ":2:1: unknown field \"rulez\""}},
		{name: "missing --config file", args: []string{"check", "--config", "does-not-exist.yml", testdata + "valid/01-minimal.yml"}, wantCode: 2, empty: true, stderrContains: []string{"does-not-exist.yml"}},
		{name: "excluded folder is skipped", args: []string{"check", testdata + "config/exclude"}, wantCode: 1, contains: []string{filepath.FromSlash("exclude/pipeline.yml:")}},
		{name: "excluded file is skipped when named", args: []string{"check", testdata + "config/exclude/templates/child.yml"}, wantCode: 0, empty: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := runCLI(t, testCase.stdin, testCase.args...)
			if result.exitCode != testCase.wantCode {
				t.Errorf("exit code %d, want %d (stderr: %s)", result.exitCode, testCase.wantCode, result.stderr)
			}
			for _, expected := range testCase.contains {
				if !strings.Contains(result.stdout, expected) {
					t.Errorf("stdout does not contain %q:\n%s", expected, result.stdout)
				}
			}
			if testCase.empty && result.stdout != "" {
				t.Errorf("stdout should be empty:\n%s", result.stdout)
			}
			for _, expected := range testCase.stderrContains {
				if !strings.Contains(result.stderr, expected) {
					t.Errorf("stderr does not contain %q:\n%s", expected, result.stderr)
				}
			}
		})
	}
}

// TestVersionFromLdflags guards the release build: the version is set with
// -ldflags "-X main.version=...", which silently does nothing if the
// variable is renamed.
func TestVersionFromLdflags(t *testing.T) {
	releaseBinary := filepath.Join(t.TempDir(), executableName("concourse-lint"))
	build := exec.Command("go", "build", "-ldflags", "-X main.version=v9.8.7", "-o", releaseBinary, ".")
	if buildOutput, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, buildOutput)
	}
	output, err := exec.Command(releaseBinary, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "concourse-lint v9.8.7\n" {
		t.Errorf("got %q, want %q", output, "concourse-lint v9.8.7\n")
	}
}

// TestJSONContractInputErrors: a file that cannot be read is listed under
// "errors", the other files are still checked, and every field is present.
// Needs only CL000, so it is green before the rules exist (M2).
func TestJSONContractInputErrors(t *testing.T) {
	result := runCLI(t, "", "check", "--format", "json", "does-not-exist.yml", testdata+"syntax/unclosed-flow.yml")
	if result.exitCode != 2 {
		t.Fatalf("exit code %d, want 2", result.exitCode)
	}
	var output struct {
		Findings []map[string]any `json:"findings"`
		Errors   []struct {
			File    string `json:"file"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &output); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, result.stdout)
	}
	if len(output.Findings) != 1 {
		t.Fatalf("%d findings, want 1 (CL000 of the readable file)", len(output.Findings))
	}
	finding := output.Findings[0]
	for _, key := range []string{"file", "rule", "severity", "pos", "message", "hint", "docs"} {
		if _, present := finding[key]; !present {
			t.Errorf("finding has no %q: %v", key, finding)
		}
	}
	if finding["rule"] != "CL000" || !strings.HasSuffix(finding["file"].(string), "syntax/unclosed-flow.yml") {
		t.Errorf("finding wrong: %v", finding)
	}
	if len(output.Errors) != 1 || output.Errors[0].File != "does-not-exist.yml" || !strings.Contains(output.Errors[0].Message, "does-not-exist.yml") {
		t.Errorf("errors wrong: %+v", output.Errors)
	}
}

// TestJSONContract: the JSON output is the interface for AI agents.
// Its shape may only change deliberately.
func TestJSONContract(t *testing.T) {
	result := runCLI(t, "", "check", "--format", "json", testdata+"rules/correctness/CL002/get-put.yml")
	if result.exitCode != 1 {
		t.Fatalf("exit code %d, want 1", result.exitCode)
	}
	var output struct {
		Findings []struct {
			File     string `json:"file"`
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
			Position struct {
				Line   int `json:"line"`
				Column int `json:"col"`
			} `json:"pos"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
			Docs    string `json:"docs"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &output); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, result.stdout)
	}
	if len(output.Findings) != 2 {
		t.Fatalf("%d findings, want 2", len(output.Findings))
	}
	first := output.Findings[0]
	if first.Docs != "https://github.com/mfast99/concourse-lint/blob/main/docs/rules/CL002.md" {
		t.Errorf("docs link %q, want the rule page of CL002", first.Docs)
	}
	if first.Rule != "CL002" || first.Severity != "error" || first.Position.Line != 13 || first.Position.Column != 10 {
		t.Errorf("first finding wrong: %+v", first)
	}
	if !strings.Contains(first.Hint, "repo") {
		t.Errorf("hint should suggest 'repo': %q", first.Hint)
	}
}

// TestDirectory: a folder is checked recursively. YAML files that are no
// pipeline (no top-level jobs: with a plan:, e.g. task files, var files, BOSH
// release manifests), ytt and spruce templates and dot folders such as
// .github are skipped; a file given by name is always checked.
func TestDirectory(t *testing.T) {
	folder := t.TempDir()
	files := map[string]string{
		"pipeline.yml":             "jobs:\n- name: a\n  plan:\n  - get: nope\nresources: []\n",
		"nested/deeper/other.yaml": "resources: []\njobs:\n- name: b\n  plan:\n  - get: missing\n",
		"vars.yml":                 "region: eu-central-1\n",
		"tasks/unit.yml":           "platform: linux\nrun: {path: make}\n",
		".github/workflows/ci.yml": "on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n",
		"README.md":                "jobs:\n",
		"releases/garden-1.0.yml":  "name: garden\njobs:\n- name: garden\n  version: abc\npackages: []\n",
		"vendor/lib/.travis.yml":   "language: go\njobs:\n  include:\n  - go: 1.24\n",
		"templates/ytt.yml":        "#@ load(\"@ytt:data\", \"data\")\njobs:\n- name: c\n  plan:\n  - get: nope\n",
		"templates/spruce.yml":     "jobs:\n- name: d\n  plan:\n  - get: (( grab meta.repo ))\n",
	}
	for name, content := range files {
		path := filepath.Join(folder, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result := runCLI(t, "", "check", folder)
	if result.exitCode != 1 {
		t.Fatalf("exit code %d, want 1 (stderr: %s)", result.exitCode, result.stderr)
	}
	for _, expected := range []string{"pipeline.yml:4:", filepath.FromSlash("nested/deeper/other.yaml:5:")} {
		if !strings.Contains(result.stdout, expected) {
			t.Errorf("stdout does not contain %q:\n%s", expected, result.stdout)
		}
	}
	for _, unexpected := range []string{"vars.yml", "unit.yml", "ci.yml", "README", "garden-1.0.yml", "travis", "templates"} {
		if strings.Contains(result.stdout, unexpected) {
			t.Errorf("stdout contains %q, which is no pipeline:\n%s", unexpected, result.stdout)
		}
	}
	explicit := runCLI(t, "", "check", filepath.Join(folder, "vars.yml"))
	if !strings.Contains(explicit.stdout, "CL001") {
		t.Errorf("a file given by name is always checked, got:\n%s", explicit.stdout)
	}
	template := runCLI(t, "", "check", filepath.Join(folder, "templates", "spruce.yml"))
	if template.exitCode != 0 || !strings.Contains(template.stdout, "spruce.yml:4:10: info CL022") {
		t.Errorf("a template given by name gets one CL022 info, got exit %d:\n%s", template.exitCode, template.stdout)
	}
}
