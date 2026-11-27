package report

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// githubRun is a GitHub Actions run that starts in the repository root.
func githubRun(t *testing.T) GitHubEnvironment {
	t.Helper()
	workspace := t.TempDir()
	return GitHubEnvironment{
		Workspace:        workspace,
		WorkingDirectory: workspace,
		ServerURL:        "https://github.com",
		Repository:       "org/repo",
		Commit:           "abc123",
	}
}

func TestGitHubGolden(t *testing.T) {
	testCases := []struct {
		golden string
		result Result
	}{
		{golden: "github-empty", result: Result{Files: []File{{Path: "clean.yml"}}}},
		{golden: "github-mixed", result: mixedResult},
	}
	for _, testCase := range testCases {
		t.Run(testCase.golden, func(t *testing.T) {
			var output strings.Builder
			if err := GitHub(&output, testCase.result, githubRun(t)); err != nil {
				t.Fatal(err)
			}
			compareGolden(t, testCase.golden, []byte(output.String()))
		})
	}
}

func TestGitHub(t *testing.T) {
	multiLine := withEnd(finding("CL001", analysis.SeverityError, 3, 1, "wrong type", ""), 5, 2)
	escapedMessage := finding("CL001", analysis.SeverityError, 1, 1, "100% sure\r\nnext line", "")
	testCases := []struct {
		name string
		// As given on the command line, with / as separator.
		path string
		// The test joins path to the workspace, which makes it absolute.
		absolute bool
		// The working directory, relative to the workspace.
		subfolder string
		// GITHUB_WORKSPACE is not set.
		noWorkspace bool
		findings    []analysis.Finding
		// Messages of inputs at path that could not be read.
		errors []string
		want   string
	}{
		{
			name:     "a finding over several lines has no columns",
			path:     "pipeline.yml",
			findings: []analysis.Finding{multiLine},
			want:     "::error file=pipeline.yml,line=3,endLine=5,title=concourse-lint CL001::wrong type\n",
		},
		{
			name:     "message escaping",
			path:     "pipeline.yml",
			findings: []analysis.Finding{escapedMessage},
			want:     "::error file=pipeline.yml,line=1,col=1,endLine=1,endColumn=1,title=concourse-lint CL001::100%25 sure%0D%0Anext line\n",
		},
		{
			name:     "property escaping",
			path:     "a,b:c.yml",
			findings: []analysis.Finding{publicJob},
			want:     "::warning file=a%2Cb%3Ac.yml,line=9,col=3,endLine=9,endColumn=3,title=concourse-lint CS008::job 'build' has public build logs\n",
		},
		{
			name:     "absolute path inside the workspace",
			path:     "ci/pipeline.yml",
			absolute: true,
			findings: []analysis.Finding{publicJob},
			want:     "::warning file=ci/pipeline.yml,line=9,col=3,endLine=9,endColumn=3,title=concourse-lint CS008::job 'build' has public build logs\n",
		},
		{
			name:      "relative path from a subfolder",
			path:      "pipeline.yml",
			subfolder: "ci",
			findings:  []analysis.Finding{publicJob},
			want:      "::warning file=ci/pipeline.yml,line=9,col=3,endLine=9,endColumn=3,title=concourse-lint CS008::job 'build' has public build logs\n",
		},
		{
			name:     "relative path out of the workspace stays as given",
			path:     "../pipeline.yml",
			findings: []analysis.Finding{publicJob},
			want:     "::warning file=../pipeline.yml,line=9,col=3,endLine=9,endColumn=3,title=concourse-lint CS008::job 'build' has public build logs\n",
		},
		{
			name:        "without GITHUB_WORKSPACE the path stays as given",
			path:        "./ci/pipeline.yml",
			noWorkspace: true,
			findings:    []analysis.Finding{publicJob},
			want:        "::warning file=./ci/pipeline.yml,line=9,col=3,endLine=9,endColumn=3,title=concourse-lint CS008::job 'build' has public build logs\n",
		},
		{
			name:      "input error relative to the workspace",
			path:      "missing.yml",
			subfolder: "ci",
			errors:    []string{"open missing.yml: no such file or directory"},
			want:      "::error file=ci/missing.yml,title=concourse-lint::open missing.yml: no such file or directory\n",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			environment := githubRun(t)
			environment.WorkingDirectory = filepath.Join(environment.Workspace, testCase.subfolder)
			// The path as the user gives it; "/" is a separator on Windows
			// too, and a path that stays as given keeps it.
			path := testCase.path
			if testCase.absolute {
				path = filepath.Join(environment.Workspace, filepath.FromSlash(path))
			}
			if testCase.noWorkspace {
				environment.Workspace = ""
			}
			var result Result
			if testCase.findings != nil {
				result.Files = []File{{Path: path, Findings: testCase.findings}}
			}
			for _, message := range testCase.errors {
				result.Errors = append(result.Errors, InputError{Path: path, Message: message})
			}
			var output strings.Builder
			if err := GitHub(&output, result, environment); err != nil {
				t.Fatal(err)
			}
			if output.String() != testCase.want {
				t.Errorf("got:\n%s\nwant:\n%s", output.String(), testCase.want)
			}
		})
	}
}

