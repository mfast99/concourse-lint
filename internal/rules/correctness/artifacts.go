package correctness

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/flow"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// forEachStepInScope calls visit for every step of every job, together with
// the scope it starts in. Invalid steps (CL001) are skipped.
func forEachStepInScope(pipeline *model.Pipeline, visit func(step *model.Step, scope flow.Scope)) {
	for _, job := range pipeline.Jobs {
		if job == nil {
			continue
		}
		for step, scope := range flow.Steps(job) {
			if step.HasOneKind() {
				visit(step, scope)
			}
		}
	}
}

// reportMissingArtifact reports an artifact name that does not exist at this
// point, with a hint to the closest available one.
func reportMissingArtifact(pass *analysis.Pass, name model.Value[string], scope flow.Scope, usage string) {
	pass.Report(name.Position,
		fmt.Sprintf("%s %q is not available at this point", usage, name.Value),
		missingArtifactHint(pass, name.Value, scope))
}

// missingArtifactHint suggests the closest available artifact. The artifact
// of a get or put with an undefined resource (a CL002) is never suggested,
// because its name is the typo; when the missing name is the resource that
// step probably means, the hint points to the step instead.
func missingArtifactHint(pass *analysis.Pass, name string, scope flow.Scope) string {
	resources := navigate.ResourceNames(pass.Pipeline)
	var candidates []string
	for _, artifactName := range scope.ArtifactNames() {
		artifact := scope.Artifacts[artifactName]
		if !fetchesUndefinedResource(artifactName, artifact, resources) {
			candidates = append(candidates, artifactName)
			continue
		}
		if intended, found := analysis.ClosestName(artifactName, resources); found && intended == name {
			return fmt.Sprintf("the %s %q in line %d probably means resource %q; fix it there",
				artifact.Kind, artifactName, artifact.Position.Line, intended)
		}
	}
	if hint := analysis.DidYouMean(name, candidates); hint != "" {
		return hint
	}
	return "fetch it with a get or produce it as a task output in an earlier step"
}

// fetchesUndefinedResource reports whether the artifact comes from a get or
// put without resource: whose name is no defined resource. With resource:
// the artifact name was chosen on purpose and stays a valid suggestion.
func fetchesUndefinedResource(artifactName string, artifact flow.Artifact, resources []string) bool {
	return artifact.Resource.Set && artifact.Resource.Value == artifactName &&
		!slices.Contains(resources, artifactName)
}

// inputPlanNames returns the plan names of the inline task's inputs (after
// input_mapping).
func inputPlanNames(task *model.TaskStep) map[string]bool {
	mapped := map[string]string{}
	for _, mapping := range task.InputMapping {
		if mapping != nil && mapping.TaskName.Set && mapping.PlanName.Set {
			mapped[mapping.TaskName.Value] = mapping.PlanName.Value
		}
	}
	names := map[string]bool{}
	if task.Config == nil {
		return names
	}
	for _, input := range task.Config.Inputs {
		if input == nil || !input.Name.Set {
			continue
		}
		if planName, isMapped := mapped[input.Name.Value]; isMapped {
			names[planName] = true
		} else {
			names[input.Name.Value] = true
		}
	}
	return names
}

// isInlineTask reports whether the task's inputs and outputs are known: an
// inline config without file:.
func isInlineTask(task *model.TaskStep) bool {
	return task.Config != nil && !task.File.Set
}

// firstPathSegment returns the artifact name at the start of a file: path.
func firstPathSegment(path string) string {
	segment, _, _ := strings.Cut(path, "/")
	return segment
}
