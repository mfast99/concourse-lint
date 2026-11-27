package model_test

import (
	"slices"
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
)

// TestAllSteps: AllSteps visits every step of a job exactly once, depth
// first: a step, then the steps nested in it, then its hooks; the job hooks
// come after the plan.
func TestAllSteps(t *testing.T) {
	named := func(name string) *model.Step {
		return &model.Step{Task: &model.TaskStep{Name: model.Value[string]{Value: name, Set: true}}}
	}
	parallelFirst, parallelSecond := named("parallel-first"), named("parallel-second")
	parallel := &model.Step{InParallel: &model.InParallelStep{Steps: []*model.Step{parallelFirst, parallelSecond}}}
	tried := named("tried")
	try := &model.Step{Try: &model.TryStep{Step: tried}}
	doneFirst := named("done-first")
	do := &model.Step{Do: &model.DoStep{Steps: []*model.Step{doneFirst}}}
	stepSuccess, stepEnsure := named("step-success"), named("step-ensure")
	withHooks := named("with-hooks")
	withHooks.OnSuccess, withHooks.Ensure = stepSuccess, stepEnsure
	jobFailure, jobEnsure := named("job-failure"), named("job-ensure")

	job := &model.Job{Plan: []*model.Step{parallel, try, do, withHooks}}
	job.OnFailure, job.Ensure = jobFailure, jobEnsure

	want := []*model.Step{
		parallel, parallelFirst, parallelSecond,
		try, tried,
		do, doneFirst,
		withHooks, stepSuccess, stepEnsure,
		jobFailure, jobEnsure,
	}
	if got := slices.Collect(job.AllSteps()); !slices.Equal(got, want) {
		t.Errorf("got %d steps in a different order than the %d expected", len(got), len(want))
	}

	// Stopping early (break in a for loop) must not visit further steps.
	visited := 0
	for range job.AllSteps() {
		visited++
		if visited == 2 {
			break
		}
	}
	if visited != 2 {
		t.Errorf("visited %d steps after break, want 2", visited)
	}
}
