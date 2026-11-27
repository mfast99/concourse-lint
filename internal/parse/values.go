package parse

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/mfast99/go-yaml/ast"
	"github.com/mfast99/go-yaml/token"

	"github.com/mfast99/concourse-lint/internal/model"
)

// isUnknownValue reports whether a value is empty (null) or contains
// ((var)). Neither is a wrong type: fly fills in variables before it reads
// the types, and an empty value means "not set".
func (decoder *pipelineDecoder) isUnknownValue(node ast.Node) bool {
	switch typed := decoder.resolve(node).(type) {
	case nil, *ast.NullNode:
		return true
	case *ast.StringNode:
		return isVariable(typed.Value)
	}
	return false
}

// stringValue reads a string value.
func (decoder *pipelineDecoder) stringValue(pair *ast.MappingValueNode) model.Value[string] {
	value := model.Value[string]{KeyPosition: position(pair.Key)}
	switch typed := decoder.resolve(pair.Value).(type) {
	case nil, *ast.NullNode:
	// A block scalar (| or >).
	case *ast.LiteralNode:
		value.Value, value.Position, value.Set = typed.Value.Value, position(typed), true
	case *ast.MappingNode, *ast.MappingValueNode, *ast.SequenceNode:
		decoder.reportWrongType(pair, "a string", "")
	default:
		text, isSet := decoder.scalarInStringField(typed, key(pair), func(hint string) {
			decoder.reportWrongType(pair, "a string", hint)
		})
		value.Value, value.Position, value.Set = text, position(typed), isSet
	}
	return value
}

// stringItem reads a string in a list, e.g. an entry of passed:.
func (decoder *pipelineDecoder) stringItem(node ast.Node, listName string) model.Value[string] {
	var value model.Value[string]
	report := func(hint string) {
		actual := decoder.resolve(node)
		decoder.reportStructureError(position(actual),
			fmt.Sprintf("entries of %s must be strings, not %s", listName, describeValue(actual)), hint)
	}
	switch typed := decoder.resolve(node).(type) {
	case nil, *ast.NullNode:
	case *ast.LiteralNode:
		value.Value, value.Position, value.Set = typed.Value.Value, position(typed), true
	case *ast.MappingNode, *ast.MappingValueNode, *ast.SequenceNode:
		report("")
	default:
		text, isSet := decoder.scalarInStringField(typed, listName, report)
		value.Value, value.Position, value.Set = text, position(typed), isSet
	}
	return value
}

// scalarInStringField reads a scalar written in a string field and returns
// the text the field gets, and whether it gets one. A quoted value is text
// as written. An unquoted one is read like YAML 1.1 does: inside a step a
// number or boolean is a wrong type, because
// fly rejects it (CL001, reported with reportWrongType); outside steps
// Concourse turns it into text, and if that text differs from what is
// written, it is CL017 (`name: 1.10` becomes "1.1"). Infinity and NaN fail
// everywhere.
func (decoder *pipelineDecoder) scalarInStringField(node ast.Node, fieldName string, reportWrongType func(hint string)) (string, bool) {
	if decoder.reportInfinity(node) {
		return "", false
	}
	scalarToken := node.GetToken()
	if isQuoted(scalarToken) {
		return scalarToken.Value, true
	}
	text := scalarToken.Value
	scalar := model.ResolveYAML11(text)
	switch scalar.Kind {
	case model.StringScalar:
		return text, true
	case model.NullScalar:
		return "", false
	}
	if decoder.stepDepth > 0 {
		reportWrongType(fmt.Sprintf("quote it: %q", text))
		return "", false
	}
	converted := scalar.AsString()
	if converted != text {
		decoder.reportChangedValue(position(node),
			fmt.Sprintf("unquoted %s in %s becomes %q", text, fieldName, converted),
			fmt.Sprintf("quote it: %q", text))
	}
	return converted, true
}

