package navigate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mfast99/concourse-lint/internal/model"
)

// Summary describes the name at a position for a hover: a title like
// "resource repo" and details like "type: git". Nothing from source: or
// params: goes in, because those may hold secrets.
type Summary struct {
	// The name the summary is about.
	Position model.Position
	// E.g. "resource" and "repo".
	Kind  string
	Name  string
	Facts []string
}

// String is the one-line form the cases in spec/editor/hover/ use:
// "resource repo | type: git | used by: build (get)".
func (summary Summary) String() string {
	return strings.Join(append([]string{summary.Kind + " " + summary.Name}, summary.Facts...), " | ")
}

// Hover returns the summary of the resource, job, resource type or glob at
// the cursor. A name that is defined nowhere (and is no built-in resource
// type) has none.
func Hover(pipeline *model.Pipeline, cursor model.Position) (Summary, bool) {
	for _, pattern := range GroupPatterns(pipeline) {
		if IsGlob(pattern.Entry.Value) && contains(pattern.Entry.Position, cursor) {
			return globSummary(pipeline, pattern), true
		}
	}
	for _, reference := range Index(pipeline) {
		if !contains(reference.Position, cursor) {
			continue
		}
		switch reference.Kind {
		case KindResource:
			return resourceSummary(pipeline, reference)
		case KindJob:
			return jobSummary(pipeline, reference)
		case KindResourceType:
			return resourceTypeSummary(pipeline, reference)
		}
	}
	return Summary{}, false
}

func globSummary(pipeline *model.Pipeline, pattern GroupPattern) Summary {
	var matched []string
	for _, job := range JobNames(pipeline) {
		if pattern.Matches(job) {
			matched = append(matched, job)
		}
	}
	return Summary{Position: pattern.Entry.Position, Kind: "glob", Name: pattern.Entry.Value,
		Facts: []string{"matches: " + listOrNone(matched, "no job")}}
}

func resourceSummary(pipeline *model.Pipeline, reference Reference) (Summary, bool) {
	index := slices.IndexFunc(pipeline.Resources, func(resource *model.Resource) bool {
		return resource != nil && resource.Name.Set && resource.Name.Value == reference.Name
	})
	if index < 0 {
		return Summary{}, false
	}
	resource := pipeline.Resources[index]
	summary := Summary{Position: reference.Position, Kind: "resource", Name: reference.Name}
	summary.addFact("type", resource.Type)
	summary.addFact("check_every", resource.CheckEvery)
	var users []string
	for _, job := range pipeline.Jobs {
		if job == nil || !job.Name.Set {
			continue
		}
		if kinds := resourceStepKinds(job, reference.Name); len(kinds) > 0 {
			users = append(users, fmt.Sprintf("%s (%s)", job.Name.Value, strings.Join(kinds, ", ")))
		}
	}
	summary.Facts = append(summary.Facts, "used by: "+listOrNone(users, "no job"))
	return summary, true
}

// resourceStepKinds returns "get" and/or "put", in the order the job first
// uses the resource that way.
func resourceStepKinds(job *model.Job, resourceName string) []string {
	var kinds []string
	for step := range job.AllSteps() {
		resource, isResourceStep := StepResource(step)
		if !isResourceStep || !resource.Set || resource.Value != resourceName {
			continue
		}
		kind := "get"
		if step.Put != nil {
			kind = "put"
		}
		if !slices.Contains(kinds, kind) {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

func jobSummary(pipeline *model.Pipeline, reference Reference) (Summary, bool) {
	index := slices.IndexFunc(pipeline.Jobs, func(job *model.Job) bool {
		return job != nil && job.Name.Set && job.Name.Value == reference.Name
	})
	if index < 0 {
		return Summary{}, false
	}
	job := pipeline.Jobs[index]
	summary := Summary{Position: reference.Position, Kind: "job", Name: reference.Name}
	stepCount := 0
	var after []string
	for step := range job.AllSteps() {
		stepCount++
		if step.Get == nil {
			continue
		}
		for _, passed := range step.Get.Passed {
			if Resolvable(passed) && !slices.Contains(after, passed.Value) {
				after = append(after, passed.Value)
			}
		}
	}
	if stepCount == 1 {
		summary.Facts = append(summary.Facts, "1 step")
	} else {
		summary.Facts = append(summary.Facts, fmt.Sprintf("%d steps", stepCount))
	}
	var groups []string
	for _, pattern := range GroupPatterns(pipeline) {
		if pattern.Matches(reference.Name) && !slices.Contains(groups, pattern.Group) {
			groups = append(groups, pattern.Group)
		}
	}
	if len(groups) > 0 {
		summary.Facts = append(summary.Facts, "groups: "+strings.Join(groups, ", "))
	}
	if len(after) > 0 {
		summary.Facts = append(summary.Facts, "after: "+strings.Join(after, ", ")+" (passed:)")
	}
	return summary, true
}

func resourceTypeSummary(pipeline *model.Pipeline, reference Reference) (Summary, bool) {
	summary := Summary{Position: reference.Position, Kind: "resource type", Name: reference.Name}
	index := slices.IndexFunc(pipeline.ResourceTypes, func(resourceType *model.ResourceType) bool {
		return resourceType != nil && resourceType.Name.Set && resourceType.Name.Value == reference.Name
	})
	switch {
	case index >= 0:
		summary.addFact("type", pipeline.ResourceTypes[index].Type)
	case slices.Contains(BuiltinResourceTypes, reference.Name):
		summary.Facts = append(summary.Facts, "built-in")
	default:
		return Summary{}, false
	}
	summary.Facts = append(summary.Facts, "used by: "+listOrNone(resourceTypeUsers(pipeline, reference.Name), "nothing"))
	return summary, true
}

// resourceTypeUsers returns the resource types, resources and jobs (through
// an image_resource) that use the type, in the order of the file.
func resourceTypeUsers(pipeline *model.Pipeline, typeName string) []string {
	type user struct {
		position model.Position
		label    string
	}
	var users []user
	for _, resourceType := range pipeline.ResourceTypes {
		if resourceType != nil && resourceType.Type.Set && resourceType.Type.Value == typeName && resourceType.Name.Set {
			users = append(users, user{resourceType.Type.Position, resourceType.Name.Value})
		}
	}
	for _, resource := range pipeline.Resources {
		if resource != nil && resource.Type.Set && resource.Type.Value == typeName && resource.Name.Set {
			users = append(users, user{resource.Type.Position, resource.Name.Value})
		}
	}
	for _, job := range pipeline.Jobs {
		if job == nil || !job.Name.Set {
			continue
		}
		for step := range job.AllSteps() {
			if step.Task == nil || step.Task.Config == nil || step.Task.Config.ImageResource == nil {
				continue
			}
			imageType := step.Task.Config.ImageResource.Type
			if imageType.Set && imageType.Value == typeName {
				users = append(users, user{imageType.Position, job.Name.Value + " (image_resource)"})
				break
			}
		}
	}
	slices.SortStableFunc(users, func(first, second user) int {
		return comparePositions(first.position, second.position)
	})
	var labels []string
	for _, entry := range users {
		labels = append(labels, entry.label)
	}
	return labels
}

// addFact adds "label: value" for a field that is set.
func (summary *Summary) addFact(label string, value model.Value[string]) {
	if value.Set && value.Value != "" {
		summary.Facts = append(summary.Facts, label+": "+value.Value)
	}
}

func listOrNone(names []string, none string) string {
	if len(names) == 0 {
		return none
	}
	return strings.Join(names, ", ")
}
