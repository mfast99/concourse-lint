// Package flow knows what exists when a step of a job starts: the artifacts
// (from get, put, task outputs) and the local variables (from load_var and
// across). The rules CL007, CL008, CL011, CL012 and CL014 use it.
//
// "Exists" means "can exist on the success path": a step only runs if
// everything before it succeeded,
// so artifacts of on_failure, on_error and on_abort count only inside try
// and for the job hooks. Rules report only what is certainly missing.
package flow

import (
	"iter"
	"maps"
	"slices"
	"strings"

	"github.com/mfast99/concourse-lint/internal/model"
)

// Artifact is an artifact that exists when a step starts.
type Artifact struct {
	// Position of the step name that produces it.
	Position model.Position
	// Kind of the step that produces it: "get", "put" or "task".
	Kind string
	// Resource the get or put fetched (its name when resource: is
	// missing); not set for a task output.
	Resource model.Value[string]
}

// Scope is what exists when a step starts.
type Scope struct {
	// Artifact name -> where it comes from.
	Artifacts map[string]Artifact
	// Local variable name -> position of the load_var, across variable or
	// get that defines it.
	LocalVars map[string]model.Position
	// Open: a task from file:, a name with a ((var)) or a step without a
	// step type ran before on this path, so there may be artifacts nobody
	// can name.
	Open bool
}

// HasArtifact reports whether an artifact with this name can exist. In an
// open scope every name can.
func (scope Scope) HasArtifact(name string) bool {
	return scope.Open || scope.KnowsArtifact(name)
}

// KnowsArtifact reports whether a step that produces this artifact ran
// before, ignoring an open scope.
func (scope Scope) KnowsArtifact(name string) bool {
	_, known := scope.Artifacts[name]
	return known
}

// ArtifactNames returns the known artifact names, sorted (for hints).
func (scope Scope) ArtifactNames() []string {
	return slices.Sorted(maps.Keys(scope.Artifacts))
}

// HasLocalVar reports whether a local variable with this name exists.
func (scope Scope) HasLocalVar(name string) bool {
	_, known := scope.LocalVars[name]
	return known
}

// LocalVarNames returns the known local variable names, sorted (for hints).
func (scope Scope) LocalVarNames() []string {
	return slices.Sorted(maps.Keys(scope.LocalVars))
}

// Steps yields every step of the job with the scope it starts in, in the
// order of model.Job.AllSteps: a step, its nested steps, its hooks; the job
// hooks after the plan.
func Steps(job *model.Job) iter.Seq2[*model.Step, Scope] {
	return func(yield func(*model.Step, Scope) bool) {
		walker := &walker{yield: yield}
		walker.job(job)
	}
}

// walker runs through a job and hands every step with its scope to yield.
type walker struct {
	yield func(*model.Step, Scope) bool
	// stopped is set once yield returned false (the caller left the loop).
	stopped bool
}

// outcome is what exists after a step: on the success path, and on any
// path (success or failure), which is what try and failure hooks see.
type outcome struct {
	success Scope
	any     Scope
}

func (walker *walker) job(job *model.Job) {
	result := walker.sequence(job.Plan, emptyScope())
	walker.hooks(&job.Hooks, result)
}

// sequence runs steps one after another, each in the success scope of the
// one before.
func (walker *walker) sequence(steps []*model.Step, start Scope) outcome {
	current := outcome{success: start, any: start}
	for _, step := range steps {
		next := walker.step(step, current.success)
		current = outcome{success: next.success, any: union(current.any, next.any)}
	}
	return current
}

// step runs one step with its hooks. Concourse wraps a step as
// ensure(on_error(on_abort(on_failure(on_success(across(step)))))), so the
// hooks are outside across.
func (walker *walker) step(step *model.Step, start Scope) outcome {
	if step == nil {
		return outcome{success: start, any: start}
	}
	var result outcome
	if step.Across != nil {
		// Every iteration runs in its own scope; nothing leaks out.
		inner := start.clone()
		for _, variable := range step.Across.Variables {
			if variable != nil && variable.Name.Set {
				inner.LocalVars[variable.Name.Value] = variable.Name.Position
			}
		}
		walker.kind(step, inner)
		result = outcome{success: start, any: start}
	} else {
		result = walker.kind(step, start)
	}
	return walker.hooks(&step.Hooks, result)
}

// hooks runs the hooks around a finished step (or job). on_success runs on
// the success path; on_failure, on_error and on_abort only after a failure,
// so what they produce never reaches the next step; ensure runs on both.
func (walker *walker) hooks(hooks *model.Hooks, result outcome) outcome {
	if hooks.OnSuccess != nil {
		hook := walker.step(hooks.OnSuccess, result.success)
		result = outcome{success: hook.success, any: union(result.any, hook.any)}
	}
	for _, failureHook := range []*model.Step{hooks.OnFailure, hooks.OnAbort, hooks.OnError} {
		if failureHook != nil {
			hook := walker.step(failureHook, result.any)
			result.any = union(result.any, hook.any)
		}
	}
	if hooks.Ensure != nil {
		hook := walker.step(hooks.Ensure, result.any)
		result = outcome{
			success: union(result.success, produced(hook.success, result.any)),
			any:     union(result.any, hook.any),
		}
	}
	return result
}