// reportInfinity reports .inf and .nan, which make fly fail on the whole
// file, and says whether the node was one.
func (decoder *pipelineDecoder) reportInfinity(node ast.Node) bool {
	switch node.(type) {
	case *ast.InfinityNode, *ast.NanNode:
		text := node.GetToken().Value
		decoder.reportStructureError(position(node),
			fmt.Sprintf("%s is not allowed: Concourse cannot pass on infinity or NaN", text),
			fmt.Sprintf("quote it: %q", text))
		return true
	}
	return false
}

// boolValue reads a boolean the way Concourse does: an unquoted value from
// the YAML 1.1 table (yes, on, y, true, …). An empty
// value and ((var)) leave it unset without a finding;
// anything else is a CL001 wrong type.
func (decoder *pipelineDecoder) boolValue(pair *ast.MappingValueNode) model.Value[bool] {
	value := model.Value[bool]{KeyPosition: position(pair.Key)}
	if decoder.isUnknownValue(pair.Value) {
		return value
	}
	switch typed := decoder.resolve(pair.Value).(type) {
	case *ast.BoolNode, *ast.StringNode:
		scalarToken := typed.GetToken()
		if boolean, isBoolean := model.YAML11Boolean(scalarToken.Value); isBoolean && !isQuoted(scalarToken) {
			value.Value, value.Position, value.Set = boolean, position(typed), true
			return value
		}
	}
	decoder.reportWrongType(pair, "true or false", decoder.booleanHint(pair.Value))
	return value
}

// booleanHint suggests how to fix a value that is no boolean: a quoted
// boolean only needs its quotes removed.
func (decoder *pipelineDecoder) booleanHint(node ast.Node) string {
	if stringNode, isString := decoder.resolve(node).(*ast.StringNode); isString && isQuoted(stringNode.GetToken()) {
		if _, isBoolean := model.YAML11Boolean(stringNode.Value); isBoolean {
			return "remove the quotes: " + stringNode.Value
		}
	}
	return "use true or false"
}

// intValue reads a whole number. fly also accepts it written with a zero
// fraction, like 2.0.
func (decoder *pipelineDecoder) intValue(pair *ast.MappingValueNode) model.Value[int] {
	value := model.Value[int]{KeyPosition: position(pair.Key)}
	if decoder.isUnknownValue(pair.Value) {
		return value
	}
	switch typed := decoder.resolve(pair.Value).(type) {
	case *ast.IntegerNode:
		switch number := typed.Value.(type) {
		case int64:
			value.Value, value.Position, value.Set = int(number), position(typed), true
			return value
		case uint64:
			value.Value, value.Position, value.Set = int(number), position(typed), true
			return value
		}
	case *ast.FloatNode:
		if typed.Value == math.Trunc(typed.Value) {
			value.Value, value.Position, value.Set = int(typed.Value), position(typed), true
			return value
		}
	}
	decoder.reportWrongType(pair, "a whole number", "")
	return value
}

// stringList reads a list of strings, e.g. passed: [unit, lint].
func (decoder *pipelineDecoder) stringList(pair *ast.MappingValueNode) []model.Value[string] {
	items, isList := decoder.sequence(pair.Value)
	if !isList {
		if !decoder.isUnknownValue(pair.Value) {
			decoder.reportWrongType(pair, "a list", decoder.listHint(pair.Value))
		}
		return nil
	}
	var values []model.Value[string]
	for _, item := range items {
		if value := decoder.stringItem(item, key(pair)); value.Set {
			values = append(values, value)
		}
	}
	return values
}

// listHint suggests how to write a single value as a list.
func (decoder *pipelineDecoder) listHint(node ast.Node) string {
	if stringNode, isString := decoder.resolve(node).(*ast.StringNode); isString {
		return fmt.Sprintf("use a list: [%s]", stringNode.Value)
	}
	return ""
}

