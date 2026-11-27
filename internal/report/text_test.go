package report

import (
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// finding builds a finding for the tests in this package.
func finding(rule string, severity analysis.Severity, line, column int, message, hint string) analysis.Finding {
	return analysis.Finding{
		Rule:     rule,
		Severity: severity,
		Position: model.Position{Line: line, Column: column},
		Message:  message,
		Hint:     hint,
	}
}

// withSuggestion gives a finding a suggestion (the text for its range).
func withSuggestion(finding analysis.Finding, suggestion string) analysis.Finding {
	finding.Suggestion = suggestion
	return finding
}

// withEnd gives a finding an end; findings without one have length zero.
func withEnd(finding analysis.Finding, endLine, endColumn int) analysis.Finding {
	finding.Position.EndLine, finding.Position.EndColumn = endLine, endColumn
	return finding
}

var (
	unknownResource = withSuggestion(withEnd(finding("CL002", analysis.SeverityError, 13, 10, "resource 'rep' is not defined", "did you mean 'repo'?"), 13, 13), "repo")
	publicJob       = finding("CS008", analysis.SeverityWarning, 9, 3, "job 'build' has public build logs", "")
	revealedVar     = finding("CS009", analysis.SeverityInfo, 13, 5, "load_var 'version' reveals the value in the log", "")
)

func TestText(t *testing.T) {
	testCases := []struct {
		name   string
		result Result
		want   string
	}{
		{
			name:   "no files",
			result: Result{},
			want:   "",
		},
		{
			name:   "file without findings",
			result: Result{Files: []File{{Path: "clean.yml"}}},
			want:   "",
		},
		{
			name:   "hint in parentheses",
			result: Result{Files: []File{{Path: "pipeline.yml", Findings: []analysis.Finding{unknownResource}}}},
			want:   "pipeline.yml:13:10: error CL002 resource 'rep' is not defined (did you mean 'repo'?)\n",
		},
		{
			name:   "no parentheses without hint",
			result: Result{Files: []File{{Path: "pipeline.yml", Findings: []analysis.Finding{publicJob}}}},
			want:   "pipeline.yml:9:3: warning CS008 job 'build' has public build logs\n",
		},
		{
			name:   "info severity",
			result: Result{Files: []File{{Path: "pipeline.yml", Findings: []analysis.Finding{revealedVar}}}},
			want:   "pipeline.yml:13:5: info CS009 load_var 'version' reveals the value in the log\n",
		},
		{
			name: "files and findings keep their order",
			result: Result{Files: []File{
				{Path: "b.yml", Findings: []analysis.Finding{publicJob, unknownResource}},
				{Path: "clean.yml"},
				{Path: "a.yml", Findings: []analysis.Finding{revealedVar}},
			}},
			want: "b.yml:9:3: warning CS008 job 'build' has public build logs\n" +
				"b.yml:13:10: error CL002 resource 'rep' is not defined (did you mean 'repo'?)\n" +
				"a.yml:13:5: info CS009 load_var 'version' reveals the value in the log\n",
		},
		{
			name:   "path as given, stdin by its name",
			result: Result{Files: []File{{Path: StdinPath, Findings: []analysis.Finding{publicJob}}}},
			want:   "<stdin>:9:3: warning CS008 job 'build' has public build logs\n",
		},
		{
			name: "read errors are not part of the text output",
			result: Result{
				Files:  []File{{Path: "pipeline.yml", Findings: []analysis.Finding{publicJob}}},
				Errors: []InputError{{Path: "missing.yml", Message: "no such file or directory"}},
			},
			want: "pipeline.yml:9:3: warning CS008 job 'build' has public build logs\n",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var output strings.Builder
			if err := Text(&output, testCase.result); err != nil {
				t.Fatal(err)
			}
			if output.String() != testCase.want {
				t.Errorf("got:\n%s\nwant:\n%s", output.String(), testCase.want)
			}
		})
	}
}
