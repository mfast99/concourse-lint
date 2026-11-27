package correctness

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// CL006 reports a resource that no get or put uses. Concourse rejects the
// pipeline then. Cases: spec/rules/correctness/CL006/
var CL006 = analysis.Rule{
	ID:       "CL006",
	Severity: analysis.SeverityError,
	Title:    "Resource is never used",
	Run:      runCL006,
}

func runCL006(pass *analysis.Pass) {
	used := map[string]bool{}
	for _, job := range pass.Pipeline.Jobs {
		if job == nil {
			continue
		}
		// A step without a step type (CL001) may use any resource.
		if hasUnknownSteps(job) {
			return
		}
		for step := range job.AllSteps() {
			if resource, isResourceStep := navigate.StepResource(step); isResourceStep && resource.Set {
				used[resource.Value] = true
			}
		}
	}
	for _, resource := range pass.Pipeline.Resources {
		if resource == nil || !checkable(resource.Name) || used[resource.Name.Value] {
			continue
		}
		pass.Report(resource.Name.Position,
			fmt.Sprintf("resource %q is never used by a get or put", resource.Name.Value),
			"use it in a job or remove it")
	}
}