// artifactMappings reads input_mapping or output_mapping.
func (decoder *pipelineDecoder) artifactMappings(pair *ast.MappingValueNode) []*model.ArtifactMapping {
	if !decoder.expectMapping(pair) {
		return nil
	}
	pairs, _ := decoder.mapping(pair.Value)
	var mappings []*model.ArtifactMapping
	for _, entry := range pairs {
		keyPosition := position(entry.Key)
		mappings = append(mappings, &model.ArtifactMapping{
			TaskName: model.Value[string]{Value: key(entry), Position: keyPosition, KeyPosition: keyPosition, Set: true},
			PlanName: decoder.stringValue(entry),
		})
	}
	return mappings
}

// expectMapping reports whether a field holds a mapping. Any other value,
// except an empty one or ((var)), is a CL001 wrong type.
func (decoder *pipelineDecoder) expectMapping(pair *ast.MappingValueNode) bool {
	if _, isMapping := decoder.mapping(pair.Value); isMapping {
		return true
	}
	if !decoder.isUnknownValue(pair.Value) {
		decoder.reportWrongType(pair, "a mapping", "")
	}
	return false
}

// objectField decodes a field that holds one object, e.g. config: of a task
// or a hook.
func objectField[T any](decoder *pipelineDecoder, pair *ast.MappingValueNode, decodeObject func(ast.Node) *T) *T {
	if !decoder.expectMapping(pair) {
		return nil
	}
	return decodeObject(pair.Value)
}

// freeMapping reads a free-form field that Concourse expects to be a mapping
// (source:, params:, vars:, …).
func (decoder *pipelineDecoder) freeMapping(pair *ast.MappingValueNode) *model.Node {
	if !decoder.expectMapping(pair) {
		return nil
	}
	return decoder.freeNode(pair.Value)
}

// freeNode reads a free-form value as a tree. An empty value (null) gives
// nil.
func (decoder *pipelineDecoder) freeNode(node ast.Node) *model.Node {
	node = decoder.resolve(node)
	switch typed := node.(type) {
	case nil, *ast.NullNode:
		return nil
	case *ast.MappingNode, *ast.MappingValueNode:
		pairs, _ := decoder.mapping(typed)
		result := &model.Node{Kind: model.MappingNode, Position: decoder.collectionStart(typed)}
		for _, entry := range pairs {
			result.Pairs = append(result.Pairs, &model.NodePair{
				Key:         key(entry),
				KeyPosition: position(entry.Key),
				Value:       decoder.freeNode(entry.Value),
			})
		}
		return result
	case *ast.SequenceNode:
		result := &model.Node{Kind: model.ListNode, Position: position(typed)}
		for _, item := range typed.Values {
			result.Items = append(result.Items, decoder.freeNode(item))
		}
		return result
	case *ast.LiteralNode:
		return &model.Node{Kind: model.ScalarNode, Position: position(typed), Text: typed.Value.Value}
	}
	decoder.reportInfinity(node)
	scalarToken := node.GetToken()
	return &model.Node{
		Kind:     model.ScalarNode,
		Position: position(node),
		Text:     scalarToken.Value,
		Quoted:   isQuoted(scalarToken),
	}
}

// collectionStart returns where a mapping starts: the "{" of a flow mapping,
// or the first key of a block mapping (goccy points at its colon).
func (decoder *pipelineDecoder) collectionStart(node ast.Node) model.Position {
	if flowMapping, isMapping := node.(*ast.MappingNode); isMapping && flowMapping.IsFlowStyle {
		return position(flowMapping)
	}
	return decoder.elementPosition(node)
}

// reportWrongType reports a CL001 for a value of the wrong type: at the value,
// or at its key when the value is a block (spec/README.md, "Positions").
func (decoder *pipelineDecoder) reportWrongType(pair *ast.MappingValueNode, expected, hint string) {
	actual := decoder.resolve(pair.Value)
	location := position(actual)
	switch actual.(type) {
	case *ast.MappingNode, *ast.MappingValueNode, *ast.SequenceNode:
		location = position(pair.Key)
	}
	decoder.reportStructureError(location, key(pair)+" must be "+expected+", not "+describeValue(actual), hint)
}

