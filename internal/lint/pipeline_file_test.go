package lint

import "testing"

func TestLooksLikePipeline(t *testing.T) {
	tests := map[string]bool{
		"jobs:\n- name: a\n  plan:\n  - get: repo\n":             true,
		"jobs:\n- {name: a, plan: [{get: repo}]}\n":              true,
		"resources: []\njobs:\n- name: a\n  plan: []\n":          true,
		"platform: linux\nrun: {path: make}\n":                   false,
		"region: eu-central-1\n":                                 false,
		"name: garden\njobs:\n- name: garden\n  version: abc\n":  false,
		"language: go\njobs:\n  include:\n  - go: 1.24\n":        false,
		"on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n": false,
		"steps:\n- plan: x\n":                                    false,
		// jobs: is not the last top-level key; a comment in the jobs block.
		"jobs:\n# build\n- name: a\n  plan:\n  - get: repo\nresources:\n- name: repo\n": true,
		"jobs: [{name: a, plan: [{get: repo}]}]\n":                                      true,
		// CircleCI: a step named terraform-plan is no plan: key.
		"version: 2.1\njobs:\n  build:\n    steps:\n    - terraform-plan:\n        dir: x\n": false,
		// BOSH deployment manifest: plan: is a property outside the jobs block.
		"instance_groups:\n- name: rabbitmq\n  properties:\n    plan: standalone\njobs:\n- name: rabbitmq\n  release: rabbitmq\n": false,
	}
	for source, want := range tests {
		if got := LooksLikePipeline([]byte(source)); got != want {
			t.Errorf("LooksLikePipeline(%q) = %v, want %v", source, got, want)
		}
	}
}
