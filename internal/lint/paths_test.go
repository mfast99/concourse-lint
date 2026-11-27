package lint

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	// cleanPipeline has no findings.
	cleanPipeline = "resources:\n- name: repo\n  type: git\n  source: {uri: https://example.com/repo.git}\njobs:\n- name: build\n  plan:\n  - get: repo\n"
	// brokenPipeline has one CL002 (unknown resource).
	brokenPipeline = "resources:\n- name: repo\n  type: git\n  source: {uri: https://example.com/repo.git}\njobs:\n- name: build\n  plan:\n  - get: repo\n  - get: missing\n"
	// fragment is a job file without resources (CL023).
	fragment = "jobs:\n- name: build\n  plan:\n  - get: repo\n"
	// yttTemplate is a ytt template of brokenPipeline (CL022).
	yttTemplate = "#@ load(\"@ytt:data\", \"data\")\n" + brokenPipeline
)

// writeFiles creates files below a temporary folder and returns it.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// checkedPaths lists the paths of the checked files, relative to root.
func checkedPaths(t *testing.T, root string, result PathsResult) []string {
	t.Helper()
	var paths []string
	for _, file := range result.Files {
		relative, err := filepath.Rel(root, file.Path)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, filepath.ToSlash(relative))
	}
	return paths
}

// TestCheckPathsFolder: a folder gives its pipelines, recursively and
// sorted; other YAML files, templates, fragments and dot folders are
// skipped.
func TestCheckPathsFolder(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"b.yml":              brokenPipeline,
		"a/pipeline.yaml":    cleanPipeline,
		"task.yml":           "platform: linux\nrun: {path: ls}\n",
		".github/ci.yml":     cleanPipeline,
		"notes.txt":          cleanPipeline,
		"a/nested/other.yml": cleanPipeline,
		"templates/ytt.yml":  yttTemplate,
		"jobs/fragment.yml":  fragment,
	})
	result, err := CheckPaths([]string{root}, PathOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a/nested/other.yml", "a/pipeline.yaml", "b.yml"}
	if got := checkedPaths(t, root, result); !slices.Equal(got, want) {
		t.Errorf("checked %v, want %v", got, want)
	}
	if findings := result.Files[2].Findings; len(findings) != 1 || findings[0].Rule != "CL002" {
		t.Errorf("findings of b.yml = %v, want one CL002", findings)
	}
}

