package correctness

import (
	"reflect"

	"github.com/mfast99/concourse-lint/internal/model"
)

// stepStrings returns every string in the step's own fields, including the
// scalars of free-form blocks (params, vars, source, ...), but not the
// strings of nested steps and hooks: those are visited as steps of their
// own, with their own scope. Reflection keeps this complete when the model
// gets new fields.
func stepStrings(step *model.Step) []model.Value[string] {
	var collected []model.Value[string]
	collectStrings(reflect.ValueOf(step).Elem(), &collected)
	return collected
}

var (
	stepPointerType = reflect.TypeOf((*model.Step)(nil))
	stepSliceType   = reflect.TypeOf([]*model.Step(nil))
	hooksType       = reflect.TypeOf(model.Hooks{})
	stringValueType = reflect.TypeOf(model.Value[string]{})
	nodePointerType = reflect.TypeOf((*model.Node)(nil))
)

func collectStrings(value reflect.Value, collected *[]model.Value[string]) {
	switch value.Type() {
	case stepPointerType, stepSliceType, hooksType:
		return
	case stringValueType:
		if text := value.Interface().(model.Value[string]); text.Set {
			*collected = append(*collected, text)
		}
		return
	case nodePointerType:
		collectNodeStrings(value.Interface().(*model.Node), collected)
		return
	}
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			collectStrings(value.Elem(), collected)
		}
	case reflect.Struct:
		for index := range value.NumField() {
			if value.Type().Field(index).IsExported() {
				collectStrings(value.Field(index), collected)
			}
		}
	case reflect.Slice:
		for index := range value.Len() {
			collectStrings(value.Index(index), collected)
		}
	}
}

func collectNodeStrings(node *model.Node, collected *[]model.Value[string]) {
	if node == nil {
		return
	}
	switch node.Kind {
	case model.ScalarNode:
		*collected = append(*collected, model.Value[string]{Value: node.Text, Position: node.Position, Set: true})
	case model.MappingNode:
		for _, pair := range node.Pairs {
			collectNodeStrings(pair.Value, collected)
		}
	case model.ListNode:
		for _, item := range node.Items {
			collectNodeStrings(item, collected)
		}
	}
}
