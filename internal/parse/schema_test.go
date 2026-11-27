package parse_test

import (
	"slices"
	"testing"

	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestFields: the keys come from the field tables; a step of a kind has
// its kind key, its fields and the modifiers, a step without a kind the
// step kinds and the modifiers.
func TestFields(t *testing.T) {
	tests := []struct {
		object  string
		has     []string
		hasNot  []string
		isEmpty bool
	}{
		{object: "resource", has: []string{"name", "type", "source", "check_every"}, hasNot: []string{"plan"}},
		{object: "step:get", has: []string{"get", "trigger", "passed", "attempts", "on_failure", "tags"}, hasNot: []string{"file", "put"}},
		{object: "step:load_var", has: []string{"load_var", "file", "format"}, hasNot: []string{"tags"}},
		{object: "step", has: []string{"get", "put", "task", "in_parallel", "attempts"}, hasNot: []string{"trigger"}},
		{object: "task_config", has: []string{"platform", "run", "image_resource"}},
		{object: "source", isEmpty: true},
		{object: "step:nothing", isEmpty: true},
	}
	for _, test := range tests {
		keys := parse.Fields(test.object)
		if test.isEmpty != (len(keys) == 0) {
			t.Errorf("Fields(%s) = %v", test.object, keys)
		}
		if !slices.IsSorted(keys) {
			t.Errorf("Fields(%s) is not sorted: %v", test.object, keys)
		}
		for _, key := range test.has {
			if !slices.Contains(keys, key) {
				t.Errorf("Fields(%s) lacks %q: %v", test.object, key, keys)
			}
		}
		for _, key := range test.hasNot {
			if slices.Contains(keys, key) {
				t.Errorf("Fields(%s) has %q", test.object, key)
			}
		}
	}
	if got := parse.Keywords("get", "version"); !slices.Equal(got, []string{"latest", "every"}) {
		t.Errorf("Keywords(get, version) = %v", got)
	}
}
