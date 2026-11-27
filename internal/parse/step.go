package parse

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mfast99/go-yaml/ast"

	"github.com/mfast99/concourse-lint/internal/model"
)

// The step tables refer to decoder.step (nested steps and hooks), and
// decoder.step refers to them. Go does not allow such a cycle between
// package-level variables and their initial values, so these two are filled
// in init(), which runs once when the program starts.
var (
	// stepKindFields set the step kind, together with the value of its key
	// (e.g. the resource of get:).
	stepKindFields fieldTable[model.Step]
	// stepModifierFields apply to every step kind.
	stepModifierFields fieldTable[model.Step]
)

func init() {
	stepKindFields = fieldTable[model.Step]{
		"get": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.Get = &model.GetStep{Name: decoder.stringValue(pair)}
		},
		"put": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.Put = &model.PutStep{Name: decoder.stringValue(pair)}
		},
		"task": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.Task = &model.TaskStep{Name: decoder.stringValue(pair)}
		},
		"run": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.Run = &model.RunStep{Message: decoder.stringValue(pair)}
		},
		"set_pipeline": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.SetPipeline = &model.SetPipelineStep{Name: decoder.stringValue(pair)}
		},
		"load_var": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.LoadVar = &model.LoadVarStep{Name: decoder.stringValue(pair)}
		},
		"in_parallel": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.InParallel = decoder.inParallel(pair)
		},
		"do": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.Do = &model.DoStep{Steps: decodeList(decoder, pair, "step", decoder.step)}
		},
		"try": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.Try = &model.TryStep{Step: objectField(decoder, pair, decoder.step)}
		},
	}
	stepModifierFields = withHooks(fieldTable[model.Step]{
		"across": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.Across = &model.Across{Variables: decodeList(decoder, pair, "across variable", decoder.acrossVariable)}
			if items, isList := decoder.sequence(pair.Value); isList && len(items) == 0 {
				decoder.reportStructureError(position(pair.Key), "across needs at least one variable", "")
			}
		},
		"attempts": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.Attempts = decoder.intValue(pair)
		},
		"timeout": func(decoder *pipelineDecoder, step *model.Step, pair *ast.MappingValueNode) {
			step.Timeout = decoder.runtimeDuration(pair, "the step fails every time it runs")
		},
	}, func(step *model.Step) *model.Hooks { return &step.Hooks })
}

// step decodes a step in two passes: first the step kind and the modifiers,
// then the fields of the kind. In YAML the order of keys is free, so "file:"
// may come before "task:".
func (decoder *pipelineDecoder) step(node ast.Node) *model.Step {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	// defer runs when the function returns, like finally in C#: the depth
	// goes down again however step() ends.
	decoder.stepDepth++
	defer func() { decoder.stepDepth-- }()
	step := &model.Step{Position: decoder.elementPosition(node)}
	var kindPairs []*ast.MappingValueNode
	for _, pair := range pairs {
		if !decodeField(decoder, step, pair, stepKindFields) && !decodeField(decoder, step, pair, stepModifierFields) {
			kindPairs = append(kindPairs, pair)
		}
	}
	for _, pair := range kindPairs {
		if !decoder.stepKindField(step, pair) {
			decoder.reportUnknownStepField(step, pair)
		}
	}
	decoder.checkStepKind(step, pairs)
	return step
}

// checkStepKind reports a step without a step type or with more than one,
// and a task step without file: or config: or with both (fly rejects all of
// them). Reported at the first line of the step.
func (decoder *pipelineDecoder) checkStepKind(step *model.Step, pairs []*ast.MappingValueNode) {
	kinds := stepKindNames(step)
	switch {
	case len(kinds) == 0:
		decoder.reportStructureError(step.Position,
			"step has no step type (get, put, task, run, set_pipeline, load_var, in_parallel, do or try)",
			decoder.stepKindHint(pairs))
	case len(kinds) > 1:
		decoder.reportStructureError(step.Position,
			"step has more than one step type: "+strings.Join(kinds, " and "),
			"use one step per type")
	default:
		decoder.checkRequiredStepFields(step, pairs, kinds[0])
	}
}

// stepKindHint suggests a step type for a step without one, if one of its
// keys looks like a misspelled step type (e.g. tsak).
func (decoder *pipelineDecoder) stepKindHint(pairs []*ast.MappingValueNode) string {
	if hasKey(pairs, "aggregate") {
		return "aggregate was removed in Concourse 7, use in_parallel"
	}
	for _, pair := range pairs {
		if hint := didYouMean(key(pair), stepKindFields.keys()); hint != "" {
			return hint
		}
	}
	return ""
}

