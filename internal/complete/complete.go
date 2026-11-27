// Package complete answers completion in the editor: keys of
// the object at the cursor, names of resources, jobs and resource types,
// keywords and the artifacts available at a step. The place at the cursor
// comes from the text, the keys from the parser's field tables, the names
// from a model (in the editor the last one that could be read).
package complete

import (
	"regexp"
	"slices"

	"github.com/mfast99/concourse-lint/internal/flow"
	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/navigate"
	"github.com/mfast99/concourse-lint/internal/parse"
)

// Kind says what an item is.
type Kind string

const (
	KindField        Kind = "field"
	KindKeyword      Kind = "keyword"
	KindResource     Kind = "resource"
	KindJob          Kind = "job"
	KindResourceType Kind = "resource_type"
	KindArtifact     Kind = "artifact"
)

// Item is one completion.
type Item struct {
	// What the editor lists, e.g. "trigger" or "repo".
	Label string
	Kind  Kind
	// What replaces the word at the cursor, e.g. "trigger: ".
	Insert string
}

// Result are the items and where the word they replace starts (it ends at
// the cursor).
type Result struct {
	Start model.Position
	Items []Item
}

var lineBreaks = regexp.MustCompile(`\r\n|\r|\n`)

// At returns the completions at the cursor (line and column from 1, the
// column in characters). names may be nil; then no names are offered.
func At(source []byte, cursor model.Position, names *model.Pipeline) Result {
	lines := lineBreaks.Split(string(source), -1)
	found, ok := placeAt(lines, cursor.Line, cursor.Column)
	if !ok {
		return Result{}
	}
	result := Result{Start: model.Position{Line: cursor.Line, Column: found.wordStart}}
	if names == nil {
		names = &model.Pipeline{}
	}
	context := completion{place: found, names: names, cursor: cursor}
	if found.atKey && isScalarListItem(found.path) {
		context.asListValue()
	}
	if context.place.atKey {
		result.Items = context.keys()
	} else {
		result.Items = context.values()
	}
	slices.SortFunc(result.Items, func(first, second Item) int {
		if first.Label < second.Label {
			return -1
		}
		if first.Label > second.Label {
			return 1
		}
		return 0
	})
	return result
}

// completion holds what the items are computed from.
type completion struct {
	place  place
	names  *model.Pipeline
	cursor model.Position
}

// child is what a key of an object holds: an object, or a list of them.
type child struct {
	object string
	isList bool
}

// hooks are the keys that hold a step, on jobs and on steps.
var hooks = []string{"on_success", "on_failure", "on_abort", "on_error", "ensure"}

// schema maps an object and a key to what the key holds. Keys that are not
// here hold free-form values (source:, params:, ...) without fixed keys.
var schema = map[string]map[string]child{
	"pipeline": {
		"jobs": {"job", true}, "resources": {"resource", true}, "resource_types": {"resource_type", true},
		"prototypes": {"resource_type", true}, "groups": {"group", true}, "var_sources": {"var_source", true},
		"display": {"display", false},
	},
	"job":         {"plan": {"step", true}, "build_log_retention": {"build_log_retention", false}},
	"step":        {"across": {"across_variable", true}, "do": {"step", true}, "try": {"step", false}, "config": {"task_config", false}},
	"in_parallel": {"steps": {"step", true}},
	"task_config": {
		"image_resource": {"image_resource", false}, "run": {"task_run", false},
		"inputs": {"task_input", true}, "outputs": {"task_output", true}, "caches": {"task_cache", true},
	},
}

// scalarLists are the keys that hold a list of names or words, not of
// objects.
var scalarLists = map[string][]string{
	"step":          {"passed", "inputs", "tags"},
	"group":         {"jobs", "resources"},
	"job":           {"tags", "serial_groups"},
	"resource":      {"tags"},
	"resource_type": {"tags"},
}

func childOf(object, key string, nextIsItem bool) (child, bool) {
	if (object == "job" || object == "step") && slices.Contains(hooks, key) {
		return child{"step", false}, true
	}
	if object == "step" && key == "in_parallel" {
		if nextIsItem {
			return child{"step", true}, true
		}
		return child{"in_parallel", false}, true
	}
	found, known := schema[object][key]
	return found, known
}

// objectAt returns the object of the mapping at the end of the path, or
// false for a free-form value.
func objectAt(path []segment) (string, bool) {
	object := "pipeline"
	for index := 0; index < len(path); index++ {
		if path[index].isItem {
			return "", false
		}
		nextIsItem := index+1 < len(path) && path[index+1].isItem
		found, known := childOf(object, path[index].key, nextIsItem)
		if !known || found.isList != nextIsItem {
			return "", false
		}
		object = found.object
		if found.isList {
			index++
		}
	}
	return object, true
}

// isScalarListItem reports whether the path ends with an item of a list of
// names, e.g. "- build" under passed:.
func isScalarListItem(path []segment) bool {
	if len(path) < 2 || !path[len(path)-1].isItem || path[len(path)-2].isItem {
		return false
	}
	container, known := objectAt(path[:len(path)-2])
	return known && slices.Contains(scalarLists[container], path[len(path)-2].key)
}

