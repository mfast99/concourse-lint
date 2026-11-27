package correctness

import (
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// jobsByName maps every job name to its job (the first one for duplicates).
func jobsByName(pipeline *model.Pipeline) map[string]*model.Job {
	jobs := map[string]*model.Job{}
	for _, job := range pipeline.Jobs {
		if job == nil || !job.Name.Set {
			continue
		}
		if _, seen := jobs[job.Name.Value]; !seen {
			jobs[job.Name.Value] = job
		}
	}
	return jobs
}

// usesResource reports whether a step of the job gets or puts the resource,
// or may do so in a step without a step type.
func usesResource(job *model.Job, resource string) bool {
	if hasUnknownSteps(job) {
		return true
	}
	for step := range job.AllSteps() {
		if used, isResourceStep := navigate.StepResource(step); isResourceStep && used.Set && used.Value == resource {
			return true
		}
	}
	return false
}

// isVariable reports whether the text contains a ((var)), whose value the
// linter cannot know.
func isVariable(text string) bool {
	return navigate.IsVariable(text)
}

// checkable reports whether a name can be checked: set and without ((var)).
func checkable(name model.Value[string]) bool {
	return navigate.Resolvable(name)
}

// hasUnknownSteps reports whether a job has a step without a step type
// (CL001), whose resources and artifacts are unknown.
func hasUnknownSteps(job *model.Job) bool {
	for step := range job.AllSteps() {
		if step.HasNoKind() {
			return true
		}
	}
	return false
}
