package navigate

import (
	"strings"

	"github.com/mfast99/concourse-lint/internal/model"
)

// BuiltinResourceTypes are the resource types every Concourse worker
// provides (spec/README.md, "Built-in types").
var BuiltinResourceTypes = []string{
	"bosh-io-release", "bosh-io-stemcell", "cf", "docker-image", "git",
	"github-release", "hg", "mock", "pool", "registry-image", "s3", "semver",
	"time", "tracker",
}

// StepResource returns the resource a get or put step refers to: the value
// of resource: if set, otherwise the step name.
func StepResource(step *model.Step) (resource model.Value[string], isResourceStep bool) {
	switch {
	case step.Get != nil:
		if step.Get.Resource.Set {
			return step.Get.Resource, true
		}
		return step.Get.Name, true
	case step.Put != nil:
		if step.Put.Resource.Set {
			return step.Put.Resource, true
		}
		return step.Put.Name, true
	}
	return model.Value[string]{}, false
}

// ResourceNames returns the names of all resources.
func ResourceNames(pipeline *model.Pipeline) []string {
	var names []string
	for _, resource := range pipeline.Resources {
		if resource != nil && resource.Name.Set {
			names = append(names, resource.Name.Value)
		}
	}
	return names
}

// JobNames returns the names of all jobs.
func JobNames(pipeline *model.Pipeline) []string {
	var names []string
	for _, job := range pipeline.Jobs {
		if job != nil && job.Name.Set {
			names = append(names, job.Name.Value)
		}
	}
	return names
}

// IsVariable reports whether the text contains a ((var)), whose value the
// linter cannot know.
func IsVariable(text string) bool {
	return strings.Contains(text, "((")
}

// Resolvable reports whether a name can be resolved: set and without
// ((var)).
func Resolvable(name model.Value[string]) bool {
	return name.Set && !IsVariable(name.Value)
}
