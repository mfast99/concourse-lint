package parse

import (
	"fmt"
	"maps"
	"slices"

	"github.com/mfast99/go-yaml/ast"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// fieldDecoder decodes the value of one key into the object the key belongs
// to.
type fieldDecoder[T any] func(decoder *pipelineDecoder, target *T, pair *ast.MappingValueNode)

// fieldTable lists every key an object can have, each with its decoder. It
// is the one place that knows the keys of an object: the decoder uses it, and
// so do the check for unknown keys, the "did
// you mean" hints and, later, completion in the editor.
type fieldTable[T any] map[string]fieldDecoder[T]

// keys returns the keys of the table in alphabetical order.
func (table fieldTable[T]) keys() []string {
	return slices.Sorted(maps.Keys(table))
}

// decodeFields decodes every pair whose key is in the table. Any other key is
// an unknown field that Concourse ignores: CL018.
// objectName names the object in the message, e.g. "resource".
func decodeFields[T any](decoder *pipelineDecoder, target *T, pairs []*ast.MappingValueNode, table fieldTable[T], objectName string) {
	for _, pair := range pairs {
		if !decodeField(decoder, target, pair, table) {
			decoder.reportIgnoredField(position(pair.Key),
				fmt.Sprintf("unknown field %q in %s, which Concourse ignores", key(pair), objectName),
				didYouMean(key(pair), table.keys()))
			decoder.suggest(key(pair), table.keys())
		}
	}
}

// decodeFieldsStrictly is decodeFields for the objects where Concourse
// rejects unknown fields (an inline task config, an across variable): CL001.
func decodeFieldsStrictly[T any](decoder *pipelineDecoder, target *T, pairs []*ast.MappingValueNode, table fieldTable[T], objectName string) {
	for _, pair := range pairs {
		if !decodeField(decoder, target, pair, table) {
			decoder.reportStructureError(position(pair.Key),
				fmt.Sprintf("unknown field %q in %s", key(pair), objectName),
				didYouMean(key(pair), table.keys()))
			decoder.suggest(key(pair), table.keys())
		}
	}
}

// decodeField decodes one pair if its key is in the table, and reports
// whether it was.
func decodeField[T any](decoder *pipelineDecoder, target *T, pair *ast.MappingValueNode, table fieldTable[T]) bool {
	decodeValue, known := table[key(pair)]
	if known {
		decodeValue(decoder, target, pair)
	}
	return known
}

// didYouMean is the hint for an unknown key: the closest known key, or
// nothing if none is close.
func didYouMean(unknownKey string, knownKeys []string) string {
	if closest, found := analysis.ClosestName(unknownKey, knownKeys); found {
		return fmt.Sprintf("did you mean %q?", closest)
	}
	return ""
}

// withHooks adds the hook keys (on_success, on_failure, on_abort, on_error,
// ensure) to the table of an object that embeds model.Hooks: jobs and steps.
func withHooks[T any](table fieldTable[T], hooksOf func(target *T) *model.Hooks) fieldTable[T] {
	hookSetters := map[string]func(hooks *model.Hooks, hook *model.Step){
		"on_success": func(hooks *model.Hooks, hook *model.Step) { hooks.OnSuccess = hook },
		"on_failure": func(hooks *model.Hooks, hook *model.Step) { hooks.OnFailure = hook },
		"on_abort":   func(hooks *model.Hooks, hook *model.Step) { hooks.OnAbort = hook },
		"on_error":   func(hooks *model.Hooks, hook *model.Step) { hooks.OnError = hook },
		"ensure":     func(hooks *model.Hooks, hook *model.Step) { hooks.Ensure = hook },
	}
	for name, setHook := range hookSetters {
		table[name] = func(decoder *pipelineDecoder, target *T, pair *ast.MappingValueNode) {
			setHook(hooksOf(target), objectField(decoder, pair, decoder.step))
		}
	}
	return table
}

// requireField reports a required field that is missing or empty: CL001 at
// the first line of the element. A field with
// another value is present: ((var)) is fine, and a wrong type is already
// reported where the value is read.
func (decoder *pipelineDecoder) requireField(pairs []*ast.MappingValueNode, fieldKey, fieldLabel, objectName string, location model.Position) {
	if !decoder.isMissingOrEmpty(pairs, fieldKey) {
		return
	}
	decoder.reportStructureError(location, fmt.Sprintf("%s needs a %s", objectName, fieldLabel), fmt.Sprintf("add %s:", fieldKey))
}

// isMissingOrEmpty reports whether the key is absent, or present without a
// value or with an empty string.
func (decoder *pipelineDecoder) isMissingOrEmpty(pairs []*ast.MappingValueNode, fieldKey string) bool {
	for _, pair := range pairs {
		if key(pair) != fieldKey {
			continue
		}
		switch typed := decoder.resolve(pair.Value).(type) {
		case nil, *ast.NullNode:
			return true
		case *ast.StringNode:
			return typed.Value == ""
		}
		return false
	}
	return true
}
