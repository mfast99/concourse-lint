package model_test

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
)

// TestNodeBoolean: rules read booleans in free-form blocks with the same
// YAML 1.1 table as the decoder, e.g. "skip_ssl_verification: yes".
func TestNodeBoolean(t *testing.T) {
	testCases := []struct {
		name        string
		node        *model.Node
		wantValue   bool
		wantBoolean bool
	}{
		{"yes", &model.Node{Kind: model.ScalarNode, Text: "yes"}, true, true},
		{"Off", &model.Node{Kind: model.ScalarNode, Text: "Off"}, false, true},
		{"true", &model.Node{Kind: model.ScalarNode, Text: "true"}, true, true},
		{"quoted yes", &model.Node{Kind: model.ScalarNode, Text: "yes", Quoted: true}, false, false},
		{"mixed case", &model.Node{Kind: model.ScalarNode, Text: "yEs"}, false, false},
		{"number", &model.Node{Kind: model.ScalarNode, Text: "1"}, false, false},
		{"mapping", &model.Node{Kind: model.MappingNode}, false, false},
		{"missing", nil, false, false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			value, isBoolean := testCase.node.Boolean()
			if value != testCase.wantValue || isBoolean != testCase.wantBoolean {
				t.Errorf("Boolean() = %v, %v; want %v, %v", value, isBoolean, testCase.wantValue, testCase.wantBoolean)
			}
		})
	}
}