// TestGitHubPathOutsideWorkspace keeps an absolute path outside the
// workspace as given.
func TestGitHubPathOutsideWorkspace(t *testing.T) {
	environment := githubRun(t)
	outsidePath := filepath.Join(t.TempDir(), "pipeline.yml")
	result := Result{Files: []File{{Path: outsidePath, Findings: []analysis.Finding{publicJob}}}}
	var output strings.Builder
	if err := GitHub(&output, result, environment); err != nil {
		t.Fatal(err)
	}
	want := "::warning file=" + escapeForTest(outsidePath) + ",line=9,"
	if !strings.HasPrefix(output.String(), want) {
		t.Errorf("got:\n%s\nwant prefix:\n%s", output.String(), want)
	}
}

// escapeForTest escapes a property value the way the spec says, so that the
// test also holds for Windows paths with a drive letter.
func escapeForTest(value string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(value)
}

func TestGitHubSummaryGolden(t *testing.T) {
	testCases := []struct {
		golden string
		result Result
	}{
		{golden: "github-summary-clean", result: Result{Files: []File{{Path: "clean.yml"}}}},
		{golden: "github-summary-mixed", result: mixedResult},
	}
	for _, testCase := range testCases {
		t.Run(testCase.golden, func(t *testing.T) {
			var output strings.Builder
			if err := GitHubSummary(&output, testCase.result, githubRun(t)); err != nil {
				t.Fatal(err)
			}
			compareGolden(t, testCase.golden, []byte(output.String()))
		})
	}
}

func TestGitHubSummaryCountLine(t *testing.T) {
	testCases := []struct {
		name     string
		files    []File
		wantLine string
	}{
		{name: "no files", files: nil, wantLine: "No findings in 0 files."},
		{name: "one file", files: []File{{Path: "a.yml"}}, wantLine: "No findings in 1 file."},
		{
			name:     "plural errors, one file",
			files:    []File{{Path: "a.yml", Findings: []analysis.Finding{unknownResource, unknownResource}}},
			wantLine: "2 errors in 1 file.",
		},
		{
			name:     "warnings only",
			files:    []File{{Path: "a.yml", Findings: []analysis.Finding{publicJob}}, {Path: "b.yml", Findings: []analysis.Finding{publicJob, publicJob}}},
			wantLine: "3 warnings in 2 files.",
		},
		{
			name:     "info has no plural",
			files:    []File{{Path: "a.yml", Findings: []analysis.Finding{revealedVar, revealedVar}}},
			wantLine: "2 info in 1 file.",
		},
		{
			name:     "order error, warning, info",
			files:    []File{{Path: "a.yml", Findings: []analysis.Finding{revealedVar, publicJob, unknownResource}}},
			wantLine: "1 error, 1 warning, 1 info in 1 file.",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var output strings.Builder
			if err := GitHubSummary(&output, Result{Files: testCase.files}, githubRun(t)); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(output.String(), "\n")
			if len(lines) < 3 || lines[2] != testCase.wantLine {
				t.Errorf("count line is not %q:\n%s", testCase.wantLine, output.String())
			}
		})
	}
}

