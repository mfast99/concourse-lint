package parse_test

import (
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestRequiredFields: every case was checked with fly 8.3.4. A missing field
// is CL001 only where fly rejects it; what fly
// accepts gives no finding from the parser.
func TestRequiredFields(t *testing.T) {
	const resources = "resources:\n- name: repo\n  type: mock\n  source: {}\n"
	const getRepo = "jobs:\n- name: j\n  plan:\n  - get: repo\n"
	const image = "      image_resource: {type: mock, source: {}}\n"
	testCases := []struct {
		name   string
		source string
		// 0: no finding expected.
		wantLine    int
		wantMessage string
	}{
		{name: "resource without source", source: "resources:\n- name: repo\n  type: mock\n" + getRepo},
		{name: "resource type without source", source: "resource_types:\n- name: t\n  type: mock\nresources:\n- name: repo\n  type: t\n" + getRepo},
		{name: "job without plan", source: resources + getRepo + "- name: k\n"},
		{name: "job with empty plan", source: resources + getRepo + "- name: k\n  plan: []\n"},
		{name: "group without jobs", source: resources + "groups:\n- name: g\n" + getRepo},
		{name: "config without run and image", source: resources + getRepo + "  - task: t\n    config:\n      platform: linux\n"},
		{name: "image_resource without type", source: resources + getRepo + "  - task: t\n    config:\n      platform: linux\n      image_resource: {source: {}}\n"},
		{name: "cache without path", source: resources + getRepo + "  - task: t\n    config:\n      platform: linux\n" + image + "      caches: [{ttl: 1h}]\n"},
		{name: "across variable without var and values", source: resources + getRepo + "    across: [{max_in_flight: 1}]\n"},
		{name: "empty do and in_parallel", source: resources + getRepo + "  - do: []\n  - in_parallel: []\n"},
		{name: "empty resource name", source: "resources:\n- name: \"\"\n  type: mock\n" + getRepo,
			wantLine: 2, wantMessage: "resource needs a name"},
		{name: "empty task name", source: resources + getRepo + "  - task:\n    file: repo/t.yml\n",
			wantLine: 9, wantMessage: "task step needs a name"},
		{name: "var source without name", source: resources + "var_sources:\n- type: dummy\n  config: {}\n" + getRepo,
			wantLine: 6, wantMessage: "var source needs a name"},
		{name: "prototype without type", source: resources + "prototypes:\n- name: p\n  source: {}\n" + getRepo,
			wantLine: 6, wantMessage: "prototype needs a type"},
		{name: "run without type", source: resources + getRepo + "  - run: hello\n",
			wantLine: 9, wantMessage: "run step needs a type"},
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
			if finding.Rule != "CL001" || finding.Position.Line != testCase.wantLine || !strings.Contains(finding.Message, testCase.wantMessage) {
				t.Errorf("got  %s line %d %q\nwant CL001 line %d containing %q",
					finding.Rule, finding.Position.Line, finding.Message, testCase.wantLine, testCase.wantMessage)
			}
		})
	}
}
