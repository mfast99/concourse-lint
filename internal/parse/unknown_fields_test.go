package parse_test

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestUnknownFieldMessages pins the message and hint of unknown fields: CL001
// where Concourse rejects them, CL018 where
// it ignores them. The spec cases only check rule and line.
func TestUnknownFieldMessages(t *testing.T) {
	const resources = "resources:\n- name: repo\n  type: mock\n  source: {}\n"
	const jobs = "jobs:\n- name: test\n  plan:\n  - get: repo\n"
	testCases := []struct {
		name        string
		source      string
		wantRule    string
		wantMessage string
		wantHint    string
	}{
		{
			name:        "typo in a resource",
			source:      "resources:\n- name: repo\n  type: mock\n  sorce: {}\n" + jobs,
			wantRule:    "CL018",
			wantMessage: `unknown field "sorce" in resource, which Concourse ignores`,
			wantHint:    `did you mean "source"?`,
		},
		{
			name:        "no close key, no hint",
			source:      "resources:\n- name: repo\n  type: mock\n  bogus: 1\n" + jobs,
			wantRule:    "CL018",
			wantMessage: `unknown field "bogus" in resource, which Concourse ignores`,
		},
		{
			name:        "prototype is named as such",
			source:      resources + "prototypes:\n- name: p\n  type: mock\n  sorce: {}\n" + jobs,
			wantRule:    "CL018",
			wantMessage: `unknown field "sorce" in prototype, which Concourse ignores`,
			wantHint:    `did you mean "source"?`,
		},
		{
			name:        "top-level key without anchors",
			source:      resources + jobs + "shared: {retries: 3}\n",
			wantRule:    "CL018",
			wantMessage: `unknown top-level field "shared", which Concourse ignores`,
		},
		{
			name:   "top-level key holding anchors",
			source: resources + jobs + "shared: &retries {attempts: 3}\n",
		},
		{
			name:        "field of another step kind",
			source:      resources + "jobs:\n- name: test\n  plan:\n  - put: repo\n    trigger: true\n",
			wantRule:    "CL001",
			wantMessage: `put step has no field "trigger"`,
			wantHint:    "trigger belongs to get steps",
		},
		{
			name:        "fail_fast without across",
			source:      resources + "jobs:\n- name: test\n  plan:\n  - get: repo\n    fail_fast: true\n",
			wantRule:    "CL001",
			wantMessage: `get step has no field "fail_fast"`,
			wantHint:    "fail_fast only works with across on the same step, or inside an in_parallel block",
		},
		{
			name:   "fail_fast with across",
			source: resources + "jobs:\n- name: test\n  plan:\n  - get: repo\n    across: [{var: v, values: [a]}]\n    fail_fast: true\n",
		},
		{
			name:   "tags on a get step",
			source: resources + "jobs:\n- name: test\n  plan:\n  - get: repo\n    tags: [linux]\n",
		},
		{
			name:        "tags on a try step",
			source:      resources + "jobs:\n- name: test\n  plan:\n  - try: {get: repo}\n    tags: [linux]\n",
			wantRule:    "CL001",
			wantMessage: `try step has no field "tags"`,
			wantHint:    "tags belongs to get, put, task, run, do, in_parallel steps",
		},
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
			if finding.Rule != testCase.wantRule || finding.Message != testCase.wantMessage || finding.Hint != testCase.wantHint {
				t.Errorf("got  %s %q (hint %q)\nwant %s %q (hint %q)",
					finding.Rule, finding.Message, finding.Hint,
					testCase.wantRule, testCase.wantMessage, testCase.wantHint)
			}
		})
	}
}
