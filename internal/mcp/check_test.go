package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

const (
	// brokenPipeline has one CL002 (unknown resource) at 4:10.
	brokenPipeline = "jobs:\n- name: build\n  plan:\n  - get: missing\nresources: []\n"
)

// checkDocument is the JSON document of the CLI contract.
type checkDocument struct {
	Findings []struct {
		File     string
		Rule     string
		Severity string
		Pos      struct{ Line, Col int }
		Message  string
	}
	Errors []struct{ File, Message string }
}

// callCheckPipeline calls check_pipeline once and returns the tool result.
func callCheckPipeline(t *testing.T, arguments any) toolResult {
	t.Helper()
	answers := session(t, latestProtocolVersion, callTool(1, "check_pipeline", arguments))
	return readToolResult(t, answers[0], latestProtocolVersion)
}

// checkDocumentOf calls check_pipeline and expects a document, no tool error.
func checkDocumentOf(t *testing.T, arguments any) checkDocument {
	t.Helper()
	result := callCheckPipeline(t, arguments)
	if result.isError {
		t.Fatalf("tool error: %s", result.text)
	}
	var document checkDocument
	if err := json.Unmarshal([]byte(result.text), &document); err != nil {
		t.Fatal(err)
	}
	return document
}

// toFindings is the way back from the document to findings, for spectest.
func (document checkDocument) toFindings() []analysis.Finding {
	var findings []analysis.Finding
	for _, finding := range document.Findings {
		findings = append(findings, analysis.Finding{
			Rule: finding.Rule, Severity: analysis.Severity(finding.Severity),
			Position: model.Position{Line: finding.Pos.Line, Column: finding.Pos.Col}, Message: finding.Message,
		})
	}
	return findings
}

// inTempProject changes into a new folder that is a Git root (so the
// config search ends there) with the given files.
func inTempProject(t *testing.T, files map[string]string) string {
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
	t.Chdir(root)
	return root
}

// TestSpec sends every file of spec/ as text through the protocol and
// compares the findings with the "# want" lines: an agent gets what the CLI
// reports.
func TestSpec(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	repositoryRoot := filepath.Dir(spectest.Dir())
	t.Chdir(repositoryRoot)
	var lines []string
	files := spectest.Files(t)
	for index, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		relative, _ := filepath.Rel(repositoryRoot, file)
		lines = append(lines, callTool(index+1, "check_pipeline", map[string]any{"text": string(source), "filename": filepath.ToSlash(relative)}))
	}
	answers := session(t, latestProtocolVersion, lines...)
	for index, file := range files {
		result := readToolResult(t, answers[index], latestProtocolVersion)
		var document checkDocument
		if err := json.Unmarshal([]byte(result.text), &document); err != nil || result.isError {
			t.Errorf("%s: no document: %s", spectest.Name(file), result.text)
			continue
		}
		source, _ := os.ReadFile(file)
		for _, problem := range spectest.Compare(spectest.ParseExpectations(source), catalog, document.toFindings()) {
			t.Errorf("%s: %s", spectest.Name(file), problem)
		}
	}
}

func TestCheckTextNames(t *testing.T) {
	inTempProject(t, map[string]string{})
	for _, testCase := range []struct {
		arguments map[string]any
		wantFile  string
	}{
		{map[string]any{"text": brokenPipeline}, "<text>"},
		{map[string]any{"text": brokenPipeline, "filename": "ci/pipeline.yml"}, "ci/pipeline.yml"},
	} {
		document := checkDocumentOf(t, testCase.arguments)
		if len(document.Findings) != 1 || document.Findings[0].File != testCase.wantFile || document.Findings[0].Rule != "CL002" {
			t.Errorf("%v: findings %+v, want one CL002 in %s", testCase.arguments, document.Findings, testCase.wantFile)
		}
		if document.Errors == nil || len(document.Errors) != 0 {
			t.Errorf("errors = %v, want []", document.Errors)
		}
	}
}

// TestCheckTextNotAPipeline: text is always checked, also when it does
// not look like a pipeline.
func TestCheckTextNotAPipeline(t *testing.T) {
	inTempProject(t, map[string]string{})
	document := checkDocumentOf(t, map[string]any{"text": "platform: linux\n"})
	if len(document.Findings) == 0 {
		t.Error("no findings for a task file checked as a pipeline")
	}
}

// TestCheckTextConfig: the config search starts at the folder of filename,
// or at the working directory; exclude does not apply to text.
func TestCheckTextConfig(t *testing.T) {
	inTempProject(t, map[string]string{
		".concourse-lint.yml":     "rules:\n  CL002: warning\nexclude: [\"**\"]\n",
		"sub/.concourse-lint.yml": "rules:\n  CL002: info\n",
	})
	cases := []struct {
		arguments    map[string]any
		wantSeverity string
	}{
		{map[string]any{"text": brokenPipeline}, "warning"},
		{map[string]any{"text": brokenPipeline, "filename": "pipeline.yml"}, "warning"},
		{map[string]any{"text": brokenPipeline, "filename": "sub/pipeline.yml"}, "info"},
		{map[string]any{"text": brokenPipeline, "filename": "sub/deeper/pipeline.yml"}, "info"},
	}
	for _, testCase := range cases {
		document := checkDocumentOf(t, testCase.arguments)
		if len(document.Findings) != 1 || document.Findings[0].Severity != testCase.wantSeverity {
			t.Errorf("%v: findings %+v, want one CL002 as %s", testCase.arguments["filename"], document.Findings, testCase.wantSeverity)
		}
	}
}

func TestCheckTextBrokenConfig(t *testing.T) {
	inTempProject(t, map[string]string{".concourse-lint.yml": "rules:\n  CL999: off\n"})
	result := callCheckPipeline(t, map[string]any{"text": brokenPipeline})
	if !result.isError || !strings.Contains(result.text, ".concourse-lint.yml:2:3:") {
		t.Errorf("result = %+v, want a tool error at the position of CL999", result)
	}
}

func TestCheckArgumentErrors(t *testing.T) {
	inTempProject(t, map[string]string{})
	cases := []struct {
		arguments any
		wantText  string
	}{
		{map[string]any{}, "text or paths"},
		{nil, "text or paths"},
		{map[string]any{"filename": "a.yml"}, "text or paths"},
		{map[string]any{"text": 3}, `"text"`},
		{map[string]any{"text": "jobs: []", "format": "sarif"}, "format"},
		{map[string]any{"text": "jobs: []", "filename": "../outside.yml"}, "outside the working directory"},
	}
	for _, testCase := range cases {
		result := callCheckPipeline(t, testCase.arguments)
		if !result.isError || !strings.Contains(result.text, testCase.wantText) {
			t.Errorf("arguments %v: result %+v, want a tool error containing %q", testCase.arguments, result, testCase.wantText)
		}
	}
}
