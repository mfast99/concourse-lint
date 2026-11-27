package report

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

func TestSARIFGolden(t *testing.T) {
	testCases := []struct {
		golden string
		result Result
	}{
		{golden: "sarif-empty", result: Result{}},
		{golden: "sarif-mixed", result: mixedResult},
	}
	for _, testCase := range testCases {
		t.Run(testCase.golden, func(t *testing.T) {
			var output strings.Builder
			if err := SARIF(&output, testCase.result); err != nil {
				t.Fatal(err)
			}
			compareGolden(t, testCase.golden, []byte(output.String()))
		})
	}
}

// writeSARIF writes result and reads the log back, for checks on single
// values. The golden files cover the overall shape.
func writeSARIF(t *testing.T, result Result) sarifLog {
	t.Helper()
	var output strings.Builder
	if err := SARIF(&output, result); err != nil {
		t.Fatal(err)
	}
	var log sarifLog
	if err := json.Unmarshal([]byte(output.String()), &log); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, output.String())
	}
	if len(log.Runs) != 1 {
		t.Fatalf("%d runs, want 1\n%s", len(log.Runs), output.String())
	}
	return log
}

func TestSARIFLevels(t *testing.T) {
	testCases := []struct {
		severity analysis.Severity
		want     string
	}{
		{severity: analysis.SeverityError, want: "error"},
		{severity: analysis.SeverityWarning, want: "warning"},
		// SARIF has no "info"; "note" is its lowest level that is shown.
		{severity: analysis.SeverityInfo, want: "note"},
	}
	for _, testCase := range testCases {
		t.Run(string(testCase.severity), func(t *testing.T) {
			someFinding := finding("CL002", testCase.severity, 1, 1, "message", "")
			log := writeSARIF(t, Result{Files: []File{{Path: "p.yml", Findings: []analysis.Finding{someFinding}}}})
			results := log.Runs[0].Results
			if len(results) != 1 || results[0].Level != testCase.want {
				t.Errorf("results %+v, want one with level %q", results, testCase.want)
			}
		})
	}
}

func TestSARIFURI(t *testing.T) {
	testCases := []struct {
		path string
		want string
	}{
		{path: "pipeline.yml", want: "pipeline.yml"},
		{path: "ci/pipeline.yml", want: "ci/pipeline.yml"},
		{path: "./ci/pipeline.yml", want: "ci/pipeline.yml"},
		{path: "../spec/valid/01-minimal.yml", want: "../spec/valid/01-minimal.yml"},
		{path: "ci/my pipeline.yml", want: "ci/my%20pipeline.yml"},
		{path: "ci/a#b.yml", want: "ci/a%23b.yml"},
		{path: "ci/what?.yml", want: "ci/what%3F.yml"},
		{path: "ci/100%.yml", want: "ci/100%25.yml"},
		// Without "./" a reader would take "a:" as a URI scheme.
		{path: "a:b.yml", want: "./a:b.yml"},
		// "<" and ">" are not allowed in a URI, so stdin is written as "stdin".
		{path: StdinPath, want: "stdin"},
	}
	// An absolute path is a file URI; on Windows it has a drive letter.
	if runtime.GOOS == "windows" {
		testCases = append(testCases,
			struct{ path, want string }{path: `C:\ci\pipeline.yml`, want: "file:///C:/ci/pipeline.yml"},
			struct{ path, want string }{path: `C:\ci\my pipeline.yml`, want: "file:///C:/ci/my%20pipeline.yml"},
		)
	} else {
		testCases = append(testCases,
			struct{ path, want string }{path: "/home/ci/pipeline.yml", want: "file:///home/ci/pipeline.yml"},
			struct{ path, want string }{path: "/home/ci/my pipeline.yml", want: "file:///home/ci/my%20pipeline.yml"},
		)
	}
	for _, testCase := range testCases {
		t.Run(testCase.path, func(t *testing.T) {
			if got := sarifURI(testCase.path); got != testCase.want {
				t.Errorf("sarifURI(%q) = %q, want %q", testCase.path, got, testCase.want)
			}
		})
	}
}

