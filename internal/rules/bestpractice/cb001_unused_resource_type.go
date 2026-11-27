package bestpractice

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// CB001 reports a resource type that nothing refers to: no resource, no
// other resource type and no image_resource. In a chain of unused types only
// the end is reported. Cases: spec/rules/bestpractice/CB001/
var CB001 = analysis.Rule{
	ID:       "CB001",
	Severity: analysis.SeverityInfo,
	Title:    "Resource type is not used",
	Run:      runCB001,
}

func runCB001(pass *analysis.Pass) {
	pipeline := pass.Pipeline
	referenced := map[string]bool{}
	for _, resource := range pipeline.Resources {
		if resource != nil && resource.Type.Set {
			referenced[resource.Type.Value] = true
		}
	}
	for _, resourceType := range pipeline.ResourceTypes {
		// A type that names itself (an override of a built-in on top of the
		// built-in) is no use of itself.
		if resourceType != nil && resourceType.Type.Set && resourceType.Type.Value != resourceType.Name.Value {
			referenced[resourceType.Type.Value] = true
		}
	}
	for _, job := range pipeline.Jobs {
		if job == nil {
			continue
		}
		for step := range job.AllSteps() {
			if step.Task != nil && step.Task.Config != nil && step.Task.Config.ImageResource != nil && step.Task.Config.ImageResource.Type.Set {
				referenced[step.Task.Config.ImageResource.Type.Value] = true
			}
		}
	}
	for _, resourceType := range pipeline.ResourceTypes {
		if resourceType == nil || !resourceType.Name.Set || referenced[resourceType.Name.Value] {
			continue
		}
		pass.Report(resourceType.Name.Position,
			fmt.Sprintf("resource type %q is not used", resourceType.Name.Value),
			"remove it, or use it in a resource")
	}
}