// hasKey reports whether one of the pairs has the key.
func hasKey(pairs []*ast.MappingValueNode, wanted string) bool {
	return slices.ContainsFunc(pairs, func(pair *ast.MappingValueNode) bool { return key(pair) == wanted })
}

// stepKindField decodes a key that belongs to the step kind, e.g. trigger: of
// a get step, and reports whether it does belong to it.
func (decoder *pipelineDecoder) stepKindField(step *model.Step, pair *ast.MappingValueNode) bool {
	switch {
	// fail_fast belongs to across, which wraps every step kind.
	case key(pair) == "fail_fast" && step.Across != nil:
		step.Across.FailFast = decoder.boolValue(pair)
		return true
	case key(pair) == "tags" && slices.Contains(stepKindsWithTags, stepKindName(step)):
		step.Tags = decoder.stringList(pair)
		return true
	case step.Get != nil:
		return decodeField(decoder, step.Get, pair, getFields)
	case step.Put != nil:
		return decodeField(decoder, step.Put, pair, putFields)
	case step.Task != nil:
		return decodeField(decoder, step.Task, pair, taskFields)
	case step.Run != nil:
		return decodeField(decoder, step.Run, pair, runFields)
	case step.SetPipeline != nil:
		return decodeField(decoder, step.SetPipeline, pair, setPipelineFields)
	case step.LoadVar != nil:
		return decodeField(decoder, step.LoadVar, pair, loadVarFields)
	}
	return false
}

var getFields = fieldTable[model.GetStep]{
	"resource": func(decoder *pipelineDecoder, get *model.GetStep, pair *ast.MappingValueNode) {
		get.Resource = decoder.stringValue(pair)
	},
	"version": func(decoder *pipelineDecoder, get *model.GetStep, pair *ast.MappingValueNode) {
		get.Version = decoder.freeNode(pair.Value)
		decoder.checkVersion(pair)
	},
	"params": func(decoder *pipelineDecoder, get *model.GetStep, pair *ast.MappingValueNode) {
		get.Params = decoder.freeMapping(pair)
	},
	"passed": func(decoder *pipelineDecoder, get *model.GetStep, pair *ast.MappingValueNode) {
		get.Passed = decoder.stringList(pair)
	},
	"trigger": func(decoder *pipelineDecoder, get *model.GetStep, pair *ast.MappingValueNode) {
		get.Trigger = decoder.boolValue(pair)
	},
}

var putFields = fieldTable[model.PutStep]{
	"resource": func(decoder *pipelineDecoder, put *model.PutStep, pair *ast.MappingValueNode) {
		put.Resource = decoder.stringValue(pair)
	},
	"params": func(decoder *pipelineDecoder, put *model.PutStep, pair *ast.MappingValueNode) {
		put.Params = decoder.freeMapping(pair)
	},
	"inputs": func(decoder *pipelineDecoder, put *model.PutStep, pair *ast.MappingValueNode) {
		put.Inputs = decoder.freeNode(pair.Value)
		decoder.checkPutInputs(pair)
	},
	"get_params": func(decoder *pipelineDecoder, put *model.PutStep, pair *ast.MappingValueNode) {
		put.GetParams = decoder.freeMapping(pair)
	},
	"no_get": func(decoder *pipelineDecoder, put *model.PutStep, pair *ast.MappingValueNode) {
		put.NoGet = decoder.boolValue(pair)
	},
}

var runFields = fieldTable[model.RunStep]{
	"type": func(decoder *pipelineDecoder, run *model.RunStep, pair *ast.MappingValueNode) {
		run.Type = decoder.stringValue(pair)
	},
	"params": func(decoder *pipelineDecoder, run *model.RunStep, pair *ast.MappingValueNode) {
		run.Params = decoder.freeMapping(pair)
	},
	"privileged": func(decoder *pipelineDecoder, run *model.RunStep, pair *ast.MappingValueNode) {
		run.Privileged = decoder.boolValue(pair)
	},
	"container_limits": func(decoder *pipelineDecoder, run *model.RunStep, pair *ast.MappingValueNode) {
		run.ContainerLimits = decoder.freeMapping(pair)
	},
}