// describeValue names a value for a message, e.g. `the string "sure"`.
func describeValue(node ast.Node) string {
	if node == nil {
		return "nothing"
	}
	switch typed := node.(type) {
	case *ast.MappingNode, *ast.MappingValueNode:
		return "a mapping"
	case *ast.SequenceNode:
		return "a list"
	case *ast.IntegerNode, *ast.FloatNode:
		return "the number " + typed.GetToken().Value
	case *ast.StringNode:
		// An unquoted yes or 1_000 is no string for Concourse.
		if !isQuoted(typed.GetToken()) && model.ResolveYAML11(typed.Value).Kind != model.StringScalar {
			return typed.Value
		}
		return `the string "` + typed.Value + `"`
	}
	return node.GetToken().Value
}

// isQuoted reports whether a scalar was written in quotes.
func isQuoted(scalarToken *token.Token) bool {
	return scalarToken.Type == token.DoubleQuoteType || scalarToken.Type == token.SingleQuoteType
}

// isVariable reports whether a value contains ((var)), which fly fills in
// from -v before it reads the types.
func isVariable(text string) bool {
	opening := strings.Index(text, "((")
	return opening >= 0 && strings.Contains(text[opening:], "))")
}

// durationValue reads a Go duration such as 10m or 1h30m, which fly checks
// for check_every and a cache ttl. allowNever also
// accepts "never" (check_every). An empty value means "not set".
func (decoder *pipelineDecoder) durationValue(pair *ast.MappingValueNode, allowNever bool) model.Value[string] {
	value := model.Value[string]{KeyPosition: position(pair.Key)}
	if decoder.isUnknownValue(pair.Value) {
		return value
	}
	expected := "a duration like 10m"
	if allowNever {
		expected += ` or "never"`
	}
	switch typed := decoder.resolve(pair.Value).(type) {
	case *ast.StringNode:
		text := typed.Value
		if text == "" {
			return value
		}
		if (allowNever && text == "never") || isDuration(text) {
			value.Value, value.Position, value.Set = text, position(typed), true
			return value
		}
		decoder.reportStructureError(position(typed),
			fmt.Sprintf("%s must be %s, not %q", key(pair), expected, text), durationHint(text))
	case *ast.IntegerNode, *ast.FloatNode:
		decoder.reportWrongType(pair, expected, fmt.Sprintf("add a unit, e.g. %sm", typed.GetToken().Value))
	default:
		decoder.reportWrongType(pair, expected, "")
	}
	return value
}

// isDuration reports whether text is a Go duration, as Concourse parses it.
func isDuration(text string) bool {
	_, err := time.ParseDuration(text)
	return err == nil
}

// durationHint explains the most common mistake: days, which Go durations
// do not have.
func durationHint(text string) string {
	if days, found := strings.CutSuffix(text, "d"); found && isDuration(days+"h") {
		return "Go durations have no days: use hours, e.g. 24h"
	}
	return "valid units are ns, us, ms, s, m and h, e.g. 1h30m"
}

// checkMaxInFlight checks max_in_flight of an across variable: a whole number
// or "all" (fly rejects anything else).
func (decoder *pipelineDecoder) checkMaxInFlight(pair *ast.MappingValueNode) {
	if decoder.isUnknownValue(pair.Value) {
		return
	}
	switch typed := decoder.resolve(pair.Value).(type) {
	case *ast.IntegerNode:
		return
	case *ast.FloatNode:
		if typed.Value == math.Trunc(typed.Value) {
			return
		}
	case *ast.StringNode:
		if typed.Value == "all" {
			return
		}
	}
	decoder.reportWrongType(pair, `a whole number or "all"`, "")
}

