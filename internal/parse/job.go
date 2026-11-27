package parse

import (
	"github.com/mfast99/go-yaml/ast"

	"github.com/mfast99/concourse-lint/internal/model"
)

var jobFields = withHooks(fieldTable[model.Job]{
	"name": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.Name = decoder.stringValue(pair)
	},
	"old_name": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.OldName = decoder.stringValue(pair)
	},
	"public": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.Public = decoder.boolValue(pair)
	},
	"disable_manual_trigger": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.DisableManualTrigger = decoder.boolValue(pair)
	},
	"disable_reruns": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.DisableReruns = decoder.boolValue(pair)
	},
	"serial": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.Serial = decoder.boolValue(pair)
	},
	"interruptible": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.Interruptible = decoder.boolValue(pair)
	},
	"serial_groups": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.SerialGroups = decoder.stringList(pair)
	},
	"max_in_flight": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.MaxInFlight = decoder.intValue(pair)
	},
	"tags": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.Tags = decoder.stringList(pair)
	},
	"build_logs_to_retain": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.BuildLogsToRetain = decoder.intValue(pair)
	},
	"build_log_retention": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.BuildLogRetention = objectField(decoder, pair, decoder.buildLogRetention)
	},
	"plan": func(decoder *pipelineDecoder, job *model.Job, pair *ast.MappingValueNode) {
		job.Plan = decodeList(decoder, pair, "step", decoder.step)
		if _, isList := decoder.sequence(pair.Value); !isList {
			_, isNull := decoder.resolve(pair.Value).(*ast.NullNode)
			job.PlanUnknown = !isNull
		}
	},
}, func(job *model.Job) *model.Hooks { return &job.Hooks })

func (decoder *pipelineDecoder) job(node ast.Node) *model.Job {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	job := &model.Job{Position: decoder.elementPosition(node)}
	decodeFields(decoder, job, pairs, jobFields, "job")
	decoder.requireField(pairs, "name", "name", "job", job.Position)
	return job
}

var buildLogRetentionFields = fieldTable[model.BuildLogRetention]{
	"builds": func(decoder *pipelineDecoder, retention *model.BuildLogRetention, pair *ast.MappingValueNode) {
		retention.Builds = decoder.intValue(pair)
	},
	"minimum_succeeded_builds": func(decoder *pipelineDecoder, retention *model.BuildLogRetention, pair *ast.MappingValueNode) {
		retention.MinimumSucceededBuilds = decoder.intValue(pair)
	},
	"days": func(decoder *pipelineDecoder, retention *model.BuildLogRetention, pair *ast.MappingValueNode) {
		retention.Days = decoder.intValue(pair)
	},
}

func (decoder *pipelineDecoder) buildLogRetention(node ast.Node) *model.BuildLogRetention {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	retention := &model.BuildLogRetention{Position: decoder.elementPosition(node)}
	decodeFields(decoder, retention, pairs, buildLogRetentionFields, "build_log_retention")
	return retention
}
