package parse

import (
	"github.com/mfast99/go-yaml/ast"

	"github.com/mfast99/concourse-lint/internal/model"
)

var taskFields = fieldTable[model.TaskStep]{
	"privileged": func(decoder *pipelineDecoder, task *model.TaskStep, pair *ast.MappingValueNode) {
		task.Privileged = decoder.boolValue(pair)
	},
	"hermetic": func(decoder *pipelineDecoder, task *model.TaskStep, pair *ast.MappingValueNode) {
		task.Hermetic = decoder.boolValue(pair)
	},
	"file": func(decoder *pipelineDecoder, task *model.TaskStep, pair *ast.MappingValueNode) {
		task.File = decoder.stringValue(pair)
	},
	"container_limits": func(decoder *pipelineDecoder, task *model.TaskStep, pair *ast.MappingValueNode) {
		task.ContainerLimits = decoder.freeMapping(pair)
	},
	"config": func(decoder *pipelineDecoder, task *model.TaskStep, pair *ast.MappingValueNode) {
		task.Config = objectField(decoder, pair, decoder.taskConfig)
		if configPairs, isMapping := decoder.mapping(pair.Value); isMapping {
			decoder.requireField(configPairs, "platform", "platform", "task config", position(pair.Key))
		}
	},
	"params": func(decoder *pipelineDecoder, task *model.TaskStep, pair *ast.MappingValueNode) {
		task.Params = decoder.freeMapping(pair)
	},
	"vars": func(decoder *pipelineDecoder, task *model.TaskStep, pair *ast.MappingValueNode) {
		task.Vars = decoder.freeMapping(pair)
	},
	"input_mapping": func(decoder *pipelineDecoder, task *model.TaskStep, pair *ast.MappingValueNode) {
		task.InputMapping = decoder.artifactMappings(pair)
	},
	"output_mapping": func(decoder *pipelineDecoder, task *model.TaskStep, pair *ast.MappingValueNode) {
		task.OutputMapping = decoder.artifactMappings(pair)
	},
	"image": func(decoder *pipelineDecoder, task *model.TaskStep, pair *ast.MappingValueNode) {
		task.Image = decoder.stringValue(pair)
	},
}

var taskConfigFields = fieldTable[model.TaskConfig]{
	"platform": func(decoder *pipelineDecoder, config *model.TaskConfig, pair *ast.MappingValueNode) {
		config.Platform = decoder.stringValue(pair)
	},
	"rootfs_uri": func(decoder *pipelineDecoder, config *model.TaskConfig, pair *ast.MappingValueNode) {
		config.RootfsURI = decoder.stringValue(pair)
	},
	"image_resource": func(decoder *pipelineDecoder, config *model.TaskConfig, pair *ast.MappingValueNode) {
		config.ImageResource = objectField(decoder, pair, decoder.imageResource)
	},
	"container_limits": func(decoder *pipelineDecoder, config *model.TaskConfig, pair *ast.MappingValueNode) {
		config.ContainerLimits = decoder.freeMapping(pair)
	},
	"params": func(decoder *pipelineDecoder, config *model.TaskConfig, pair *ast.MappingValueNode) {
		config.Params = decoder.freeMapping(pair)
	},
	"run": func(decoder *pipelineDecoder, config *model.TaskConfig, pair *ast.MappingValueNode) {
		config.Run = objectField(decoder, pair, decoder.taskRun)
	},
	"inputs": func(decoder *pipelineDecoder, config *model.TaskConfig, pair *ast.MappingValueNode) {
		config.Inputs = decodeList(decoder, pair, "task input", decoder.taskInput)
	},
	"outputs": func(decoder *pipelineDecoder, config *model.TaskConfig, pair *ast.MappingValueNode) {
		config.Outputs = decodeList(decoder, pair, "task output", decoder.taskOutput)
	},
	"caches": func(decoder *pipelineDecoder, config *model.TaskConfig, pair *ast.MappingValueNode) {
		config.Caches = decodeList(decoder, pair, "task cache", decoder.taskCache)
	},
}

func (decoder *pipelineDecoder) taskConfig(node ast.Node) *model.TaskConfig {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	config := &model.TaskConfig{Position: decoder.elementPosition(node)}
	decodeFieldsStrictly(decoder, config, pairs, taskConfigFields, "task config")
	return config
}

