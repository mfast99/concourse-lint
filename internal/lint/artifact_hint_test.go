package lint_test

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/lint"
)

// TestMissingArtifactHint: the hint of a missing artifact (CL007, CL011,
// CL012) never suggests the artifact of a get or put whose resource is
// undefined (a CL002), and points to that step when the missing name is the
// resource it probably means.
func TestMissingArtifactHint(t *testing.T) {
	const header = `resources:
- name: repo
  type: git
  source: {uri: https://github.com/example/app.git}
- name: image
  type: registry-image
  source: {repository: example/app}
jobs:
- name: build
  plan:
`
	const task = `
    config:
      platform: linux
      image_resource: {type: registry-image, source: {repository: golang, tag: "1.24"}}
      run: {path: make}
`
	tests := []struct {
		name string
		rule string
		plan string
		hint string
	}{
		{
			name: "CL011 input named after the resource of a misspelled get",
			rule: "CL011",
			plan: "  - get: rpeo\n  - task: compile" + task + "      inputs: [{name: repo}]\n",
			hint: `the get "rpeo" in line 11 probably means resource "repo"; fix it there`,
		},
		{
			name: "CL011 input named after the resource of a misspelled put",
			rule: "CL011",
			plan: "  - put: imgae\n  - task: scan" + task + "      inputs: [{name: image}]\n",
			hint: `the put "imgae" in line 11 probably means resource "image"; fix it there`,
		},
		{
			name: "CL007 file: in the artifact of a misspelled get",
			rule: "CL007",
			plan: "  - get: rpeo\n  - task: compile\n    file: repo/ci/compile.yml\n",
			hint: `the get "rpeo" in line 11 probably means resource "repo"; fix it there`,
		},
		{
			name: "CL012 input_mapping to the artifact of a misspelled get",
			rule: "CL012",
			plan: "  - get: rpeo\n  - task: compile\n    input_mapping: {source: repo}" + task + "      inputs: [{name: source}]\n",
			hint: `the get "rpeo" in line 11 probably means resource "repo"; fix it there`,
		},
		{
			name: "the misspelled name is not suggested for another input",
			rule: "CL011",
			plan: "  - get: rpeo\n  - task: compile" + task + "      inputs: [{name: rpo}]\n",
			hint: "fetch it with a get or produce it as a task output in an earlier step",
		},
		{
			name: "an alias of a misspelled resource is still suggested",
			rule: "CL011",
			plan: "  - get: source\n    resource: rpeo\n  - task: compile" + task + "      inputs: [{name: sorce}]\n",
			hint: `did you mean "source"?`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var hints []string
			for _, finding := range lint.Check("pipeline.yml", []byte(header+test.plan)) {
				if finding.Rule == test.rule {
					hints = append(hints, finding.Hint)
				}
			}
			if len(hints) != 1 || hints[0] != test.hint {
				t.Errorf("%s hints = %q, want [%q]", test.rule, hints, test.hint)
			}
		})
	}
}
