package model

import "iter"

// AllSteps visits every step of the job exactly once, depth first: a step,
// then the steps nested in it (in_parallel, do, try), then its hooks. The job
// hooks come after the plan. Use it like a list:
//
//	for step := range job.AllSteps() { ... }
//
// It only visits; which artifacts are available where is a question for the
// rules.
func (job *Job) AllSteps() iter.Seq[*Step] {
	return func(yield func(*Step) bool) {
		for _, step := range job.Plan {
			if !visit(step, yield) {
				return
			}
		}
		for _, hook := range job.Hooks.steps() {
			if !visit(hook, yield) {
				return
			}
		}
	}
}

// Children returns the steps directly inside this step: nested steps first,
// then the hooks.
func (step *Step) Children() []*Step {
	var children []*Step
	if step.InParallel != nil {
		children = append(children, step.InParallel.Steps...)
	}
	if step.Do != nil {
		children = append(children, step.Do.Steps...)
	}
	if step.Try != nil && step.Try.Step != nil {
		children = append(children, step.Try.Step)
	}
	return append(children, step.Hooks.steps()...)
}

// steps returns the hooks that are set, in a fixed order.
func (hooks *Hooks) steps() []*Step {
	var steps []*Step
	for _, hook := range []*Step{hooks.OnSuccess, hooks.OnFailure, hooks.OnAbort, hooks.OnError, hooks.Ensure} {
		if hook != nil {
			steps = append(steps, hook)
		}
	}
	return steps
}

// visit hands the step and everything inside it to yield. It returns false
// as soon as yield does, which means the caller stopped the loop (break).
func visit(step *Step, yield func(*Step) bool) bool {
	if step == nil {
		return true
	}
	if !yield(step) {
		return false
	}
	for _, child := range step.Children() {
		if !visit(child, yield) {
			return false
		}
	}
	return true
}
