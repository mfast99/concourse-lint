package parse_test

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestStringConversions: an unquoted number or boolean in a string field.
// Outside steps Concourse turns it into text, and
// CL017 fires when the text differs from what is written. Inside steps fly
// rejects it (CL001).
func TestStringConversions(t *testing.T) {
	const resources = "resources:\n- name: repo\n  type: mock\n  source: {}\n"
	testCases := []struct {
		name        string
		source      string
		wantJobName string
		// Empty: no finding expected.
		wantRule    string
		wantMessage string
		wantHint    string
	}{
		{name: "job name yes", source: resources + "jobs:\n- name: yes\n  plan:\n  - get: repo\n",
			wantJobName: "true", wantRule: "CL017", wantMessage: `unquoted yes in name becomes "true"`, wantHint: `quote it: "yes"`},
		{name: "job name 1.10", source: resources + "jobs:\n- name: 1.10\n  plan:\n  - get: repo\n",
			wantJobName: "1.1", wantRule: "CL017", wantMessage: `unquoted 1.10 in name becomes "1.1"`, wantHint: `quote it: "1.10"`},
		{name: "job name 0777", source: resources + "jobs:\n- name: 0777\n  plan:\n  - get: repo\n",
			wantJobName: "511", wantRule: "CL017", wantMessage: `unquoted 0777 in name becomes "511"`},
		{name: "job name 12 stays", source: resources + "jobs:\n- name: 12\n  plan:\n  - get: repo\n",
			wantJobName: "12"},
		{name: "quoted job name", source: resources + "jobs:\n- name: \"1.10\"\n  plan:\n  - get: repo\n",
			wantJobName: "1.10"},
		{name: "serial group 1.10", source: resources + "jobs:\n- name: j\n  serial_groups: [1.10]\n  plan:\n  - get: repo\n",
			wantJobName: "j", wantRule: "CL017", wantMessage: `unquoted 1.10 in serial_groups becomes "1.1"`},
		{name: "step name yes", source: resources + "jobs:\n- name: j\n  plan:\n  - get: yes\n",
			wantJobName: "j", wantRule: "CL001", wantMessage: `get must be a string, not yes`, wantHint: `quote it: "yes"`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			pipeline, findings := parse.Parse([]byte(testCase.source))
			if got := pipeline.Jobs[0].Name.Value; got != testCase.wantJobName {
				t.Errorf("job name %q, want %q", got, testCase.wantJobName)
			}
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
			if finding.Rule != testCase.wantRule || finding.Message != testCase.wantMessage || (testCase.wantHint != "" && finding.Hint != testCase.wantHint) {
				t.Errorf("got  %s %q (hint %q)\nwant %s %q (hint %q)",
					finding.Rule, finding.Message, finding.Hint, testCase.wantRule, testCase.wantMessage, testCase.wantHint)
			}
		})
	}
}
