package correctness

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/flow"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CL014 reports an output of an inline task (or its output_mapping target)
// with the name of an available artifact that is not also an input of the
// task. Input and output with the same name is "modify in place" and fine.
// Cases: spec/rules/correctness/CL014/
var CL014 = analysis.Rule{
	ID:       "CL014",
	Severity: analysis.SeverityWarning,
	Title:    "Task output shadows an artifact",
	Run:      runCL014,
}

func runCL014(pass *analysis.Pass) {
	forEachStepInScope(pass.Pipeline, func(step *model.Step, scope flow.Scope) {
		if step.Task == nil || !isInlineTask(step.Task) {
			return
		}
		renamed := map[string]model.Value[string]{}
		for _, mapping := range step.Task.OutputMapping {
			if mapping != nil && mapping.TaskName.Set && mapping.PlanName.Set {
				renamed[mapping.TaskName.Value] = mapping.PlanName
			}
		}
		inputs := inputPlanNames(step.Task)
		for _, output := range step.Task.Config.Outputs {
			if output == nil || !checkable(output.Name) {
				continue
			}
			planName := output.Name
			if mapped, isRenamed := renamed[output.Name.Value]; isRenamed {
				planName = mapped
			}
			if !checkable(planName) || inputs[planName.Value] || !scope.KnowsArtifact(planName.Value) {
				continue
			}
			pass.Report(planName.Position,
				fmt.Sprintf("output %q replaces the artifact %q from line %d, which is no input of this task", planName.Value, planName.Value, scope.Artifacts[planName.Value].Position.Line),
				"rename the output, or add it as an input to modify it in place")
		}
	})
}
