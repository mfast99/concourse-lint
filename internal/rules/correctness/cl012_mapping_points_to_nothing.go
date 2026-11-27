package correctness

import (
	"fmt"
	"slices"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/flow"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CL012 reports an input_mapping value that is not an available artifact,
// and an output_mapping key that is no output of the inline task. Cases:
// spec/rules/correctness/CL012/
var CL012 = analysis.Rule{
	ID:       "CL012",
	Severity: analysis.SeverityError,
	Title:    "input_mapping/output_mapping points to nothing",
	Run:      runCL012,
}

func runCL012(pass *analysis.Pass) {
	forEachStepInScope(pass.Pipeline, func(step *model.Step, scope flow.Scope) {
		if step.Task == nil {
			return
		}
		for _, mapping := range step.Task.InputMapping {
			if mapping == nil || !checkable(mapping.PlanName) || scope.HasArtifact(mapping.PlanName.Value) {
				continue
			}
			reportMissingArtifact(pass, mapping.PlanName, scope, "input_mapping artifact")
		}
		if !isInlineTask(step.Task) {
			return
		}
		var outputs []string
		for _, output := range step.Task.Config.Outputs {
			if output != nil && output.Name.Set {
				outputs = append(outputs, output.Name.Value)
			}
		}
		for _, mapping := range step.Task.OutputMapping {
			if mapping == nil || !checkable(mapping.TaskName) || slices.Contains(outputs, mapping.TaskName.Value) {
				continue
			}
			pass.ReportTypo(mapping.TaskName.Position,
				fmt.Sprintf("output_mapping names %q, which is no output of the task", mapping.TaskName.Value),
				mapping.TaskName.Value, outputs)
		}
	})
}
