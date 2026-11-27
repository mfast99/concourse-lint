package parse

import (
	"fmt"

	"github.com/mfast99/go-yaml/ast"

	"github.com/mfast99/concourse-lint/internal/model"
)

var pipelineFields = fieldTable[model.Pipeline]{
	"groups": func(decoder *pipelineDecoder, pipeline *model.Pipeline, pair *ast.MappingValueNode) {
		pipeline.Groups = decodeList(decoder, pair, "group", decoder.group)
	},
	"var_sources": func(decoder *pipelineDecoder, pipeline *model.Pipeline, pair *ast.MappingValueNode) {
		pipeline.VarSources = decodeList(decoder, pair, "var source", decoder.varSource)
	},
	"resources": func(decoder *pipelineDecoder, pipeline *model.Pipeline, pair *ast.MappingValueNode) {
		pipeline.Resources = decodeList(decoder, pair, "resource", decoder.resource)
	},
	"resource_types": func(decoder *pipelineDecoder, pipeline *model.Pipeline, pair *ast.MappingValueNode) {
		pipeline.ResourceTypes = decodeList(decoder, pair, "resource type", decoder.resourceType)
	},
	"prototypes": func(decoder *pipelineDecoder, pipeline *model.Pipeline, pair *ast.MappingValueNode) {
		pipeline.Prototypes = decodeList(decoder, pair, "prototype", decoder.prototype)
	},
	"jobs": func(decoder *pipelineDecoder, pipeline *model.Pipeline, pair *ast.MappingValueNode) {
		pipeline.Jobs = decodeList(decoder, pair, "job", decoder.job)
	},
	"display": func(decoder *pipelineDecoder, pipeline *model.Pipeline, pair *ast.MappingValueNode) {
		pipeline.Display = objectField(decoder, pair, decoder.display)
	},
	"user_data": func(decoder *pipelineDecoder, pipeline *model.Pipeline, pair *ast.MappingValueNode) {
		pipeline.UserData = decoder.freeNode(pair.Value)
	},
}

// pipeline decodes the top level of the file.
func (decoder *pipelineDecoder) pipeline(body ast.Node) *model.Pipeline {
	pipeline := &model.Pipeline{}
	pairs, _ := decoder.mapping(body)
	for _, pair := range pairs {
		// A top-level key that holds anchors is a common way to share
		// blocks; Concourse ignores it, and so do we.
		if decodeField(decoder, pipeline, pair, pipelineFields) || containsAnchor(pair.Value) {
			continue
		}
		decoder.reportIgnoredField(position(pair.Key),
			fmt.Sprintf("unknown top-level field %q, which Concourse ignores", key(pair)),
			didYouMean(key(pair), pipelineFields.keys()))
		decoder.suggest(key(pair), pipelineFields.keys())
	}
	if len(pipeline.Jobs) == 0 {
		decoder.reportStructureError(decoder.jobsPosition(pairs), "pipeline must contain at least one job", "")
	}
	return pipeline
}

// jobsPosition is where a missing job is reported: at the jobs: key, or
// without one at the end of the file, where it would be added. A fixed line 1
// would not move with the content.
func (decoder *pipelineDecoder) jobsPosition(pairs []*ast.MappingValueNode) model.Position {
	for _, pair := range pairs {
		if key(pair) == "jobs" {
			return position(pair.Key)
		}
	}
	return model.Position{Line: decoder.lastLine, Column: 1}
}

var groupFields = fieldTable[model.Group]{
	"name": func(decoder *pipelineDecoder, group *model.Group, pair *ast.MappingValueNode) {
		group.Name = decoder.stringValue(pair)
	},
	"jobs": func(decoder *pipelineDecoder, group *model.Group, pair *ast.MappingValueNode) {
		group.Jobs = decoder.stringList(pair)
	},
	"resources": func(decoder *pipelineDecoder, group *model.Group, pair *ast.MappingValueNode) {
		group.Resources = decoder.stringList(pair)
	},
}

func (decoder *pipelineDecoder) group(node ast.Node) *model.Group {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	group := &model.Group{Position: decoder.elementPosition(node)}
	decodeFields(decoder, group, pairs, groupFields, "group")
	decoder.requireField(pairs, "name", "name", "group", group.Position)
	return group
}

var varSourceFields = fieldTable[model.VarSource]{
	"name": func(decoder *pipelineDecoder, varSource *model.VarSource, pair *ast.MappingValueNode) {
		varSource.Name = decoder.stringValue(pair)
	},
	"type": func(decoder *pipelineDecoder, varSource *model.VarSource, pair *ast.MappingValueNode) {
		varSource.Type = decoder.stringValue(pair)
	},
	"config": func(decoder *pipelineDecoder, varSource *model.VarSource, pair *ast.MappingValueNode) {
		varSource.Config = decoder.freeNode(pair.Value)
	},
}

func (decoder *pipelineDecoder) varSource(node ast.Node) *model.VarSource {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	varSource := &model.VarSource{Position: decoder.elementPosition(node)}
	decodeFields(decoder, varSource, pairs, varSourceFields, "var source")
	decoder.requireField(pairs, "name", "name", "var source", varSource.Position)
	decoder.requireField(pairs, "type", "type", "var source", varSource.Position)
	return varSource
}

