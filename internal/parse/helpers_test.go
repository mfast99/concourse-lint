package parse_test

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/parse"
)

// checkModel parses a file from spec/ and compares the result with the
// expected model. The file must not produce any finding.
func checkModel(t *testing.T, specFile string, want *model.Pipeline) {
	t.Helper()
	source, err := readSpec(specFile)
	if err != nil {
		t.Fatal(err)
	}
	got, findings := parse.Parse(source)
	if len(findings) != 0 {
		t.Errorf("findings: %+v", findings)
	}
	// Comments have their own test (TestComments).
	got.Comments = nil
	clearEnds(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("model differs:\n%s", firstDifference(asJSON(got), asJSON(want)))
	}
}

// firstDifference shows the first line where the two JSON dumps differ, with
// the path of JSON keys leading to it.
func firstDifference(got, want string) string {
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for index := 0; index < max(len(gotLines), len(wantLines)); index++ {
		var gotLine, wantLine string
		if index < len(gotLines) {
			gotLine = gotLines[index]
		}
		if index < len(wantLines) {
			wantLine = wantLines[index]
		}
		if gotLine != wantLine {
			context := strings.Join(wantLines[max(0, index-6):min(index, len(wantLines))], "\n")
			return context + "\n got: " + gotLine + "\nwant: " + wantLine
		}
	}
	return "(no line differs; compare the types)"
}

// readSpec reads a file below spec/.
func readSpec(specFile string) ([]byte, error) {
	return os.ReadFile("../../spec/" + specFile)
}

func asJSON(pipeline *model.Pipeline) string {
	encoded, _ := json.MarshalIndent(pipeline, "", "  ")
	return string(encoded)
}

func at(line, column int) model.Position { return model.Position{Line: line, Column: column} }

// stringField is a string value with its key on the same line.
func stringField(text string, line, keyColumn, valueColumn int) model.Value[string] {
	return model.Value[string]{Value: text, Position: at(line, valueColumn), KeyPosition: at(line, keyColumn), Set: true}
}

// boolField is a boolean value with its key on the same line.
func boolField(value bool, line, keyColumn, valueColumn int) model.Value[bool] {
	return model.Value[bool]{Value: value, Position: at(line, valueColumn), KeyPosition: at(line, keyColumn), Set: true}
}

// intField is a number value with its key on the same line.
func intField(value int, line, keyColumn, valueColumn int) model.Value[int] {
	return model.Value[int]{Value: value, Position: at(line, valueColumn), KeyPosition: at(line, keyColumn), Set: true}
}

// listItem is a string in a list; it has no key.
func listItem(text string, line, column int) model.Value[string] {
	return model.Value[string]{Value: text, Position: at(line, column), Set: true}
}

// scalar is an unquoted free-form value.
func scalar(text string, line, column int) *model.Node {
	return &model.Node{Kind: model.ScalarNode, Position: at(line, column), Text: text}
}

// quoted is a quoted free-form value; the position is the opening quote.
func quoted(text string, line, column int) *model.Node {
	return &model.Node{Kind: model.ScalarNode, Position: at(line, column), Text: text, Quoted: true}
}

// mappingNode is a free-form mapping starting at line:column.
func mappingNode(line, column int, pairs ...*model.NodePair) *model.Node {
	return &model.Node{Kind: model.MappingNode, Position: at(line, column), Pairs: pairs}
}

// listNode is a free-form list starting at line:column.
func listNode(line, column int, items ...*model.Node) *model.Node {
	return &model.Node{Kind: model.ListNode, Position: at(line, column), Items: items}
}

// pair is a key at line:column with its value.
func pair(key string, line, column int, value *model.Node) *model.NodePair {
	return &model.NodePair{Key: key, KeyPosition: at(line, column), Value: value}
}

// clearEnds sets the end of every position below value to zero, so that
// model tests compare only where things start (TestEnds checks the ends).
func clearEnds(value any) {
	clearEndsOf(reflect.ValueOf(value))
}

func clearEndsOf(value reflect.Value) {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !value.IsNil() {
			clearEndsOf(value.Elem())
		}
	case reflect.Slice, reflect.Array:
		for index := 0; index < value.Len(); index++ {
			clearEndsOf(value.Index(index))
		}
	case reflect.Map:
		for _, mapKey := range value.MapKeys() {
			entry := reflect.New(value.Type().Elem()).Elem()
			entry.Set(value.MapIndex(mapKey))
			clearEndsOf(entry)
			value.SetMapIndex(mapKey, entry)
		}
	case reflect.Struct:
		if position, isPosition := value.Addr().Interface().(*model.Position); isPosition {
			*position = position.Start()
			return
		}
		for index := 0; index < value.NumField(); index++ {
			if value.Type().Field(index).IsExported() {
				clearEndsOf(value.Field(index))
			}
		}
	}
}