// kind visits the step itself and runs what is inside it.
func (walker *walker) kind(step *model.Step, start Scope) outcome {
	walker.visit(step, start)
	after := start.clone()
	if step.HasNoKind() {
		// Unknown content: it may produce any artifact.
		after.Open = true
	}
	switch {
	case step.Get != nil:
		after.addFetched("get", step.Get.Name, step.Get.Resource)
	case step.Put != nil:
		if !step.Put.NoGet.Set || !step.Put.NoGet.Value {
			after.addFetched("put", step.Put.Name, step.Put.Resource)
		}
	case step.Task != nil:
		addTaskOutputs(&after, step.Task)
	case step.LoadVar != nil:
		if step.LoadVar.Name.Set {
			after.LocalVars[step.LoadVar.Name.Value] = step.LoadVar.Name.Position
		}
	case step.Do != nil:
		return walker.sequence(step.Do.Steps, start)
	case step.InParallel != nil:
		if step.InParallel.Limit.Set && step.InParallel.Limit.Value == 1 {
			return walker.sequence(step.InParallel.Steps, start)
		}
		return walker.parallel(step.InParallel.Steps, start)
	case step.Try != nil:
		inner := walker.step(step.Try.Step, start)
		// try turns a failure into success, so everything that can exist
		// afterwards is on the success path.
		return outcome{success: inner.any, any: inner.any}
	}
	return outcome{success: after, any: after}
}

// parallel runs steps at the same time: each sees only what existed before.
func (walker *walker) parallel(steps []*model.Step, start Scope) outcome {
	result := outcome{success: start, any: start}
	for _, step := range steps {
		branch := walker.step(step, start)
		result = outcome{success: union(result.success, branch.success), any: union(result.any, branch.any)}
	}
	return result
}

// addTaskOutputs adds the outputs of a task, renamed by output_mapping. The
// outputs of a task from file: are unknown, so the scope becomes open.
func addTaskOutputs(scope *Scope, task *model.TaskStep) {
	renamed := map[string]model.Value[string]{}
	for _, mapping := range task.OutputMapping {
		if mapping != nil && mapping.TaskName.Set {
			renamed[mapping.TaskName.Value] = mapping.PlanName
		}
	}
	if task.Config == nil || task.File.Set {
		scope.Open = true
		for _, planName := range renamed {
			scope.addArtifact(Artifact{Kind: "task"}, planName)
		}
		return
	}
	for _, output := range task.Config.Outputs {
		if output == nil || !output.Name.Set {
			continue
		}
		if planName, isRenamed := renamed[output.Name.Value]; isRenamed {
			scope.addArtifact(Artifact{Kind: "task"}, planName)
		} else {
			scope.addArtifact(Artifact{Kind: "task"}, output.Name)
		}
	}
}

func (walker *walker) visit(step *model.Step, scope Scope) {
	if walker.stopped {
		return
	}
	if !walker.yield(step, scope) {
		walker.stopped = true
	}
}

func emptyScope() Scope {
	return Scope{Artifacts: map[string]Artifact{}, LocalVars: map[string]model.Position{}}
}

// clone copies the scope, so that adding to the copy leaves the original
// alone (maps are shared when a struct is copied).
func (scope Scope) clone() Scope {
	return Scope{Artifacts: maps.Clone(scope.Artifacts), LocalVars: maps.Clone(scope.LocalVars), Open: scope.Open}
}

// addArtifact adds an artifact name with its origin (the position is taken
// from the name). A name with a ((var)) cannot be known, so it opens the
// scope instead.
func (scope *Scope) addArtifact(origin Artifact, name model.Value[string]) {
	if !name.Set {
		return
	}
	if strings.Contains(name.Value, "((") {
		scope.Open = true
		return
	}
	if _, known := scope.Artifacts[name.Value]; !known {
		origin.Position = name.Position
		scope.Artifacts[name.Value] = origin
	}
}

// addFetched adds what a get (or the implicit get of a put) leaves behind:
// the artifact and a local variable with the same name, which holds the
// metadata of the fetched version (exec/get_step.go in Concourse 8.3.4).
// Without resource: the step fetches the resource of its name.
func (scope *Scope) addFetched(kind string, name, resource model.Value[string]) {
	if !resource.Set {
		resource = name
	}
	scope.addArtifact(Artifact{Kind: kind, Resource: resource}, name)
	if name.Set && !strings.Contains(name.Value, "((") {
		if _, known := scope.LocalVars[name.Value]; !known {
			scope.LocalVars[name.Value] = name.Position
		}
	}
}

// union returns what exists in either scope.
func union(first, second Scope) Scope {
	result := first.clone()
	for name, artifact := range second.Artifacts {
		if _, known := result.Artifacts[name]; !known {
			result.Artifacts[name] = artifact
		}
	}
	for name, position := range second.LocalVars {
		if _, known := result.LocalVars[name]; !known {
			result.LocalVars[name] = position
		}
	}
	result.Open = first.Open || second.Open
	return result
}

// produced returns what after has and before does not: what a step added.
func produced(after, before Scope) Scope {
	result := emptyScope()
	for name, artifact := range after.Artifacts {
		if _, existed := before.Artifacts[name]; !existed {
			result.Artifacts[name] = artifact
		}
	}
	for name, position := range after.LocalVars {
		if _, existed := before.LocalVars[name]; !existed {
			result.LocalVars[name] = position
		}
	}
	result.Open = after.Open && !before.Open
	return result
}
