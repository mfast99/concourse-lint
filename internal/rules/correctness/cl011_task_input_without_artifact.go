package correctness

import (
	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/flow"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CL011 reports an input of an inline task that is not available at this
// point. Optional inputs and inputs provided by input_mapping (CL012) are
// skipped. Cases: spec/rules/correctness/CL011/
var CL011 = analysis.Rule{
	ID:       "CL011",
	Severity: analysis.SeverityError,
	Title:    "Task input without an available artifact",
	Run:      runCL011,
}

func runCL011(pass *analysis.Pass) {
	forEachStepInScope(pass.Pipeline, func(step *model.Step, scope flow.Scope) {
		if step.Task == nil || !isInlineTask(step.Task) {
			return
		}
		mapped := map[string]bool{}
		for _, mapping := range step.Task.InputMapping {
			if mapping != nil && mapping.TaskName.Set {
				mapped[mapping.TaskName.Value] = true
			}
		}
		for _, input := range step.Task.Config.Inputs {
			if input == nil || !checkable(input.Name) || mapped[input.Name.Value] {
				continue
			}
			if input.Optional.Set && input.Optional.Value {
				continue
			}
			if !scope.HasArtifact(input.Name.Value) {
				reportMissingArtifact(pass, input.Name, scope, "input")
			}
		}
	})
}
