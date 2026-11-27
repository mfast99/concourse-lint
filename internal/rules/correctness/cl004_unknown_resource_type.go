package correctness

import (
	"fmt"
	"slices"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// CL004 reports a type that is neither built in nor defined under
// resource_types: (in resources, resource types and image_resource). Cases:
// spec/rules/correctness/CL004/
var CL004 = analysis.Rule{
	ID:       "CL004",
	Severity: analysis.SeverityError,
	Title:    "Resource type is not defined",
	Run:      runCL004,
}

func runCL004(pass *analysis.Pass) {
	knownTypes := slices.Concat(navigate.BuiltinResourceTypes, pass.Settings.ExtraResourceTypes)
	for _, resourceType := range pass.Pipeline.ResourceTypes {
		if resourceType != nil && resourceType.Name.Set {
			knownTypes = append(knownTypes, resourceType.Name.Value)
		}
	}
	check := func(typeName model.Value[string]) {
		if !checkable(typeName) || slices.Contains(knownTypes, typeName.Value) {
			return
		}
		pass.ReportTypo(typeName.Position,
			fmt.Sprintf("resource type %q is neither built in nor defined under resource_types:", typeName.Value),
			typeName.Value, knownTypes)
	}
	for _, resource := range pass.Pipeline.Resources {
		if resource != nil {
			check(resource.Type)
		}
	}
	for _, resourceType := range pass.Pipeline.ResourceTypes {
		if resourceType != nil {
			check(resourceType.Type)
		}
	}
	for _, job := range pass.Pipeline.Jobs {
		if job == nil {
			continue
		}
		for step := range job.AllSteps() {
			if step.Task != nil && step.Task.Config != nil && step.Task.Config.ImageResource != nil {
				check(step.Task.Config.ImageResource.Type)
			}
		}
	}
}