var setPipelineFields = fieldTable[model.SetPipelineStep]{
	"file": func(decoder *pipelineDecoder, setPipeline *model.SetPipelineStep, pair *ast.MappingValueNode) {
		setPipeline.File = decoder.stringValue(pair)
	},
	"team": func(decoder *pipelineDecoder, setPipeline *model.SetPipelineStep, pair *ast.MappingValueNode) {
		setPipeline.Team = decoder.stringValue(pair)
	},
	"vars": func(decoder *pipelineDecoder, setPipeline *model.SetPipelineStep, pair *ast.MappingValueNode) {
		setPipeline.Vars = decoder.freeMapping(pair)
	},
	"var_files": func(decoder *pipelineDecoder, setPipeline *model.SetPipelineStep, pair *ast.MappingValueNode) {
		setPipeline.VarFiles = decoder.stringList(pair)
	},
	"instance_vars": func(decoder *pipelineDecoder, setPipeline *model.SetPipelineStep, pair *ast.MappingValueNode) {
		setPipeline.InstanceVars = decoder.freeMapping(pair)
	},
}

var loadVarFields = fieldTable[model.LoadVarStep]{
	"file": func(decoder *pipelineDecoder, loadVar *model.LoadVarStep, pair *ast.MappingValueNode) {
		loadVar.File = decoder.stringValue(pair)
	},
	"format": func(decoder *pipelineDecoder, loadVar *model.LoadVarStep, pair *ast.MappingValueNode) {
		loadVar.Format = decoder.loadVarFormat(pair)
	},
	"reveal": func(decoder *pipelineDecoder, loadVar *model.LoadVarStep, pair *ast.MappingValueNode) {
		loadVar.Reveal = decoder.boolValue(pair)
	},
}

// inParallelFields are the keys of the block form of in_parallel.
var inParallelFields = fieldTable[model.InParallelStep]{
	"steps": func(decoder *pipelineDecoder, inParallel *model.InParallelStep, pair *ast.MappingValueNode) {
		inParallel.Steps = decodeList(decoder, pair, "step", decoder.step)
	},
	"limit": func(decoder *pipelineDecoder, inParallel *model.InParallelStep, pair *ast.MappingValueNode) {
		inParallel.Limit = decoder.intValue(pair)
	},
	"fail_fast": func(decoder *pipelineDecoder, inParallel *model.InParallelStep, pair *ast.MappingValueNode) {
		inParallel.FailFast = decoder.boolValue(pair)
	},
}

// inParallel decodes both forms: a plain list of steps, or a block with
// steps:, limit: and fail_fast:.
func (decoder *pipelineDecoder) inParallel(pair *ast.MappingValueNode) *model.InParallelStep {
	inParallel := &model.InParallelStep{}
	if items, isList := decoder.sequence(pair.Value); isList {
		inParallel.Steps = decodeItems(decoder, items, "step", decoder.step)
		return inParallel
	}
	pairs, isMapping := decoder.mapping(pair.Value)
	if !isMapping {
		if !decoder.isUnknownValue(pair.Value) {
			decoder.reportWrongType(pair, "a list of steps or a mapping", "")
		}
		return inParallel
	}
	decodeFields(decoder, inParallel, pairs, inParallelFields, "in_parallel")
	return inParallel
}

var acrossVariableFields = fieldTable[model.AcrossVariable]{
	"var": func(decoder *pipelineDecoder, variable *model.AcrossVariable, pair *ast.MappingValueNode) {
		variable.Name = decoder.stringValue(pair)
	},
	"values": func(decoder *pipelineDecoder, variable *model.AcrossVariable, pair *ast.MappingValueNode) {
		variable.Values = decoder.freeNode(pair.Value)
	},
	"max_in_flight": func(decoder *pipelineDecoder, variable *model.AcrossVariable, pair *ast.MappingValueNode) {
		variable.MaxInFlight = decoder.freeNode(pair.Value)
		decoder.checkMaxInFlight(pair)
	},
}

func (decoder *pipelineDecoder) acrossVariable(node ast.Node) *model.AcrossVariable {
	pairs, isMapping := decoder.mapping(node)
	if !isMapping {
		return nil
	}
	variable := &model.AcrossVariable{Position: decoder.elementPosition(node)}
	decodeFieldsStrictly(decoder, variable, pairs, acrossVariableFields, "across variable")
	return variable
}

// stepKindsWithTags are the step kinds that accept tags: (atc structs of
// Concourse 8.3.4; fly rejects tags on load_var, set_pipeline and try).
var stepKindsWithTags = []string{"get", "put", "task", "run", "do", "in_parallel"}

