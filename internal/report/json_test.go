package report

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// mixedResult has every case the writers must handle: several files, one
// without findings, stdin, all severities, with and without hint, and an
// input that could not be read.
var mixedResult = Result{
	Files: []File{
		{Path: "ci/pipeline.yml", Findings: []analysis.Finding{publicJob, unknownResource}},
		{Path: "clean.yml"},
		{Path: StdinPath, Findings: []analysis.Finding{revealedVar}},
	},
	Errors:      []InputError{{Path: "missing.yml", Message: "open missing.yml: no such file or directory"}},
	ToolVersion: "v1.2.3",
}

func TestJSONGolden(t *testing.T) {
	testCases := []struct {
		golden string
		result Result
	}{
		{golden: "json-empty", result: Result{}},
		{golden: "json-mixed", result: mixedResult},
	}
	for _, testCase := range testCases {
		t.Run(testCase.golden, func(t *testing.T) {
			var output strings.Builder
			if err := JSON(&output, testCase.result); err != nil {
				t.Fatal(err)
			}
			compareGolden(t, testCase.golden, []byte(output.String()))
		})
	}
}

// TestJSONEveryFieldPresent guards the contract for consumers that index
// fields directly (e.g. finding["hint"] in Python): every key is always
// there and lists are never null.
func TestJSONEveryFieldPresent(t *testing.T) {
	testCases := []struct {
		name   string
		result Result
	}{
		{name: "empty", result: Result{}},
		{name: "files without findings", result: Result{Files: []File{{Path: "clean.yml"}}}},
		{name: "mixed", result: mixedResult},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var output strings.Builder
			if err := JSON(&output, testCase.result); err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			if err := json.Unmarshal([]byte(output.String()), &document); err != nil {
				t.Fatalf("not valid JSON: %v\n%s", err, output.String())
			}
			assertKeys(t, "document", document, "errors", "findings")
			findings, isList := document["findings"].([]any)
			if !isList {
				t.Fatalf("findings is %T, want a list", document["findings"])
			}
			for _, item := range findings {
				findingObject := item.(map[string]any)
				assertKeys(t, "finding", findingObject, "docs", "file", "hint", "message", "pos", "rule", "severity", "suggestion")
				assertKeys(t, "pos", findingObject["pos"].(map[string]any), "col", "end_col", "end_line", "line")
			}
			inputErrors, isList := document["errors"].([]any)
			if !isList {
				t.Fatalf("errors is %T, want a list", document["errors"])
			}
			for _, item := range inputErrors {
				assertKeys(t, "error", item.(map[string]any), "file", "message")
			}
		})
	}
}

// assertKeys checks that object has exactly the wanted keys (sorted).
func assertKeys(t *testing.T, what string, object map[string]any, wanted ...string) {
	t.Helper()
	keys := slices.Sorted(maps.Keys(object))
	if !slices.Equal(keys, wanted) {
		t.Errorf("%s has keys %v, want %v", what, keys, wanted)
	}
}