// TestCheckPathsNamedFile: a file given by name is checked even when it does
// not look like a pipeline; a missing one is an input error and the others
// are still checked.
func TestCheckPathsNamedFile(t *testing.T) {
	root := writeFiles(t, map[string]string{"task.yml": "platform: linux\n", "b.yml": brokenPipeline})
	missing := filepath.Join(root, "missing.yml")
	result, err := CheckPaths([]string{filepath.Join(root, "task.yml"), missing, filepath.Join(root, "b.yml")}, PathOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := checkedPaths(t, root, result); !slices.Equal(got, []string{"task.yml", "b.yml"}) {
		t.Errorf("checked %v, want task.yml and b.yml", got)
	}
	if len(result.Errors) != 1 || result.Errors[0].Path != missing {
		t.Errorf("errors = %v, want one for %s", result.Errors, missing)
	}
}

// TestCheckPathsDanglingSymlink: in a folder, a YAML symlink whose target is
// missing (a submodule not checked out, an editor lock file) is skipped;
// named directly it is an input error.
func TestCheckPathsDanglingSymlink(t *testing.T) {
	root := writeFiles(t, map[string]string{"b.yml": brokenPipeline})
	if err := os.Symlink(filepath.Join(root, "submodule", "missing.yml"), filepath.Join(root, "linked.yml")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if err := os.Symlink("user@host.1234", filepath.Join(root, ".#lock.yml")); err != nil {
		t.Fatal(err)
	}
	result, err := CheckPaths([]string{root}, PathOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := checkedPaths(t, root, result); !slices.Equal(got, []string{"b.yml"}) {
		t.Errorf("checked %v, want b.yml", got)
	}
	if len(result.Errors) != 0 {
		t.Errorf("errors = %v, want none", result.Errors)
	}
	named := filepath.Join(root, "linked.yml")
	result, err = CheckPaths([]string{named}, PathOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) != 1 || result.Errors[0].Path != named {
		t.Errorf("errors = %v, want one for %s", result.Errors, named)
	}
}

// TestCheckPathsTemplate: a template named directly gets one CL022. With
// CL022 off, a template in a folder is checked like a pipeline.
func TestCheckPathsTemplate(t *testing.T) {
	root := writeFiles(t, map[string]string{"ytt.yml": yttTemplate})
	result, err := CheckPaths([]string{filepath.Join(root, "ytt.yml")}, PathOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || len(result.Files[0].Findings) != 1 || result.Files[0].Findings[0].Rule != "CL022" {
		t.Errorf("files = %v, want ytt.yml with one CL022", result.Files)
	}

	root = writeFiles(t, map[string]string{
		".git/HEAD":           "",
		".concourse-lint.yml": "rules:\n  CL022: off\n",
		"ytt.yml":             yttTemplate,
	})
	result, err = CheckPaths([]string{root}, PathOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || len(result.Files[0].Findings) != 1 || result.Files[0].Findings[0].Rule != "CL002" {
		t.Errorf("files = %v, want ytt.yml with one CL002", result.Files)
	}
}

// TestCheckPathsConfig: the nearest config applies per file, exclude
// included, also for a file named directly.
func TestCheckPathsConfig(t *testing.T) {
	root := writeFiles(t, map[string]string{
		".git/HEAD":           "",
		".concourse-lint.yml": "rules:\n  CL002: warning\nexclude: [vendor]\n",
		"b.yml":               brokenPipeline,
		"vendor/b.yml":        brokenPipeline,
	})
	result, err := CheckPaths([]string{root, filepath.Join(root, "vendor", "b.yml")}, PathOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := checkedPaths(t, root, result); !slices.Equal(got, []string{"b.yml"}) {
		t.Fatalf("checked %v, want only b.yml", got)
	}
	if findings := result.Files[0].Findings; len(findings) != 1 || findings[0].Severity != "warning" {
		t.Errorf("findings = %v, want one CL002 as warning", findings)
	}
}

// TestCheckPathsExplicitConfig: ConfigPath applies to every input instead of
// the search.
func TestCheckPathsExplicitConfig(t *testing.T) {
	root := writeFiles(t, map[string]string{
		".git/HEAD":           "",
		"strict.yml":          "rules:\n  CL002: info\n",
		".concourse-lint.yml": "rules:\n  CL002: warning\n",
		"b.yml":               brokenPipeline,
	})
	result, err := CheckPaths([]string{filepath.Join(root, "b.yml")}, PathOptions{ConfigPath: filepath.Join(root, "strict.yml")})
	if err != nil {
		t.Fatal(err)
	}
	if findings := result.Files[0].Findings; len(findings) != 1 || findings[0].Severity != "info" {
		t.Errorf("findings = %v, want one CL002 as info", findings)
	}
}

// TestCheckPathsBrokenConfig: a broken config stops the run with its
// position.
func TestCheckPathsBrokenConfig(t *testing.T) {
	root := writeFiles(t, map[string]string{".git/HEAD": "", ".concourse-lint.yml": "rules:\n  CL999: off\n", "b.yml": brokenPipeline})
	_, err := CheckPaths([]string{filepath.Join(root, "b.yml")}, PathOptions{})
	if err == nil || !strings.Contains(err.Error(), ".concourse-lint.yml:2:3:") {
		t.Errorf("err = %v, want the position of CL999", err)
	}
}

// TestCheckPathsStdin: "-" reads Stdin and is reported as StdinName.
func TestCheckPathsStdin(t *testing.T) {
	result, err := CheckPaths([]string{"-"}, PathOptions{Stdin: strings.NewReader(brokenPipeline), StdinName: "<stdin>"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != "<stdin>" || len(result.Files[0].Findings) != 1 {
		t.Errorf("result = %+v, want <stdin> with one finding", result)
	}
}

// TestCheckPathsAllow: a file that Allow refuses is not read and becomes an
// input error with Allow's message.
func TestCheckPathsAllow(t *testing.T) {
	root := writeFiles(t, map[string]string{"a.yml": cleanPipeline, "b.yml": brokenPipeline})
	refused := filepath.Join(root, "b.yml")
	allow := func(path string) error {
		if path == refused {
			return errors.New("not allowed")
		}
		return nil
	}
	result, err := CheckPaths([]string{root}, PathOptions{Allow: allow})
	if err != nil {
		t.Fatal(err)
	}
	if got := checkedPaths(t, root, result); !slices.Equal(got, []string{"a.yml"}) {
		t.Errorf("checked %v, want only a.yml", got)
	}
	if len(result.Errors) != 1 || result.Errors[0].Path != refused || result.Errors[0].Message != "not allowed" {
		t.Errorf("errors = %+v, want b.yml: not allowed", result.Errors)
	}
}
