package parse

import (
	"slices"
	"strings"
)

// The keys the parser knows, for completion in the editor.
// They come from the same field tables the decoder uses, so the editor offers
// exactly what the linter accepts.

// fieldKeys maps an object name to the keys of its field table.
var fieldKeys = map[string]func() []string{
	"pipeline":            pipelineFields.keys,
	"resource":            resourceFields.keys,
	"resource_type":       resourceTypeFields.keys,
	"job":                 jobFields.keys,
	"group":               groupFields.keys,
	"var_source":          varSourceFields.keys,
	"display":             displayFields.keys,
	"build_log_retention": buildLogRetentionFields.keys,
	"in_parallel":         inParallelFields.keys,
	"across_variable":     acrossVariableFields.keys,
	"task_config":         taskConfigFields.keys,
	"image_resource":      imageResourceFields.keys,
	"task_run":            taskRunFields.keys,
	"task_input":          taskInputFields.keys,
	"task_output":         taskOutputFields.keys,
	"task_cache":          taskCacheFields.keys,
}

// Fields returns the keys of an object, sorted. Objects: "pipeline",
// "resource", "resource_type", "job", "group", "var_source", "display",
// "build_log_retention", "in_parallel", "across_variable", "task_config",
// "image_resource", "task_run", "task_input", "task_output", "task_cache";
// "step:<kind>" (e.g. "step:get") for a step of that kind: its kind key,
// its fields and the modifiers; "step" for a step without a kind yet: the
// step kinds and the modifiers. An unknown object has no keys.
func Fields(object string) []string {
	if keysOf, known := fieldKeys[object]; known {
		return keysOf()
	}
	if object == "step" {
		return sortedUnion(stepKindFields.keys(), stepModifierFields.keys())
	}
	kind, isStep := strings.CutPrefix(object, "step:")
	if !isStep || !slices.Contains(stepKindFields.keys(), kind) {
		return nil
	}
	keys := sortedUnion([]string{kind}, stepKindKeys(kind), stepModifierFields.keys())
	if slices.Contains(stepKindsWithTags, kind) {
		keys = sortedUnion(keys, []string{"tags"})
	}
	return keys
}

// StepKinds returns the keys that set the kind of a step, sorted.
func StepKinds() []string {
	return stepKindFields.keys()
}

// Keywords returns the words a field accepts besides other values:
// "version" of a "get", "inputs" of a "put", "format" of a "load_var".
func Keywords(stepKind, key string) []string {
	switch stepKind + "." + key {
	case "get.version":
		return slices.Clone(getVersionKeywords)
	case "put.inputs":
		return slices.Clone(putInputsKeywords)
	case "load_var.format":
		return slices.Clone(loadVarFormats)
	}
	return nil
}

func sortedUnion(lists ...[]string) []string {
	var union []string
	for _, list := range lists {
		for _, key := range list {
			if !slices.Contains(union, key) {
				union = append(union, key)
			}
		}
	}
	slices.Sort(union)
	return union
}
