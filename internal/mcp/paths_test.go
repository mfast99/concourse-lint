package mcp

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/lint"
	"github.com/mfast99/concourse-lint/internal/spectest"
)

// findingFiles lists the file of every finding.
func (document checkDocument) findingFiles() []string {
	var files []string
	for _, finding := range document.Findings {
		files = append(files, finding.File)
	}
	return files
}

// TestCheckPathsLikeTheCLI: folders recursively and sorted, only pipelines,
// no dot folders, exclude also for a file named directly, the config per
// file, the path as given.
func TestCheckPathsLikeTheCLI(t *testing.T) {
	root := inTempProject(t, map[string]string{
		".concourse-lint.yml":           "rules:\n  CL002: warning\nexclude: [vendor]\n",
		"ci/b.yml":                      brokenPipeline,
		"ci/a/pipeline.yaml":            brokenPipeline,
		"ci/task.yml":                   "platform: linux\nrun: {path: ls}\n",
		"ci/.hidden/pipeline.yml":       brokenPipeline,
		"ci/strict/.concourse-lint.yml": "rules:\n  CL002: info\n",
		"ci/strict/p.yml":               brokenPipeline,
		"vendor/b.yml":                  brokenPipeline,
		"named.yml":                     "platform: linux\n",
	})
	document := checkDocumentOf(t, map[string]any{"paths": []string{"ci", "vendor/b.yml", filepath.Join(root, "ci", "b.yml")}})
	wantFiles := []string{filepath.FromSlash("ci/a/pipeline.yaml"), filepath.FromSlash("ci/b.yml"), filepath.FromSlash("ci/strict/p.yml"), filepath.Join(root, "ci", "b.yml")}
	if got := document.findingFiles(); !slices.Equal(got, wantFiles) {
		t.Errorf("files = %v, want %v", got, wantFiles)
	}
	for _, finding := range document.Findings {
		wantSeverity := "warning"
		if strings.Contains(finding.File, "strict") {
			wantSeverity = "info"
		}
		if finding.Rule != "CL002" || finding.Severity != wantSeverity {
			t.Errorf("%s: %s %s, want CL002 %s", finding.File, finding.Rule, finding.Severity, wantSeverity)
		}
	}
	if len(document.Errors) != 0 {
		t.Errorf("errors = %v, want none", document.Errors)
	}

	// A file named directly is checked even when it is no pipeline.
	if named := checkDocumentOf(t, map[string]any{"paths": []string{"named.yml"}}); len(named.Findings) == 0 {
		t.Error("named.yml: no findings, want it checked as a pipeline")
	}
}

// TestCheckPathsUnreadable: a missing file goes into errors, the others are
// still checked.
func TestCheckPathsUnreadable(t *testing.T) {
	inTempProject(t, map[string]string{"b.yml": brokenPipeline})
	document := checkDocumentOf(t, map[string]any{"paths": []string{"missing.yml", "b.yml"}})
	if len(document.Errors) != 1 || document.Errors[0].File != "missing.yml" {
		t.Errorf("errors = %v, want one for missing.yml", document.Errors)
	}
	if got := document.findingFiles(); !slices.Equal(got, []string{"b.yml"}) {
		t.Errorf("files = %v, want b.yml", got)
	}
}

