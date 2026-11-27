package security

import (
	"github.com/mfast99/concourse-lint/internal/model"
)

// source is a source: block together with what it belongs to.
type source struct {
	node *model.Node
	// typeName is the resource type, e.g. "registry-image" (may be empty).
	typeName string
}

// allSources returns the source: blocks of resources, resource types and the
// image_resource of inline tasks.
func allSources(pipeline *model.Pipeline) []source {
	var sources []source
	for _, resource := range pipeline.Resources {
		if resource != nil && resource.Source != nil {
			sources = append(sources, source{node: resource.Source, typeName: resource.Type.Value})
		}
	}
	for _, resourceType := range pipeline.ResourceTypes {
		if resourceType != nil && resourceType.Source != nil {
			sources = append(sources, source{node: resourceType.Source, typeName: resourceType.Type.Value})
		}
	}
	for _, imageResource := range imageResources(pipeline) {
		if imageResource.Source != nil {
			sources = append(sources, source{node: imageResource.Source, typeName: imageResource.Type.Value})
		}
	}
	return sources
}

// imageResources returns the image_resource of every inline task.
func imageResources(pipeline *model.Pipeline) []*model.ImageResource {
	var images []*model.ImageResource
	forEachStep(pipeline, func(step *model.Step) {
		if step.Task != nil && step.Task.Config != nil && step.Task.Config.ImageResource != nil {
			images = append(images, step.Task.Config.ImageResource)
		}
	})
	return images
}

// forEachStep calls visit for every step of every job, hooks included.
func forEachStep(pipeline *model.Pipeline, visit func(step *model.Step)) {
	for _, job := range pipeline.Jobs {
		if job == nil {
			continue
		}
		for step := range job.AllSteps() {
			visit(step)
		}
	}
}

// field returns the value of a key in a mapping node, or nil.
func field(node *model.Node, key string) *model.Node {
	if node == nil || node.Kind != model.MappingNode {
		return nil
	}
	for _, pair := range node.Pairs {
		if pair.Key == key {
			return pair.Value
		}
	}
	return nil
}

// forEachScalar calls visit for every scalar inside the node.
func forEachScalar(node *model.Node, visit func(scalar *model.Node)) {
	if node == nil {
		return
	}
	switch node.Kind {
	case model.ScalarNode:
		visit(node)
	case model.MappingNode:
		for _, pair := range node.Pairs {
			forEachScalar(pair.Value, visit)
		}
	case model.ListNode:
		for _, item := range node.Items {
			forEachScalar(item, visit)
		}
	}
}
