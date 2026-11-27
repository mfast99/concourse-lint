package correctness

import (
	"fmt"
	"reflect"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CL017 reports an unquoted value in a free-form block (source, params,
// vars, across values, ...) that Concourse passes on as a number in a
// different spelling: 1.10 becomes 1.1, 0022 becomes 18. The parser reports
// the same rule for string fields outside steps. Cases:
// spec/rules/correctness/CL017/
var CL017 = analysis.Rule{
	ID:       "CL017",
	Severity: analysis.SeverityWarning,
	Title:    "Unquoted value changes when Concourse reads it",
	Run:      runCL017,
}

// numericNodeFields are free-form fields that are numbers anyway, so their
// spelling does not matter.
var numericNodeFields = map[string]bool{"MaxInFlight": true, "ContainerLimits": true}

func runCL017(pass *analysis.Pass) {
	forEachNode(reflect.ValueOf(pass.Pipeline), func(node *model.Node) {
		checkNumberSpelling(pass, node)
	})
}

func checkNumberSpelling(pass *analysis.Pass, node *model.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case model.MappingNode:
		for _, pair := range node.Pairs {
			checkNumberSpelling(pass, pair.Value)
		}
	case model.ListNode:
		for _, item := range node.Items {
			checkNumberSpelling(pass, item)
		}
	case model.ScalarNode:
		if node.Quoted || node.Text == "" {
			return
		}
		spelling := model.ResolveYAML11(node.Text).AsJSON()
		if spelling == node.Text {
			return
		}
		pass.Report(node.Position,
			fmt.Sprintf("unquoted %s is passed on as %s", node.Text, spelling),
			fmt.Sprintf("quote it: %q", node.Text))
	}
}

var nodeType = reflect.TypeOf((*model.Node)(nil))

// forEachNode calls visit for every top-level free-form block in the model
// (steps and hooks included), except the fields in numericNodeFields.
func forEachNode(value reflect.Value, visit func(node *model.Node)) {
	if value.Type() == nodeType {
		if !value.IsNil() {
			visit(value.Interface().(*model.Node))
		}
		return
	}
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			forEachNode(value.Elem(), visit)
		}
	case reflect.Struct:
		for index := range value.NumField() {
			field := value.Type().Field(index)
			if field.IsExported() && !numericNodeFields[field.Name] {
				forEachNode(value.Field(index), visit)
			}
		}
	case reflect.Slice:
		for index := range value.Len() {
			forEachNode(value.Index(index), visit)
		}
	}
}