// stepKindsWithFields are the step kinds that have fields of their own.
var stepKindsWithFields = []string{"get", "put", "task", "run", "set_pipeline", "load_var"}

// stepKindNames returns the keys of all step kinds set on the step, e.g.
// ["get"], in a fixed order.
func stepKindNames(step *model.Step) []string {
	var names []string
	for _, kind := range []struct {
		name  string
		isSet bool
	}{
		{"get", step.Get != nil},
		{"put", step.Put != nil},
		{"task", step.Task != nil},
		{"run", step.Run != nil},
		{"set_pipeline", step.SetPipeline != nil},
		{"load_var", step.LoadVar != nil},
		{"in_parallel", step.InParallel != nil},
		{"do", step.Do != nil},
		{"try", step.Try != nil},
	} {
		if kind.isSet {
			names = append(names, kind.name)
		}
	}
	return names
}

// stepKindName returns the key of the step kind, e.g. "get", or "" if the
// step has none.
func stepKindName(step *model.Step) string {
	if names := stepKindNames(step); len(names) > 0 {
		return names[0]
	}
	return ""
}

// stepKindKeys returns the keys that belong to one step kind.
func stepKindKeys(kind string) []string {
	switch kind {
	case "get":
		return getFields.keys()
	case "put":
		return putFields.keys()
	case "task":
		return taskFields.keys()
	case "run":
		return runFields.keys()
	case "set_pipeline":
		return setPipelineFields.keys()
	case "load_var":
		return loadVarFields.keys()
	}
	return nil
}

// reportUnknownStepField reports a key that belongs neither to the step kind
// nor to the modifiers: CL001, because Concourse rejects it. A step without a
// kind or with two is reported as a whole
// instead, so its keys are not reported one by one.
func (decoder *pipelineDecoder) reportUnknownStepField(step *model.Step, pair *ast.MappingValueNode) {
	if len(stepKindNames(step)) != 1 {
		return
	}
	kind, unknownKey := stepKindName(step), key(pair)
	var owners []string
	for _, otherKind := range stepKindsWithFields {
		if slices.Contains(stepKindKeys(otherKind), unknownKey) {
			owners = append(owners, otherKind)
		}
	}
	if unknownKey == "tags" {
		owners = stepKindsWithTags
	}
	switch {
	case unknownKey == "fail_fast":
		decoder.reportStructureError(position(pair.Key),
			fmt.Sprintf("%s step has no field %q", kind, unknownKey),
			"fail_fast only works with across on the same step, or inside an in_parallel block")
	case len(owners) > 0:
		decoder.reportStructureError(position(pair.Key),
			fmt.Sprintf("%s step has no field %q", kind, unknownKey),
			fmt.Sprintf("%s belongs to %s steps", unknownKey, strings.Join(owners, ", ")))
	default:
		knownKeys := append(stepKindKeys(kind), stepModifierFields.keys()...)
		if slices.Contains(stepKindsWithTags, kind) {
			knownKeys = append(knownKeys, "tags")
		}
		decoder.reportStructureError(position(pair.Key),
			fmt.Sprintf("unknown field %q in %s step", unknownKey, kind),
			didYouMean(unknownKey, knownKeys))
		decoder.suggest(unknownKey, knownKeys)
	}
}

// checkRequiredStepFields reports what a step of one kind needs: a name for
// the kinds that have one, a file for set_pipeline and load_var, a type for
// run, and file: or config: (not both) for task (checked with fly).
func (decoder *pipelineDecoder) checkRequiredStepFields(step *model.Step, pairs []*ast.MappingValueNode, kind string) {
	objectName := kind + " step"
	switch kind {
	case "get", "put", "task", "set_pipeline", "load_var":
		decoder.requireField(pairs, kind, "name", objectName, step.Position)
	}
	switch kind {
	case "set_pipeline", "load_var":
		decoder.requireField(pairs, "file", "file", objectName, step.Position)
	case "run":
		decoder.requireField(pairs, "type", "type", objectName, step.Position)
	case "task":
		hasFile, hasConfig := hasKey(pairs, "file"), hasKey(pairs, "config")
		if !hasFile && !hasConfig {
			decoder.reportStructureError(step.Position, "task step needs file: or config:",
				"file: points to a task config in an artifact, config: defines it inline")
		}
		if hasFile && hasConfig {
			decoder.reportStructureError(step.Position, "task step must have either file: or config:, not both",
				"remove one of them")
		}
	}
}
