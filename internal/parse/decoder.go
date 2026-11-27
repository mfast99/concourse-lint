package parse

import (
	"fmt"

	"github.com/mfast99/go-yaml/ast"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// pipelineDecoder walks the goccy AST and builds the model. It never stops at
// a structural error: it reports CL001, leaves the affected field unset and
// carries on, so that the rules still see as much of the pipeline as possible.
type pipelineDecoder struct {
	findings []analysis.Finding
	// Last line of the file, for findings about something missing.
	lastLine int
	// The value every alias stands for (from resolveAliases).
	aliasTargets map[*ast.AliasNode]ast.Node
	// How many steps the decoder is inside right now. Inside a step fly is
	// stricter about types.
	stepDepth int
}

// suggest attaches the closest candidate as the suggestion to the finding
// just reported, whose position covers exactly the word (a "did you mean"
// quick fix in the editor).
func (decoder *pipelineDecoder) suggest(word string, candidates []string) {
	closest, found := analysis.ClosestName(word, candidates)
	if found && len(decoder.findings) > 0 {
		decoder.findings[len(decoder.findings)-1].Suggestion = closest
	}
}

// reportStructureError reports a CL001 finding. CL001 comes from the decoder,
// not from a rule in internal/rules: values of the wrong type and unknown
// keys never reach the model, so only the decoder
// still sees them.
func (decoder *pipelineDecoder) reportStructureError(location model.Position, message, hint string) {
	decoder.report("CL001", analysis.SeverityError, location, message, hint)
}

// reportIgnoredField reports a CL018 finding: a key that Concourse ignores
// without a word. Like CL001 it comes from the
// decoder, because only the decoder sees unknown keys.
func (decoder *pipelineDecoder) reportIgnoredField(location model.Position, message, hint string) {
	decoder.report("CL018", analysis.SeverityWarning, location, message, hint)
}

// reportChangedValue reports a CL017 finding: an unquoted value that
// Concourse passes on in another spelling. The
// parser reports it for string fields; the rule for free-form blocks follows
// in M2.
func (decoder *pipelineDecoder) reportChangedValue(location model.Position, message, hint string) {
	decoder.report("CL017", analysis.SeverityWarning, location, message, hint)
}

// reportRuntimeFailure reports a CL019 finding: a value that fly accepts but
// that fails every time the step or check runs.
func (decoder *pipelineDecoder) reportRuntimeFailure(location model.Position, message, hint string) {
	decoder.report("CL019", analysis.SeverityError, location, message, hint)
}

// reportIgnoredValue reports a CL020 finding: a word that Concourse ignores
// without a word of its own.
func (decoder *pipelineDecoder) reportIgnoredValue(location model.Position, message, hint string) {
	decoder.report("CL020", analysis.SeverityWarning, location, message, hint)
}

func (decoder *pipelineDecoder) report(rule string, severity analysis.Severity, location model.Position, message, hint string) {
	decoder.findings = append(decoder.findings, analysis.Finding{
		Rule:     rule,
		Severity: severity,
		Position: location,
		Message:  message,
		Hint:     hint,
	})
}

// decodeList decodes a field that holds a list of objects, e.g. resources:.
// A value that is no list is a CL001 (except an empty one or ((var))). Go
// methods cannot have type parameters, so this is a function that takes the
// decoder.
func decodeList[T any](decoder *pipelineDecoder, pair *ast.MappingValueNode, itemName string, decodeItem func(ast.Node) *T) []*T {
	items, isList := decoder.sequence(pair.Value)
	if !isList {
		if !decoder.isUnknownValue(pair.Value) {
			decoder.reportWrongType(pair, "a list", "")
		}
		return nil
	}
	return decodeItems(decoder, items, itemName, decodeItem)
}

// decodeItems decodes every item of a list with decodeItem. An item that is
// no mapping is a CL001 (an empty item is left out without a finding).
func decodeItems[T any](decoder *pipelineDecoder, items []ast.Node, itemName string, decodeItem func(ast.Node) *T) []*T {
	var decoded []*T
	for _, item := range items {
		if _, isMapping := decoder.mapping(item); !isMapping {
			if !decoder.isUnknownValue(item) {
				actual := decoder.resolve(item)
				decoder.reportStructureError(position(actual),
					fmt.Sprintf("%s must be a mapping, not %s", itemName, describeValue(actual)), "")
			}
			continue
		}
		if result := decodeItem(item); result != nil {
			decoded = append(decoded, result)
		}
	}
	return decoded
}

// resolve returns the node that carries the content: the value behind an
// anchor (&name), a tag (!!str) or an alias (*name). Aliased content is
// decoded again at every use and keeps the positions of its definition.
func (decoder *pipelineDecoder) resolve(node ast.Node) ast.Node {
	switch typed := node.(type) {
	case *ast.AnchorNode:
		return decoder.resolve(typed.Value)
	case *ast.TagNode:
		return decoder.resolve(typed.Value)
	case *ast.AliasNode:
		return decoder.resolve(decoder.aliasTargets[typed])
	}
	return node
}

// mapping returns the key/value pairs of a mapping node, with merge keys
// (<<) applied.
func (decoder *pipelineDecoder) mapping(node ast.Node) ([]*ast.MappingValueNode, bool) {
	pairs, isMapping := decoder.pairsAsWritten(node)
	if !isMapping {
		return nil, false
	}
	return decoder.applyMerges(pairs), true
}

// pairsAsWritten returns the key/value pairs as they stand in the file,
// merge keys (<<) included as they are.
func (decoder *pipelineDecoder) pairsAsWritten(node ast.Node) ([]*ast.MappingValueNode, bool) {
	switch typed := decoder.resolve(node).(type) {
	case *ast.MappingNode:
		return typed.Values, true
	// A mapping with a single pair.
	case *ast.MappingValueNode:
		return []*ast.MappingValueNode{typed}, true
	}
	return nil, false
}

// applyMerges replaces every merge key (<<) with the pairs it merges, the
// way Concourse (fly 8.3.4) does it, not the YAML specification: of two equal
// keys the one further down in the file wins,
// like {...defaults, name: "x"} in JavaScript; in "<<: [*a, *b]" the first
// anchor wins. A key keeps the place where it first appeared.
func (decoder *pipelineDecoder) applyMerges(pairs []*ast.MappingValueNode) []*ast.MappingValueNode {
	hasMerge := false
	for _, pair := range pairs {
		if isMergeKey(pair) {
			hasMerge = true
		}
	}
	if !hasMerge {
		return pairs
	}
	var merged []*ast.MappingValueNode
	placeOfKey := map[string]int{}
	set := func(pair *ast.MappingValueNode) {
		if place, seen := placeOfKey[key(pair)]; seen {
			merged[place] = pair
			return
		}
		placeOfKey[key(pair)] = len(merged)
		merged = append(merged, pair)
	}
	for _, pair := range pairs {
		if !isMergeKey(pair) {
			set(pair)
			continue
		}
		for _, source := range decoder.mergeSources(pair.Value) {
			sourcePairs, _ := decoder.mapping(source)
			for _, sourcePair := range sourcePairs {
				set(sourcePair)
			}
		}
	}
	return merged
}

// mergeSources returns the mappings a merge key merges, in the order in which
// they are applied: for a list the last one first, so that the first one
// wins.
func (decoder *pipelineDecoder) mergeSources(value ast.Node) []ast.Node {
	items, isList := decoder.sequence(value)
	if !isList {
		return []ast.Node{value}
	}
	sources := make([]ast.Node, 0, len(items))
	for index := len(items) - 1; index >= 0; index-- {
		sources = append(sources, items[index])
	}
	return sources
}

func isMergeKey(pair *ast.MappingValueNode) bool {
	_, isMerge := pair.Key.(*ast.MergeKeyNode)
	return isMerge
}

// sequence returns the items of a list node.
func (decoder *pipelineDecoder) sequence(node ast.Node) ([]ast.Node, bool) {
	if list, isList := decoder.resolve(node).(*ast.SequenceNode); isList {
		return list.Values, true
	}
	return nil, false
}

// key returns the text of a mapping key.
func key(pair *ast.MappingValueNode) string {
	return pair.Key.GetToken().Value
}

// position returns where a node starts.
func position(node ast.Node) model.Position {
	if node == nil || node.GetToken() == nil {
		return model.Position{Line: 1, Column: 1}
	}
	return tokenPosition(node.GetToken())
}

// elementPosition returns the first line of a list element or block: its
// first key as written (also when that is a merge key). goccy points at the
// colon of the first key, so the key itself is used instead.
func (decoder *pipelineDecoder) elementPosition(node ast.Node) model.Position {
	if pairs, isMapping := decoder.pairsAsWritten(node); isMapping && len(pairs) > 0 {
		return position(pairs[0].Key)
	}
	return position(decoder.resolve(node))
}
