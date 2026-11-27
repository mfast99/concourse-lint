package parse_test

import (
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestValueFormats: values with a fixed format that fly 8.3.4 checks when it
// reads the pipeline. Every case was checked with
// fly.
func TestValueFormats(t *testing.T) {
	resourceWith := func(field string) string {
		return "resources:\n- name: repo\n  type: mock\n  source: {}\n  " + field + "\njobs:\n- name: j\n  plan:\n  - get: repo\n"
	}
	stepWith := func(field string) string {
		return "resources:\n- name: repo\n  type: mock\n  source: {}\njobs:\n- name: j\n  plan:\n  - get: repo\n    " + field + "\n"
	}
	testCases := []struct {
		name   string
		source string
		// 0: no finding expected.
		wantLine    int
		wantMessage string
		wantHint    string
	}{
		{name: "check_every 10m", source: resourceWith("check_every: 10m")},
		{name: "check_every 1h30m", source: resourceWith("check_every: 1h30m")},
		{name: "check_every 1.5h", source: resourceWith("check_every: 1.5h")},
		{name: "check_every never", source: resourceWith("check_every: never")},
		{name: "check_every empty", source: resourceWith(`check_every: ""`)},
		{name: "check_every variable", source: resourceWith("check_every: ((interval))")},
		{name: "check_every soon", source: resourceWith("check_every: soon"),
			wantLine: 5, wantMessage: `check_every must be a duration like 10m or "never", not "soon"`},
		{name: "check_every in days", source: resourceWith("check_every: 1d"),
			wantLine: 5, wantMessage: `not "1d"`, wantHint: "24h"},
		{name: "check_every with a space", source: resourceWith("check_every: 10 m"),
			wantLine: 5, wantMessage: `not "10 m"`},
		{name: "check_every as a number", source: resourceWith("check_every: 10"),
			wantLine: 5, wantMessage: "check_every must be a duration like 10m or \"never\", not the number 10", wantHint: "add a unit"},
		{name: "max_in_flight all", source: stepWith("across: [{var: v, values: [a], max_in_flight: all}]")},
		{name: "max_in_flight number", source: stepWith("across: [{var: v, values: [a], max_in_flight: 2}]")},
		{name: "max_in_flight some", source: stepWith("across: [{var: v, values: [a], max_in_flight: some}]"),
			wantLine: 9, wantMessage: `max_in_flight must be a whole number or "all", not the string "some"`},
		{name: "version latest", source: stepWith("version: latest")},
		{name: "version mapping", source: stepWith("version: {ref: abc123}")},
		{name: "version list", source: stepWith("version: [abc123]"),
			wantLine: 9, wantMessage: "version must be latest, every or a mapping of version fields, not a list"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, findings := parse.Parse([]byte(testCase.source))
			if testCase.wantLine == 0 {
				if len(findings) != 0 {
					t.Errorf("findings: %+v, want none", findings)
				}
				return
			}
			if len(findings) != 1 {
				t.Fatalf("findings: %+v, want one CL001", findings)
			}
			finding := findings[0]
			if finding.Rule != "CL001" || finding.Position.Line != testCase.wantLine ||
				!strings.Contains(finding.Message, testCase.wantMessage) || !strings.Contains(finding.Hint, testCase.wantHint) {
				t.Errorf("got  %s line %d %q (hint %q)\nwant CL001 line %d containing %q (hint containing %q)",
					finding.Rule, finding.Position.Line, finding.Message, finding.Hint,
					testCase.wantLine, testCase.wantMessage, testCase.wantHint)
			}
		})
	}
}
