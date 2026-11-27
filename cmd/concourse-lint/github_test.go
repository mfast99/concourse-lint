package main

// Black-box tests for --format github (spec/README.md, "GitHub format").

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// getPutAnnotation is the first annotation of CL002/get-put.yml, without the
// file property.
const getPutAnnotation = `line=13,col=10,endLine=13,endColumn=13,title=concourse-lint CL002::resource "rep" is not defined (did you mean "repo"?)`

func TestGitHubAnnotations(t *testing.T) {
	testCases := []struct {
		name      string
		variables []string
		args      []string
		wantCode  int
		contains  []string
	}{
		{
			name:     "without GITHUB_WORKSPACE the path stays as given",
			args:     []string{"check", "--format", "github", testdata + "rules/correctness/CL002/get-put.yml"},
			wantCode: 1,
			contains: []string{"::error file=../../spec/rules/correctness/CL002/get-put.yml," + getPutAnnotation + "\n"},
		},
		{
			name:      "path relative to GITHUB_WORKSPACE",
			variables: []string{"GITHUB_WORKSPACE=" + repositoryRoot(t)},
			args:      []string{"check", "--format", "github", testdata + "rules/correctness/CL002/get-put.yml"},
			wantCode:  1,
			contains:  []string{"::error file=spec/rules/correctness/CL002/get-put.yml," + getPutAnnotation + "\n"},
		},
		{
			name:      "an input that could not be read is an annotation too",
			variables: []string{"GITHUB_WORKSPACE=" + repositoryRoot(t)},
			args:      []string{"check", "--format", "github", "does-not-exist.yml"},
			wantCode:  2,
			contains:  []string{"::error file=cmd/concourse-lint/does-not-exist.yml,title=concourse-lint::"},
		},
		{
			name:     "stdin has the position in the message",
			args:     []string{"check", "--format=github", "-"},
			wantCode: 1,
			contains: []string{"::error title=concourse-lint CL002::<stdin>:4:10: "},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stdin := "jobs:\n- name: a\n  plan:\n  - get: nope\nresources: []\n"
			result := runCLIWithEnvironment(t, testCase.variables, stdin, testCase.args...)
			if result.exitCode != testCase.wantCode {
				t.Errorf("exit code %d, want %d (stderr: %s)", result.exitCode, testCase.wantCode, result.stderr)
			}
			for _, expected := range testCase.contains {
				if !strings.Contains(result.stdout, expected) {
					t.Errorf("stdout does not contain %q:\n%s", expected, result.stdout)
				}
			}
		})
	}
}

func TestGitHubStepSummary(t *testing.T) {
	summaryPath := filepath.Join(t.TempDir(), "summary.md")
	if err := os.WriteFile(summaryPath, []byte("written before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	variables := []string{
		"GITHUB_WORKSPACE=" + repositoryRoot(t),
		"GITHUB_STEP_SUMMARY=" + summaryPath,
		"GITHUB_SERVER_URL=https://github.com",
		"GITHUB_REPOSITORY=org/repo",
		"GITHUB_SHA=abc123",
	}
	result := runCLIWithEnvironment(t, variables, "", "check", "--format", "github", testdata+"rules/correctness/CL002/get-put.yml")
	if result.exitCode != 1 {
		t.Errorf("exit code %d, want 1 (stderr: %s)", result.exitCode, result.stderr)
	}
	summary, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	wantParts := []string{
		// Appended, not overwritten.
		"written before\n### concourse-lint\n\n2 errors in 1 file.\n",
		"| [`spec/rules/correctness/CL002/get-put.yml:13:10`](https://github.com/org/repo/blob/abc123/spec/rules/correctness/CL002/get-put.yml#L13) | error | [CL002](",
	}
	for _, expected := range wantParts {
		if !strings.Contains(string(summary), expected) {
			t.Errorf("summary does not contain %q:\n%s", expected, summary)
		}
	}
}

// TestGitHubStepSummaryOnlyWithGitHubFormat: other formats never write a
// summary, even in a GitHub Actions run.
func TestGitHubStepSummaryOnlyWithGitHubFormat(t *testing.T) {
	summaryPath := filepath.Join(t.TempDir(), "summary.md")
	for _, format := range []string{"text", "json", "sarif"} {
		result := runCLIWithEnvironment(t, []string{"GITHUB_STEP_SUMMARY=" + summaryPath}, "", "check", "--format", format, testdata+"rules/correctness/CL002/get-put.yml")
		if result.exitCode != 1 {
			t.Errorf("--format %s: exit code %d, want 1 (stderr: %s)", format, result.exitCode, result.stderr)
		}
	}
	if _, err := os.Stat(summaryPath); !os.IsNotExist(err) {
		t.Errorf("summary was written (stat: %v)", err)
	}
}

// TestGitHubStepSummaryNotWritable: exit 2, a message on stderr, and the
// annotations are still on stdout.
func TestGitHubStepSummaryNotWritable(t *testing.T) {
	// A folder cannot be opened as a file.
	summaryPath := t.TempDir()
	result := runCLIWithEnvironment(t, []string{"GITHUB_STEP_SUMMARY=" + summaryPath}, "", "check", "--format", "github", testdata+"rules/correctness/CL002/get-put.yml")
	if result.exitCode != 2 {
		t.Errorf("exit code %d, want 2", result.exitCode)
	}
	if !strings.Contains(result.stderr, "summary") {
		t.Errorf("stderr does not mention the summary:\n%s", result.stderr)
	}
	if !strings.Contains(result.stdout, getPutAnnotation) {
		t.Errorf("annotations are missing on stdout:\n%s", result.stdout)
	}
}

func TestHelpListsGitHubFormat(t *testing.T) {
	result := runCLI(t, "", "help")
	if !strings.Contains(result.stdout, "text|json|sarif|github") {
		t.Errorf("help does not list the github format:\n%s", result.stdout)
	}
}