func TestGitHubSummaryRows(t *testing.T) {
	escapeMe := finding("CL001", analysis.SeverityError, 2, 4, "a|b *c* _d_ <x> & \\ [e] `f`\nnext", "")
	testCases := []struct {
		name string
		path string
		// Change the environment of githubRun.
		adjust  func(environment *GitHubEnvironment)
		finding analysis.Finding
		wantRow string
	}{
		{
			name:    "markdown in the message is escaped",
			path:    "pipeline.yml",
			finding: escapeMe,
			wantRow: "| [`pipeline.yml:2:4`](https://github.com/org/repo/blob/abc123/pipeline.yml#L2) | error | [CL001](" + analysis.DocsURL("CL001") + ") | a\\|b \\*c\\* \\_d\\_ &lt;x&gt; &amp; \\\\ \\[e\\] \\`f\\` next |",
		},
		{
			name:    "path segments are URL-escaped in the link",
			path:    "ci/my pipeline#1.yml",
			finding: publicJob,
			wantRow: "| [`ci/my pipeline#1.yml:9:3`](https://github.com/org/repo/blob/abc123/ci/my%20pipeline%231.yml#L9) | warning | [CS008](" + analysis.DocsURL("CS008") + ") | job 'build' has public build logs |",
		},
		{
			name:    "no link without the repository variables",
			path:    "pipeline.yml",
			adjust:  func(environment *GitHubEnvironment) { environment.Commit = "" },
			finding: publicJob,
			wantRow: "| `pipeline.yml:9:3` | warning | [CS008](" + analysis.DocsURL("CS008") + ") | job 'build' has public build logs |",
		},
		{
			name:    "no link for a path outside the workspace",
			path:    "../pipeline.yml",
			finding: publicJob,
			wantRow: "| `../pipeline.yml:9:3` | warning | [CS008](" + analysis.DocsURL("CS008") + ") | job 'build' has public build logs |",
		},
		{
			name:    "no link without GITHUB_WORKSPACE",
			path:    "pipeline.yml",
			adjust:  func(environment *GitHubEnvironment) { environment.Workspace = "" },
			finding: publicJob,
			wantRow: "| `pipeline.yml:9:3` | warning | [CS008](" + analysis.DocsURL("CS008") + ") | job 'build' has public build logs |",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			environment := githubRun(t)
			if testCase.adjust != nil {
				testCase.adjust(&environment)
			}
			result := Result{Files: []File{{Path: testCase.path, Findings: []analysis.Finding{testCase.finding}}}}
			var output strings.Builder
			if err := GitHubSummary(&output, result, environment); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "\n"+testCase.wantRow+"\n") {
				t.Errorf("no row\n%s\nin:\n%s", testCase.wantRow, output.String())
			}
		})
	}
}

// TestGitHubSummaryLimit keeps the summary below GitHub's 1 MiB per step:
// at most 500 rows, then one line with the rest.
func TestGitHubSummaryLimit(t *testing.T) {
	testCases := []struct {
		findingCount int
		wantRows     int
		wantMore     string
	}{
		{findingCount: 500, wantRows: 500, wantMore: ""},
		{findingCount: 501, wantRows: 500, wantMore: "… and 1 more finding (use --format json to see all)."},
		{findingCount: 502, wantRows: 500, wantMore: "… and 2 more findings (use --format json to see all)."},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprint(testCase.findingCount), func(t *testing.T) {
			findings := make([]analysis.Finding, testCase.findingCount)
			for index := range findings {
				findings[index] = publicJob
			}
			result := Result{Files: []File{{Path: "pipeline.yml", Findings: findings}}}
			var output strings.Builder
			if err := GitHubSummary(&output, result, githubRun(t)); err != nil {
				t.Fatal(err)
			}
			rows := strings.Count(output.String(), "\n| [`")
			if rows != testCase.wantRows {
				t.Errorf("%d rows, want %d", rows, testCase.wantRows)
			}
			hasMore := strings.Contains(output.String(), "more finding")
			if testCase.wantMore == "" && hasMore {
				t.Errorf("unexpected \"more findings\" line:\n%s", output.String())
			}
			if testCase.wantMore != "" && !strings.HasSuffix(output.String(), "|\n\n"+testCase.wantMore+"\n") {
				t.Errorf("summary does not end with the table and %q", testCase.wantMore)
			}
		})
	}
}

// TestGitHubSummaryPipeInPath: a "|" in a path would end the table cell,
// also inside code.
func TestGitHubSummaryPipeInPath(t *testing.T) {
	environment := githubRun(t)
	environment.Commit = ""
	result := Result{Files: []File{{Path: "a|b.yml", Findings: []analysis.Finding{publicJob}}}}
	var output strings.Builder
	if err := GitHubSummary(&output, result, environment); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "\n| `a\\|b.yml:9:3` | warning |") {
		t.Errorf("pipe in the path is not escaped:\n%s", output.String())
	}
}
