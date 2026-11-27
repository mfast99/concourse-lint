package flow_test

import (
	"slices"
	"testing"

	"github.com/mfast99/concourse-lint/internal/flow"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestSteps: the scope at the step named "probe" (a task) in the first job.
func TestSteps(t *testing.T) {
	tests := []struct {
		name      string
		plan      string
		artifacts []string
		// localVars are the local variables that are not also artifacts (a
		// get defines both; TestGetDefinesLocalVar checks that).
		localVars []string
		open      bool
	}{
		{
			name: "get, put and put with no_get",
			plan: `
  - get: source
    resource: repo
  - put: repo
  - put: other
    no_get: true
  - task: probe
    file: source/probe.yml`,
			artifacts: []string{"repo", "source"},
		},
		{
			name: "later steps are not available",
			plan: `
  - task: probe
    file: x/probe.yml
  - get: repo`,
		},
		{
			name: "in_parallel siblings do not see each other",
			plan: `
  - in_parallel:
    - get: repo
    - task: probe
      file: repo/probe.yml`,
		},
		{
			name: "in_parallel with limit 1 runs in order",
			plan: `
  - in_parallel:
      limit: 1
      steps:
      - get: repo
      - task: probe
        file: repo/probe.yml`,
			artifacts: []string{"repo"},
		},
		{
			name: "after in_parallel and do everything is available",
			plan: `
  - in_parallel:
    - get: repo
    - do:
      - get: other
  - task: probe
    file: repo/probe.yml`,
			artifacts: []string{"other", "repo"},
		},
		{
			name: "inline task outputs renamed by output_mapping",
			plan: `
  - task: build
    output_mapping: {binary: compiled}
    config:
      platform: linux
      outputs: [{name: binary}, {name: logs}]
      run: {path: build}
  - task: probe
    file: compiled/probe.yml`,
			artifacts: []string{"compiled", "logs"},
		},
		{
			name: "a task from file: opens the scope",
			plan: `
  - get: repo
  - task: build
    file: repo/build.yml
    output_mapping: {out: renamed}
  - task: probe
    file: repo/probe.yml`,
			artifacts: []string{"renamed", "repo"},
			open:      true,
		},
		{
			name: "across: variables inside, nothing leaks out",
			plan: `
  - get: repo
    across:
    - var: region
      values: [a]
  - task: probe
    file: x/probe.yml`,
		},
		{
			name: "across on do: nested steps see the variable and each other",
			plan: `
  - do:
    - get: repo
    - task: probe
      file: repo/probe.yml
    across:
    - var: region
      values: [a]`,
			artifacts: []string{"repo"},
			localVars: []string{"region"},
		},
		{
			name: "on_success output reaches the next step, on_failure output does not",
			plan: `
  - get: repo
    on_success:
      get: good
    on_failure:
      get: bad
  - task: probe
    file: repo/probe.yml`,
			artifacts: []string{"good", "repo"},
		},
		{
			name: "inside try the failure hook output is available afterwards",
			plan: `
  - try:
      get: repo
      on_failure:
        get: bad
  - task: probe
    file: repo/probe.yml`,
			artifacts: []string{"bad", "repo"},
		},
		{
			name: "ensure output reaches the next step",
			plan: `
  - get: repo
    ensure:
      get: cleanup
  - task: probe
    file: repo/probe.yml`,
			artifacts: []string{"cleanup", "repo"},
		},
		{
			name: "load_var adds a local variable",
			plan: `
  - get: repo
  - load_var: version
    file: repo/version
  - task: probe
    file: repo/probe.yml`,
			artifacts: []string{"repo"},
			localVars: []string{"version"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pipeline, _ := parse.Parse([]byte("jobs:\n- name: job\n  plan:" + test.plan + "\n"))
			if len(pipeline.Jobs) == 0 {
				t.Fatal("no job parsed")
			}
			scope, found := scopeOfProbe(pipeline.Jobs[0])
			if !found {
				t.Fatal("step probe not visited")
			}
			if got := scope.ArtifactNames(); !slices.Equal(got, test.artifacts) && !(len(got) == 0 && len(test.artifacts) == 0) {
				t.Errorf("artifacts = %v, want %v", got, test.artifacts)
			}
			var localVars []string
			for _, name := range scope.LocalVarNames() {
				if !scope.KnowsArtifact(name) {
					localVars = append(localVars, name)
				}
			}
			if got := localVars; !slices.Equal(got, test.localVars) && !(len(got) == 0 && len(test.localVars) == 0) {
				t.Errorf("local vars = %v, want %v", got, test.localVars)
			}
			if scope.Open != test.open {
				t.Errorf("open = %v, want %v", scope.Open, test.open)
			}
		})
	}
}

// TestGetDefinesLocalVar: a get and the implicit get of a put set a local
// variable with their name (the version metadata); a put with no_get does not.
func TestGetDefinesLocalVar(t *testing.T) {
	pipeline, _ := parse.Parse([]byte(`jobs:
- name: job
  plan:
  - get: source
    resource: repo
  - put: repo
  - put: other
    no_get: true
  - task: probe
    file: source/probe.yml
`))
	scope, found := scopeOfProbe(pipeline.Jobs[0])
	if !found {
		t.Fatal("step probe not visited")
	}
	if got := scope.LocalVarNames(); !slices.Equal(got, []string{"repo", "source"}) {
		t.Errorf("local vars = %v, want [repo source]", got)
	}
}

// TestJobHooks: the job hooks see everything the plan can have produced.
func TestJobHooks(t *testing.T) {
	pipeline, _ := parse.Parse([]byte(`jobs:
- name: job
  plan:
  - get: repo
    on_failure:
      get: bad
  on_failure:
    task: probe
    file: bad/probe.yml
`))
	scope, found := scopeOfProbe(pipeline.Jobs[0])
	if !found {
		t.Fatal("step probe not visited")
	}
	if !scope.KnowsArtifact("bad") || !scope.KnowsArtifact("repo") {
		t.Errorf("artifacts = %v, want bad and repo", scope.ArtifactNames())
	}
}

// TestStepsVisitsLikeAllSteps: every step is visited once, in the order of
// model.Job.AllSteps.
func TestStepsVisitsLikeAllSteps(t *testing.T) {
	pipeline, _ := parse.Parse([]byte(`jobs:
- name: job
  plan:
  - in_parallel:
    - get: a
    - do:
      - get: b
      - try:
          get: c
  - get: d
    across:
    - var: v
      values: [x]
    on_success:
      get: e
    ensure:
      get: f
  on_failure:
    get: g
`))
	job := pipeline.Jobs[0]
	var expected, visited []*model.Step
	for step := range job.AllSteps() {
		expected = append(expected, step)
	}
	for step := range flow.Steps(job) {
		visited = append(visited, step)
	}
	if !slices.Equal(expected, visited) {
		t.Errorf("visited %d steps, want the %d of AllSteps in the same order", len(visited), len(expected))
	}
}

// TestArtifactOrigin: an artifact remembers where it comes from: the kind of
// step, and for a get or put the resource it fetched (its name when
// resource: is missing).
func TestArtifactOrigin(t *testing.T) {
	pipeline, _ := parse.Parse([]byte(`jobs:
- name: job
  plan:
  - get: source
    resource: repo
  - get: tools
  - put: image
  - task: build
    config:
      platform: linux
      outputs: [{name: binary}]
      run: {path: build}
  - task: probe
    file: source/probe.yml
`))
	scope, found := scopeOfProbe(pipeline.Jobs[0])
	if !found {
		t.Fatal("step probe not visited")
	}
	tests := []struct {
		artifact string
		kind     string
		resource string
		line     int
	}{
		{artifact: "source", kind: "get", resource: "repo", line: 4},
		{artifact: "tools", kind: "get", resource: "tools", line: 6},
		{artifact: "image", kind: "put", resource: "image", line: 7},
		{artifact: "binary", kind: "task", resource: "", line: 11},
	}
	for _, test := range tests {
		artifact := scope.Artifacts[test.artifact]
		if artifact.Resource.Value != test.resource || artifact.Resource.Set != (test.resource != "") {
			t.Errorf("%s: resource = %+v, want %q", test.artifact, artifact.Resource, test.resource)
		}
		if artifact.Kind != test.kind {
			t.Errorf("%s: kind = %q, want %q", test.artifact, artifact.Kind, test.kind)
		}
		if artifact.Position.Line != test.line {
			t.Errorf("%s: line = %d, want %d", test.artifact, artifact.Position.Line, test.line)
		}
	}
}

func scopeOfProbe(job *model.Job) (flow.Scope, bool) {
	for step, scope := range flow.Steps(job) {
		if step.Task != nil && step.Task.Name.Value == "probe" {
			return scope, true
		}
	}
	return flow.Scope{}, false
}