var imageResourceFields = fieldTable[model.ImageResource]{
	"name": func(decoder *pipelineDecoder, image *model.ImageResource, pair *ast.MappingValueNode) {
		image.Name = decoder.stringValue(pair)
	},
	"type": func(decoder *pipelineDecoder, image *model.ImageResource, pair *ast.MappingValueNode) {
		image.Type = decoder.stringValue(pair)
	},
	"source": func(decoder *pipelineDecoder, image *model.ImageResource, pair *ast.MappingValueNode) {
		image.Source = decoder.freeMapping(pair)
	},
	"version": func(decoder *pipelineDecoder, image *model.ImageResource, pair *ast.MappingValueNode) {
		image.Version = decoder.freeMapping(pair)
	},
	"params": func(decoder *pipelineDecoder, image *model.ImageResource, pair *ast.MappingValueNode) {
		image.Params = decoder.freeMapping(pair)
	},
	"tags": func(decoder *pipelineDecoder, image *model.ImageResource, pair *ast.MappingValueNode) {
		image.Tags = decoder.stringList(pair)
	},
}

func (decoder *pipelineDecoder) imageResource(node ast.Node) *model.ImageResource {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	image := &model.ImageResource{Position: decoder.elementPosition(node)}
	decodeFields(decoder, image, pairs, imageResourceFields, "image_resource")
	return image
}

var taskRunFields = fieldTable[model.TaskRun]{
	"path": func(decoder *pipelineDecoder, run *model.TaskRun, pair *ast.MappingValueNode) {
		run.Path = decoder.stringValue(pair)
	},
	"args": func(decoder *pipelineDecoder, run *model.TaskRun, pair *ast.MappingValueNode) {
		run.Args = decoder.stringList(pair)
	},
	"dir": func(decoder *pipelineDecoder, run *model.TaskRun, pair *ast.MappingValueNode) {
		run.Dir = decoder.stringValue(pair)
	},
	"user": func(decoder *pipelineDecoder, run *model.TaskRun, pair *ast.MappingValueNode) {
		run.User = decoder.stringValue(pair)
	},
}

func (decoder *pipelineDecoder) taskRun(node ast.Node) *model.TaskRun {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	run := &model.TaskRun{Position: decoder.elementPosition(node)}
	decodeFields(decoder, run, pairs, taskRunFields, "run")
	return run
}

var taskInputFields = fieldTable[model.TaskInput]{
	"name": func(decoder *pipelineDecoder, input *model.TaskInput, pair *ast.MappingValueNode) {
		input.Name = decoder.stringValue(pair)
	},
	"path": func(decoder *pipelineDecoder, input *model.TaskInput, pair *ast.MappingValueNode) {
		input.Path = decoder.stringValue(pair)
	},
	"optional": func(decoder *pipelineDecoder, input *model.TaskInput, pair *ast.MappingValueNode) {
		input.Optional = decoder.boolValue(pair)
	},
}

func (decoder *pipelineDecoder) taskInput(node ast.Node) *model.TaskInput {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	input := &model.TaskInput{Position: decoder.elementPosition(node)}
	decodeFields(decoder, input, pairs, taskInputFields, "task input")
	decoder.requireField(pairs, "name", "name", "task input", input.Position)
	return input
}

var taskOutputFields = fieldTable[model.TaskOutput]{
	"name": func(decoder *pipelineDecoder, output *model.TaskOutput, pair *ast.MappingValueNode) {
		output.Name = decoder.stringValue(pair)
	},
	"path": func(decoder *pipelineDecoder, output *model.TaskOutput, pair *ast.MappingValueNode) {
		output.Path = decoder.stringValue(pair)
	},
}

func (decoder *pipelineDecoder) taskOutput(node ast.Node) *model.TaskOutput {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	output := &model.TaskOutput{Position: decoder.elementPosition(node)}
	decodeFields(decoder, output, pairs, taskOutputFields, "task output")
	decoder.requireField(pairs, "name", "name", "task output", output.Position)
	return output
}

var taskCacheFields = fieldTable[model.TaskCache]{
	"path": func(decoder *pipelineDecoder, cache *model.TaskCache, pair *ast.MappingValueNode) {
		cache.Path = decoder.stringValue(pair)
	},
	"ttl": func(decoder *pipelineDecoder, cache *model.TaskCache, pair *ast.MappingValueNode) {
		cache.TTL = decoder.durationValue(pair, false)
	},
}

func (decoder *pipelineDecoder) taskCache(node ast.Node) *model.TaskCache {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	cache := &model.TaskCache{Position: decoder.elementPosition(node)}
	decodeFields(decoder, cache, pairs, taskCacheFields, "task cache")
	return cache
}