// asListValue turns "- word" under a list of names into the value of that
// list.
func (context *completion) asListValue() {
	path := context.place.path
	context.place.valueKey = path[len(path)-2].key
	context.place.path = path[:len(path)-2]
	context.place.atKey = false
	context.place.inList = true
}

// keys returns the keys of the mapping at the cursor that are not there yet.
func (context *completion) keys() []Item {
	object, known := objectAt(context.place.path)
	if !known {
		return nil
	}
	if object == "step" {
		if kind := context.stepKind(); kind != "" {
			object = "step:" + kind
		}
	}
	var items []Item
	for _, key := range parse.Fields(object) {
		if !slices.Contains(context.place.siblings, key) {
			items = append(items, Item{Label: key, Kind: KindField, Insert: key + ": "})
		}
	}
	return items
}

// stepKind returns the kind of the step at the cursor from its keys.
func (context *completion) stepKind() string {
	for _, key := range context.place.siblings {
		if slices.Contains(parse.StepKinds(), key) {
			return key
		}
	}
	return ""
}

// values returns what fits as the value of the key at the cursor.
func (context *completion) values() []Item {
	path := context.place.path
	key := context.place.valueKey
	// input_mapping: {name: artifact} or a block mapping below it.
	if key == "input_mapping" && context.place.inFlowMapping || len(path) > 0 && path[len(path)-1].key == "input_mapping" {
		return context.artifacts("")
	}
	object, known := objectAt(path)
	if !known {
		return nil
	}
	switch object {
	case "step":
		return context.stepValues(key)
	case "group":
		switch key {
		case "jobs":
			return context.jobs(false)
		case "resources":
			return context.resources()
		}
	case "resource", "resource_type", "image_resource":
		if key == "type" {
			return context.resourceTypes()
		}
	}
	return nil
}

func (context *completion) stepValues(key string) []Item {
	kind := context.stepKind()
	switch {
	case key == "get" || key == "put" || key == "resource":
		return context.resources()
	case key == "passed":
		return context.jobs(true)
	case key == "inputs" && context.place.inList:
		return context.artifacts("")
	case key == "file":
		return context.artifacts("/")
	case key == "image":
		return context.artifacts("")
	}
	var items []Item
	for _, keyword := range parse.Keywords(kind, key) {
		items = append(items, Item{Label: keyword, Kind: KindKeyword, Insert: keyword})
	}
	return items
}

func (context *completion) resources() []Item {
	return namedItems(navigate.ResourceNames(context.names), KindResource)
}

// jobs returns the job names, without the job at the cursor for passed:
// (a job cannot wait for itself).
func (context *completion) jobs(withoutOwnJob bool) []Item {
	names := navigate.JobNames(context.names)
	if job := context.jobAtCursor(); withoutOwnJob && job != nil && job.Name.Set {
		names = slices.DeleteFunc(slices.Clone(names), func(name string) bool { return name == job.Name.Value })
	}
	return namedItems(names, KindJob)
}

// resourceTypes returns the resource types of the pipeline and the
// built-in ones.
func (context *completion) resourceTypes() []Item {
	var names []string
	for _, resourceType := range context.names.ResourceTypes {
		if resourceType != nil && resourceType.Name.Set {
			names = append(names, resourceType.Name.Value)
		}
	}
	return namedItems(append(names, navigate.BuiltinResourceTypes...), KindResourceType)
}

// artifacts returns the artifacts available when the step at the cursor
// starts, each followed by suffix (e.g. "/" for a file path).
func (context *completion) artifacts(suffix string) []Item {
	job := context.jobAtCursor()
	if job == nil {
		return nil
	}
	// The step that starts last at or above the cursor is the one the
	// cursor is in (steps are visited depth first, nested ones after their
	// parent).
	var scope flow.Scope
	found, startLine := false, 0
	for step, stepScope := range flow.Steps(job) {
		if step.Position.Line <= context.cursor.Line && step.Position.Line >= startLine {
			scope, found, startLine = stepScope, true, step.Position.Line
		}
	}
	if !found {
		return nil
	}
	var items []Item
	for _, name := range scope.ArtifactNames() {
		items = append(items, Item{Label: name + suffix, Kind: KindArtifact, Insert: name + suffix})
	}
	return items
}

// jobAtCursor returns the last job that starts at or above the cursor.
func (context *completion) jobAtCursor() *model.Job {
	var found *model.Job
	for _, job := range context.names.Jobs {
		if job != nil && job.Position.Line <= context.cursor.Line {
			found = job
		}
	}
	return found
}

func namedItems(names []string, kind Kind) []Item {
	var items []Item
	for _, name := range names {
		if !slices.ContainsFunc(items, func(item Item) bool { return item.Label == name }) {
			items = append(items, Item{Label: name, Kind: kind, Insert: name})
		}
	}
	return items
}