var resourceFields = fieldTable[model.Resource]{
	"name": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.Name = decoder.stringValue(pair)
	},
	"old_name": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.OldName = decoder.stringValue(pair)
	},
	"public": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.Public = decoder.boolValue(pair)
	},
	"webhook_token": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.WebhookToken = decoder.stringValue(pair)
	},
	"type": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.Type = decoder.stringValue(pair)
	},
	"source": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.Source = decoder.freeMapping(pair)
	},
	"check_every": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.CheckEvery = decoder.durationValue(pair, true)
	},
	"check_timeout": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.CheckTimeout = decoder.runtimeDuration(pair, "every check fails")
	},
	"tags": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.Tags = decoder.stringList(pair)
	},
	"version": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.Version = decoder.freeMapping(pair)
	},
	"icon": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.Icon = decoder.stringValue(pair)
	},
	"expose_build_created_by": func(decoder *pipelineDecoder, resource *model.Resource, pair *ast.MappingValueNode) {
		resource.ExposeBuildCreatedBy = decoder.boolValue(pair)
	},
}

func (decoder *pipelineDecoder) resource(node ast.Node) *model.Resource {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	resource := &model.Resource{Position: decoder.elementPosition(node)}
	decodeFields(decoder, resource, pairs, resourceFields, "resource")
	decoder.requireField(pairs, "name", "name", "resource", resource.Position)
	decoder.requireField(pairs, "type", "type", "resource", resource.Position)
	return resource
}

// resourceTypeFields are the keys of an entry of resource_types: or
// prototypes:.
var resourceTypeFields = fieldTable[model.ResourceType]{
	"name": func(decoder *pipelineDecoder, resourceType *model.ResourceType, pair *ast.MappingValueNode) {
		resourceType.Name = decoder.stringValue(pair)
	},
	"type": func(decoder *pipelineDecoder, resourceType *model.ResourceType, pair *ast.MappingValueNode) {
		resourceType.Type = decoder.stringValue(pair)
	},
	"source": func(decoder *pipelineDecoder, resourceType *model.ResourceType, pair *ast.MappingValueNode) {
		resourceType.Source = decoder.freeMapping(pair)
	},
	"defaults": func(decoder *pipelineDecoder, resourceType *model.ResourceType, pair *ast.MappingValueNode) {
		resourceType.Defaults = decoder.freeMapping(pair)
	},
	"privileged": func(decoder *pipelineDecoder, resourceType *model.ResourceType, pair *ast.MappingValueNode) {
		resourceType.Privileged = decoder.boolValue(pair)
	},
	"check_every": func(decoder *pipelineDecoder, resourceType *model.ResourceType, pair *ast.MappingValueNode) {
		resourceType.CheckEvery = decoder.durationValue(pair, true)
	},
	"tags": func(decoder *pipelineDecoder, resourceType *model.ResourceType, pair *ast.MappingValueNode) {
		resourceType.Tags = decoder.stringList(pair)
	},
	"params": func(decoder *pipelineDecoder, resourceType *model.ResourceType, pair *ast.MappingValueNode) {
		resourceType.Params = decoder.freeMapping(pair)
	},
}

func (decoder *pipelineDecoder) resourceType(node ast.Node) *model.ResourceType {
	return decoder.resourceTypeNamed(node, "resource type")
}

// prototype decodes an entry of prototypes:, which has the keys of a
// resource type.
func (decoder *pipelineDecoder) prototype(node ast.Node) *model.ResourceType {
	return decoder.resourceTypeNamed(node, "prototype")
}

func (decoder *pipelineDecoder) resourceTypeNamed(node ast.Node, objectName string) *model.ResourceType {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	resourceType := &model.ResourceType{Position: decoder.elementPosition(node)}
	decodeFields(decoder, resourceType, pairs, resourceTypeFields, objectName)
	decoder.requireField(pairs, "name", "name", objectName, resourceType.Position)
	decoder.requireField(pairs, "type", "type", objectName, resourceType.Position)
	return resourceType
}

var displayFields = fieldTable[model.Display]{
	"background_image": func(decoder *pipelineDecoder, display *model.Display, pair *ast.MappingValueNode) {
		display.BackgroundImage = decoder.stringValue(pair)
	},
	"background_filter": func(decoder *pipelineDecoder, display *model.Display, pair *ast.MappingValueNode) {
		display.BackgroundFilter = decoder.stringValue(pair)
	},
}

func (decoder *pipelineDecoder) display(node ast.Node) *model.Display {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	display := &model.Display{Position: decoder.elementPosition(node)}
	decodeFields(decoder, display, pairs, displayFields, "display")
	return display
}

// containsAnchor reports whether an anchor (&name) is defined anywhere in the
// value as it is written. Aliases are not followed.
func containsAnchor(node ast.Node) bool {
	switch typed := node.(type) {
	case *ast.AnchorNode:
		return true
	case *ast.TagNode:
		return containsAnchor(typed.Value)
	case *ast.MappingValueNode:
		return containsAnchor(typed.Key) || containsAnchor(typed.Value)
	case *ast.MappingNode:
		for _, pair := range typed.Values {
			if containsAnchor(pair) {
				return true
			}
		}
	case *ast.SequenceNode:
		for _, item := range typed.Values {
			if containsAnchor(item) {
				return true
			}
		}
	}
	return false
}
