package parse_test

import (
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestRuntimeValues: values that fly accepts without a check. CL019 where the
// step or check then fails every time it runs,
// CL020 where Concourse silently ignores the word.
func TestRuntimeValues(t *testing.T) {
	const resources = "resources:\n- name: repo\n  type: mock\n  source: {}\n"
	stepWith := func(step string) string {
		return resources + "jobs:\n- name: j\n  plan:\n  - get: repo\n" + step
	}
	testCases := []struct {
		name   string
		source string
		// Empty: no finding expected.
		wantRule    string
		wantMessage string
		wantHint    string
	}{
		{name: "timeout in days", source: stepWith("    timeout: 1d\n"),
			wantRule: "CL019", wantMessage: `timeout must be a duration like 10m, not "1d": the step fails every time it runs`, wantHint: "24h"},
		{name: "timeout 1h30m", source: stepWith("    timeout: 1h30m\n")},
		{name: "timeout from a variable", source: stepWith("    timeout: ((timeout))\n")},
		{name: "check_timeout in words", source: "resources:\n- name: repo\n  type: mock\n  check_timeout: 5 minutes\n  source: {}\njobs:\n- name: j\n  plan:\n  - get: repo\n",
			wantRule: "CL019", wantMessage: `check_timeout must be a duration like 10m, not "5 minutes": every check fails`, wantHint: "valid units"},
		{name: "load_var format typo", source: stepWith("  - load_var: x\n    file: repo/x\n    format: jsn\n"),
			wantRule: "CL019", wantMessage: `format must be json, yaml, yml, raw or trim, not "jsn": load_var fails every time it runs`, wantHint: `did you mean "json"?`},
		{name: "load_var format trim", source: stepWith("  - load_var: x\n    file: repo/x\n    format: trim\n")},
		{name: "version typo", source: stepWith("    version: evry\n"),
			wantRule: "CL020", wantMessage: `unknown version "evry", which Concourse ignores and takes the latest version`, wantHint: `did you mean "every"?`},
		{name: "version every", source: stepWith("    version: every\n")},
		{name: "version from a variable", source: stepWith("    version: ((version))\n")},
		{name: "put inputs typo", source: resources + "jobs:\n- name: j\n  plan:\n  - put: repo\n    inputs: al\n",
			wantRule: "CL020", wantMessage: `unknown inputs "al", which Concourse ignores and uses its default`, wantHint: `did you mean "all"?`},
		{name: "put inputs detect", source: resources + "jobs:\n- name: j\n  plan:\n  - put: repo\n    inputs: detect\n"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, findings := parse.Parse([]byte(testCase.source))
			if testCase.wantRule == "" {
				if len(findings) != 0 {
					t.Errorf("findings: %+v, want none", findings)
				}
				return
			}
			if len(findings) != 1 {
				t.Fatalf("findings: %+v, want one %s", findings, testCase.wantRule)
			}
			finding := findings[0]
			if finding.Rule != testCase.wantRule || finding.Message != testCase.wantMessage || !strings.Contains(finding.Hint, testCase.wantHint) {
				t.Errorf("got  %s %q (hint %q)\nwant %s %q (hint containing %q)",
					finding.Rule, finding.Message, finding.Hint, testCase.wantRule, testCase.wantMessage, testCase.wantHint)
			}
		})
	}
}