// SARIF requires startLine and startColumn to be at least 1, so an unknown
// position must not be written as 0.
func TestSARIFUnknownPosition(t *testing.T) {
	testCases := []struct {
		name       string
		line       int
		column     int
		wantRegion bool
		wantColumn int
	}{
		{name: "line and column", line: 4, column: 7, wantRegion: true, wantColumn: 7},
		{name: "line only", line: 4, column: 0, wantRegion: true, wantColumn: 0},
		{name: "no position", line: 0, column: 0, wantRegion: false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			someFinding := finding("CL001", analysis.SeverityError, testCase.line, testCase.column, "message", "")
			log := writeSARIF(t, Result{Files: []File{{Path: "p.yml", Findings: []analysis.Finding{someFinding}}}})
			results := log.Runs[0].Results
			if len(results) != 1 || len(results[0].Locations) != 1 {
				t.Fatalf("want one result with one location, got %+v", results)
			}
			region := results[0].Locations[0].PhysicalLocation.Region
			if (region != nil) != testCase.wantRegion {
				t.Fatalf("region %+v, want region: %v", region, testCase.wantRegion)
			}
			if region != nil && (region.StartLine != testCase.line || region.StartColumn != testCase.wantColumn) {
				t.Errorf("region %+v, want line %d column %d", region, testCase.line, testCase.wantColumn)
			}
		})
	}
}

// Every rule that fired is listed once in the driver, sorted, and each
// result points to its rule by index.
func TestSARIFRuleIndex(t *testing.T) {
	log := writeSARIF(t, Result{Files: []File{
		{Path: "a.yml", Findings: []analysis.Finding{publicJob, unknownResource, publicJob}},
		{Path: "b.yml", Findings: []analysis.Finding{revealedVar, unknownResource}},
	}})
	run := log.Runs[0]
	var ruleIDs []string
	for _, rule := range run.Tool.Driver.Rules {
		ruleIDs = append(ruleIDs, rule.ID)
	}
	if strings.Join(ruleIDs, ",") != "CL002,CS008,CS009" {
		t.Errorf("driver rules %v, want [CL002 CS008 CS009]", ruleIDs)
	}
	if len(run.Results) != 5 {
		t.Fatalf("%d results, want 5", len(run.Results))
	}
	for _, result := range run.Results {
		if result.RuleIndex < 0 || result.RuleIndex >= len(ruleIDs) || ruleIDs[result.RuleIndex] != result.RuleID {
			t.Errorf("result for %s has ruleIndex %d", result.RuleID, result.RuleIndex)
		}
	}
}

func TestSARIFHintInMessage(t *testing.T) {
	log := writeSARIF(t, Result{Files: []File{{Path: "p.yml", Findings: []analysis.Finding{unknownResource, publicJob}}}})
	results := log.Runs[0].Results
	if len(results) != 2 {
		t.Fatalf("%d results, want 2", len(results))
	}
	if want := "resource 'rep' is not defined (did you mean 'repo'?)"; results[0].Message.Text != want {
		t.Errorf("message %q, want %q", results[0].Message.Text, want)
	}
	if want := "job 'build' has public build logs"; results[1].Message.Text != want {
		t.Errorf("message %q, want %q", results[1].Message.Text, want)
	}
}

func TestSARIFInputErrors(t *testing.T) {
	t.Run("all inputs read", func(t *testing.T) {
		log := writeSARIF(t, Result{Files: []File{{Path: "p.yml"}}})
		invocations := log.Runs[0].Invocations
		if len(invocations) != 1 || !invocations[0].ExecutionSuccessful || len(invocations[0].ToolExecutionNotifications) != 0 {
			t.Errorf("invocations %+v, want one successful without notifications", invocations)
		}
	})
	t.Run("one input missing", func(t *testing.T) {
		log := writeSARIF(t, Result{
			Files:  []File{{Path: "p.yml", Findings: []analysis.Finding{publicJob}}},
			Errors: []InputError{{Path: "ci/missing file.yml", Message: "no such file"}},
		})
		run := log.Runs[0]
		if len(run.Results) != 1 {
			t.Errorf("%d results, want 1: the readable file is still reported", len(run.Results))
		}
		if len(run.Invocations) != 1 || run.Invocations[0].ExecutionSuccessful {
			t.Fatalf("invocations %+v, want one that is not successful", run.Invocations)
		}
		notifications := run.Invocations[0].ToolExecutionNotifications
		if len(notifications) != 1 {
			t.Fatalf("%d notifications, want 1", len(notifications))
		}
		notification := notifications[0]
		if notification.Level != "error" || notification.Message.Text != "no such file" ||
			len(notification.Locations) != 1 ||
			notification.Locations[0].PhysicalLocation.ArtifactLocation.URI != "ci/missing%20file.yml" {
			t.Errorf("notification %+v", notification)
		}
	})
}
