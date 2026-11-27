package parse_test

import (
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestWrongTypes: every case was checked with fly 8.3.4. A wrong type or an
// invalid step is one CL001 where fly rejects the pipeline, and no finding
// where fly accepts it.
func TestWrongTypes(t *testing.T) {
	const resources = "resources:\n- name: repo\n  type: mock\n  source: {}\n"
	const getRepo = "jobs:\n- name: j\n  plan:\n  - get: repo\n"
	testCases := []struct {
		name   string
		source string
		// 0: no finding expected.
		wantLine int
		// Part of the message, and of the hint, that must appear.
		wantMessage string
		wantHint    string
	}{
		{name: "resource as text", source: "resources: [repo]\n" + getRepo,
			wantLine: 1, wantMessage: `resource must be a mapping, not the string "repo"`},
		{name: "source as list", source: "resources:\n- name: repo\n  type: mock\n  source: [a]\n" + getRepo,
			wantLine: 4, wantMessage: "source must be a mapping, not a list"},
		{name: "empty source", source: "resources:\n- name: repo\n  type: mock\n  source:\n" + getRepo},
		{name: "name as list", source: "resources:\n- name: [repo]\n  type: mock\n  source: {}\n" + getRepo,
			wantLine: 2, wantMessage: "name must be a string, not a list"},
		{name: "group jobs as text", source: resources + "groups:\n- name: g\n  jobs: j\n" + getRepo,
			wantLine: 7, wantMessage: `jobs must be a list, not the string "j"`, wantHint: "[j]"},
		{name: "plan as mapping", source: resources + "jobs:\n- name: j\n  plan:\n    get: repo\n",
			wantLine: 7, wantMessage: "plan must be a list, not a mapping"},
		{name: "plan item as text", source: resources + getRepo + "  - repo\n",
			wantLine: 9, wantMessage: `step must be a mapping, not the string "repo"`},
		{name: "get params as text", source: resources + getRepo + "    params: a\n",
			wantLine: 9, wantMessage: `params must be a mapping, not the string "a"`},
		{name: "number as step name", source: resources + "jobs:\n- name: j\n  plan:\n  - get: 123\n",
			wantLine: 8, wantMessage: "get must be a string, not the number 123", wantHint: `quote it: "123"`},
		{name: "quoted number", source: resources + getRepo + "    attempts: \"2\"\n",
			wantLine: 9, wantMessage: `attempts must be a whole number, not the string "2"`},
		{name: "whole number written as 2.0", source: resources + getRepo + "    attempts: 2.0\n"},
		{name: "fraction", source: resources + getRepo + "    attempts: 1.5\n",
			wantLine: 9, wantMessage: "attempts must be a whole number, not the number 1.5"},
		{name: "job max_in_flight 2.0", source: resources + "jobs:\n- name: j\n  max_in_flight: 2.0\n  plan:\n  - get: repo\n"},
		{name: "in_parallel as text", source: resources + "jobs:\n- name: j\n  plan:\n  - in_parallel: repo\n",
			wantLine: 8, wantMessage: `in_parallel must be a list of steps or a mapping, not the string "repo"`},
		{name: "across as mapping", source: resources + getRepo + "    across: {var: v, values: [a]}\n",
			wantLine: 9, wantMessage: "across must be a list, not a mapping"},
		{name: "try with a list", source: resources + "jobs:\n- name: j\n  plan:\n  - try: [{get: repo}]\n",
			wantLine: 8, wantMessage: "try must be a mapping, not a list"},
		{name: "hook with a list", source: resources + getRepo + "  on_failure: [{get: repo}]\n",
			wantLine: 9, wantMessage: "on_failure must be a mapping, not a list"},
		{name: "task inputs as text", source: resources + getRepo +
			"  - task: t\n    config:\n      platform: linux\n      image_resource: {type: mock, source: {}}\n      inputs: repo\n      run: {path: x}\n",
			wantLine: 13, wantMessage: `inputs must be a list, not the string "repo"`},
		{name: "input_mapping as list", source: resources + getRepo + "  - task: t\n    file: repo/t.yml\n    input_mapping: [a]\n",
			wantLine: 11, wantMessage: "input_mapping must be a mapping, not a list"},
		{name: "infinity", source: "resources:\n- name: repo\n  type: mock\n  source: {k: .inf}\n" + getRepo,
			wantLine: 4, wantMessage: ".inf", wantHint: `quote it: ".inf"`},
		{name: "task with file and config", source: resources + getRepo +
			"  - task: t\n    file: repo/t.yml\n    config:\n      platform: linux\n      image_resource: {type: mock, source: {}}\n      run: {path: x}\n",
			wantLine: 9, wantMessage: "task step must have either file: or config:, not both"},
		{name: "task file from a variable", source: resources + getRepo + "  - task: t\n    file: ((task_file))\n"},
		{name: "step without a step type", source: resources + getRepo + "  - timeout: 5m\n",
			wantLine: 9, wantMessage: "step has no step type"},
		{name: "step type with a typo", source: resources + getRepo + "  - tsak: t\n    file: repo/t.yml\n",
			wantLine: 9, wantMessage: "step has no step type", wantHint: `did you mean "task"?`},
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
