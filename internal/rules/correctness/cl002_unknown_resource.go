package correctness

import (
	"fmt"
	"slices"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// CL002 reports get/put (or resource:) and groups[].resources pointing to a
// resource that does not exist. Cases: spec/rules/correctness/CL002/
var CL002 = analysis.Rule{
	ID:       "CL002",
	Severity: analysis.SeverityError,
	Title:    "get/put or a group refers to a missing resource",
	Run:      runCL002,
}

func runCL002(pass *analysis.Pass) {
	resources := navigate.ResourceNames(pass.Pipeline)
	for _, job := range pass.Pipeline.Jobs {
		if job == nil {
			continue
		}
		for step := range job.AllSteps() {
			resource, isResourceStep := navigate.StepResource(step)
			if !isResourceStep || !checkable(resource) || slices.Contains(resources, resource.Value) {
				continue
			}
			pass.ReportTypo(resource.Position,
				fmt.Sprintf("resource %q is not defined", resource.Value),
				resource.Value, resources)
		}
	}
	// Group entries are exact names (no globs, unlike groups[].jobs).
	for _, group := range pass.Pipeline.Groups {
		if group == nil {
			continue
		}
		for _, resource := range group.Resources {
			if !checkable(resource) || slices.Contains(resources, resource.Value) {
				continue
			}
			pass.ReportTypo(resource.Position,
				fmt.Sprintf("group %q lists resource %q, which is not defined", group.Name.Value, resource.Value),
				resource.Value, resources)
		}
	}
}