// checkVersion checks version of a get step: a string (latest, every) or a
// mapping of version fields whose values are strings (fly rejects anything
// else). Another word than latest or every is CL020.
func (decoder *pipelineDecoder) checkVersion(pair *ast.MappingValueNode) {
	if decoder.isUnknownValue(pair.Value) {
		return
	}
	switch decoder.resolve(pair.Value).(type) {
	case *ast.StringNode:
		decoder.checkKeyword(pair.Value, "version", getVersionKeywords, "takes the latest version")
		return
	case *ast.MappingNode, *ast.MappingValueNode:
		fields, _ := decoder.mapping(pair.Value)
		for _, field := range fields {
			if _, isString := decoder.resolve(field.Value).(*ast.StringNode); !isString && !decoder.isUnknownValue(field.Value) {
				decoder.reportWrongType(field, "a string", fmt.Sprintf("quote it: %q", field.Value.GetToken().Value))
			}
		}
		return
	}
	decoder.reportWrongType(pair, "latest, every or a mapping of version fields", "")
}

// runtimeDuration reads a duration that fly does not check (timeout,
// check_timeout), so that ((vars)) work. Concourse parses it when the step or
// check runs, so an invalid one is CL019.
// consequence says what fails, e.g. "every check fails".
func (decoder *pipelineDecoder) runtimeDuration(pair *ast.MappingValueNode, consequence string) model.Value[string] {
	value := decoder.stringValue(pair)
	if !value.Set || value.Value == "" || isVariable(value.Value) || isDuration(value.Value) {
		return value
	}
	decoder.reportRuntimeFailure(value.Position,
		fmt.Sprintf("%s must be a duration like 10m, not %q: %s", key(pair), value.Value, consequence),
		durationHint(value.Value))
	return value
}

// loadVarFormats are the formats load_var knows (atc/exec/load_var_step.go
// in Concourse 8.3.4).
var loadVarFormats = []string{"json", "yaml", "yml", "raw", "trim"}

// getVersionKeywords are the words version: of a get accepts (or a
// mapping of version fields).
var getVersionKeywords = []string{"latest", "every"}

// putInputsKeywords are the words inputs: of a put accepts (or a list of
// artifact names).
var putInputsKeywords = []string{"all", "detect"}

// loadVarFormat reads format of a load_var step. An unknown format is CL019:
// the step fails with "invalid format" every time it runs.
func (decoder *pipelineDecoder) loadVarFormat(pair *ast.MappingValueNode) model.Value[string] {
	value := decoder.stringValue(pair)
	if !value.Set || value.Value == "" || isVariable(value.Value) || slices.Contains(loadVarFormats, value.Value) {
		return value
	}
	decoder.reportRuntimeFailure(value.Position,
		fmt.Sprintf("format must be json, yaml, yml, raw or trim, not %q: load_var fails every time it runs", value.Value),
		didYouMean(value.Value, loadVarFormats))
	decoder.suggest(value.Value, loadVarFormats)
	return value
}

// checkKeyword reports a word that Concourse does not know for this field
// and silently ignores: CL020. fallback says what
// Concourse does instead.
func (decoder *pipelineDecoder) checkKeyword(node ast.Node, fieldKey string, keywords []string, fallback string) {
	stringNode, isString := decoder.resolve(node).(*ast.StringNode)
	if !isString || isVariable(stringNode.Value) || slices.Contains(keywords, stringNode.Value) {
		return
	}
	decoder.reportIgnoredValue(position(stringNode),
		fmt.Sprintf("unknown %s %q, which Concourse ignores and %s", fieldKey, stringNode.Value, fallback),
		didYouMean(stringNode.Value, keywords))
	decoder.suggest(stringNode.Value, keywords)
}

// checkPutInputs checks inputs of a put step: "all", "detect" or a list of
// artifact names. fly rejects other types (CL001); an unknown word is
// ignored by Concourse (CL020).
func (decoder *pipelineDecoder) checkPutInputs(pair *ast.MappingValueNode) {
	if decoder.isUnknownValue(pair.Value) {
		return
	}
	switch decoder.resolve(pair.Value).(type) {
	case *ast.StringNode:
		decoder.checkKeyword(pair.Value, "inputs", putInputsKeywords, "uses its default")
	case *ast.SequenceNode:
		decoder.stringList(pair)
	default:
		decoder.reportWrongType(pair, `"all", "detect" or a list of artifact names`, "")
	}
}