// outsideProject creates a project (the working directory) next to a folder
// "outside" with a pipeline, and changes into the project.
func outsideProject(t *testing.T) (project, outside string) {
	t.Helper()
	base := t.TempDir()
	outside = filepath.Join(base, "outside")
	project = filepath.Join(base, "project")
	for _, folder := range []string{filepath.Join(outside, "sub"), filepath.Join(project, ".git"), filepath.Join(project, "ci")} {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{filepath.Join(outside, "secret.yml"), filepath.Join(project, "ci", "b.yml")} {
		if err := os.WriteFile(path, []byte(brokenPipeline), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	links := map[string]string{
		filepath.Join(project, "out"):            outside,
		filepath.Join(project, "deep"):           filepath.Join(outside, "sub"),
		filepath.Join(project, "ci", "leak.yml"): filepath.Join(outside, "secret.yml"),
	}
	for link, target := range links {
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("no symlinks here: %v", err)
		}
	}
	t.Chdir(project)
	return project, outside
}

// TestCheckPathsOutside: a path given that lies outside the working
// directory (also through a symlink) is a tool error, and nothing is
// checked.
func TestCheckPathsOutside(t *testing.T) {
	_, outside := outsideProject(t)
	cases := [][]string{
		{"../outside/secret.yml"},
		{filepath.Join(outside, "secret.yml")},
		{"out/secret.yml"},
		{"out"},
		// Lexically "secret.yml" in the project, but deep/.. is outside.
		{"deep/../secret.yml"},
		{"ci/b.yml", "../outside"},
	}
	for _, paths := range cases {
		result := callCheckPipeline(t, map[string]any{"paths": paths})
		if !result.isError || !strings.Contains(result.text, "outside the working directory") {
			t.Errorf("paths %v: result %+v, want a tool error", paths, result)
		}
	}
}

// TestCheckPathsSymlinkInFolder: a file found in a folder that resolves to
// outside is not read and goes into errors.
func TestCheckPathsSymlinkInFolder(t *testing.T) {
	outsideProject(t)
	document := checkDocumentOf(t, map[string]any{"paths": []string{"ci"}})
	if got := document.findingFiles(); !slices.Equal(got, []string{filepath.FromSlash("ci/b.yml")}) {
		t.Errorf("files = %v, want only ci/b.yml", got)
	}
	if len(document.Errors) != 1 || document.Errors[0].File != filepath.FromSlash("ci/leak.yml") || !strings.Contains(document.Errors[0].Message, "outside the working directory") {
		t.Errorf("errors = %v, want ci/leak.yml outside the working directory", document.Errors)
	}
}

func TestCheckPathsArgumentErrors(t *testing.T) {
	inTempProject(t, map[string]string{"b.yml": brokenPipeline})
	cases := []struct {
		arguments map[string]any
		wantText  string
	}{
		{map[string]any{"text": brokenPipeline, "paths": []string{"b.yml"}}, "exactly one"},
		{map[string]any{"paths": []string{}}, "paths"},
		{map[string]any{"paths": []string{"b.yml"}, "filename": "b.yml"}, "filename"},
		{map[string]any{"paths": "b.yml"}, `"paths"`},
		{map[string]any{"paths": []string{"-"}}, "text"},
		{map[string]any{"paths": []string{""}}, "empty"},
	}
	for _, testCase := range cases {
		result := callCheckPipeline(t, testCase.arguments)
		if !result.isError || !strings.Contains(result.text, testCase.wantText) {
			t.Errorf("arguments %v: result %+v, want a tool error containing %q", testCase.arguments, result, testCase.wantText)
		}
	}
}

func TestCheckPathsBrokenConfig(t *testing.T) {
	inTempProject(t, map[string]string{".concourse-lint.yml": "rules:\n  CL999: off\n", "b.yml": brokenPipeline})
	result := callCheckPipeline(t, map[string]any{"paths": []string{"b.yml"}})
	if !result.isError || !strings.Contains(result.text, ".concourse-lint.yml:2:3:") {
		t.Errorf("result = %+v, want a tool error at the position of CL999", result)
	}
}

// TestConfigSpec runs the scenarios in spec/config/ through check_pipeline
// with paths: every pipeline named directly, with the config the search
// finds (exclude included), or a tool error for a broken config.
func TestConfigSpec(t *testing.T) {
	catalog := spectest.LoadCatalog(t)
	repositoryRoot := filepath.Dir(spectest.Dir())
	t.Chdir(repositoryRoot)
	for _, scenario := range spectest.ConfigScenarios(t) {
		pipelines, configs := spectest.ConfigScenarioFiles(t, scenario)
		brokenConfig := false
		for _, configPath := range configs {
			source, _ := os.ReadFile(configPath)
			brokenConfig = brokenConfig || spectest.Marker(source, "want-config-error")
		}
		for _, pipelinePath := range pipelines {
			relative, _ := filepath.Rel(repositoryRoot, pipelinePath)
			name := spectest.Name(pipelinePath)
			result := callCheckPipeline(t, map[string]any{"paths": []string{relative}})
			if brokenConfig {
				if !result.isError || !strings.Contains(result.text, ".concourse-lint.y") {
					t.Errorf("%s: result %+v, want a tool error naming the config", name, result)
				}
				continue
			}
			var document checkDocument
			if err := json.Unmarshal([]byte(result.text), &document); err != nil || result.isError {
				t.Errorf("%s: no document: %s", name, result.text)
				continue
			}
			source, _ := os.ReadFile(pipelinePath)
			if spectest.Marker(source, "want-excluded") {
				if len(document.Findings) != 0 || len(document.Errors) != 0 {
					t.Errorf("%s: excluded, but got %+v", name, document)
				}
				continue
			}
			config, err := lint.ConfigFor(pipelinePath)
			if err != nil {
				t.Fatal(err)
			}
			configured := maps.Clone(catalog)
			for rule, severity := range catalog {
				configured[rule] = config.Severity(rule, severity)
			}
			for _, problem := range spectest.Compare(spectest.ParseExpectations(source), configured, document.toFindings()) {
				t.Errorf("%s: %s", name, problem)
			}
		}
	}
}
