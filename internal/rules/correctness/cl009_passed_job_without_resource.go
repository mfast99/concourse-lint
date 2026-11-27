package correctness

import (
	"fmt"
	"slices"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// CL009 reports a job in passed: that does not get or put the resource of
// the step. Concourse rejects this. Cases: spec/rules/correctness/CL009/
var CL009 = analysis.Rule{
	ID:       "CL009",
	Severity: analysis.SeverityError,
	Title:    "Job in passed: does not use the resource",
	Run:      runCL009,
}

func runCL009(pass *analysis.Pass) {
	jobs := jobsByName(pass.Pipeline)
	resources := navigate.ResourceNames(pass.Pipeline)
	for _, job := range pass.Pipeline.Jobs {
		if job == nil {
			continue
		}
		for step := range job.AllSteps() {
			if step.Get == nil {
				continue
			}
			resource, _ := navigate.StepResource(step)
			// An unknown resource is CL002; checking passed: against it adds nothing.
			if !checkable(resource) || !slices.Contains(resources, resource.Value) {
				continue
			}
			for _, passedJobName := range step.Get.Passed {
				passedJob, exists := jobs[passedJobName.Value]
				if !checkable(passedJobName) || !exists || usesResource(passedJob, resource.Value) {
					continue
				}
				pass.Report(passedJobName.Position,
					fmt.Sprintf("job %q in passed: does not use resource %q", passedJobName.Value, resource.Value),
					fmt.Sprintf("add a get or put of %q to job %q, or remove it from passed:", resource.Value, passedJobName.Value))
			}
		}
	}
}
