package parse_test

import (
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/parse"
)

// These tests pin down how aliases and merge keys (<<) resolve. The expected
// values are what fly 8.3.4 shows with "fly validate-pipeline --output", not
// what the YAML specification says.

// parseSource parses a pipeline whose only resource has the given source:
// block (indented by four spaces) and returns that source.
func parseSource(t *testing.T, sourceLines ...string) *model.Node {
	t.Helper()
	source := "resources:\n- name: r\n  type: mock\n  source:\n    " +
		strings.Join(sourceLines, "\n    ") +
		"\njobs:\n- name: j\n  plan:\n  - get: r\n"
	pipeline, findings := parse.Parse([]byte(source))
	if len(findings) != 0 {
		t.Fatalf("findings: %+v", findings)
	}
	return pipeline.Resources[0].Source
}

// textAt follows the keys through nested mappings and returns the text of
// the scalar at the end, or "<missing>".
func textAt(node *model.Node, keys ...string) string {
	for _, wanted := range keys {
		var found *model.Node
		if node != nil {
			for _, entry := range node.Pairs {
				if entry.Key == wanted {
					found = entry.Value
				}
			}
		}
		node = found
	}
	if node == nil {
		return "<missing>"
	}
	return node.Text
}

func TestAnchorRedefinedUsesLatestDefinition(t *testing.T) {
	source := parseSource(t,
		"a: &x first",
		"b: *x",
		"c: &x second",
		"d: *x",
	)
	if got := textAt(source, "b"); got != "first" {
		t.Errorf("b = %q, want %q", got, "first")
	}
	if got := textAt(source, "d"); got != "second" {
		t.Errorf("d = %q, want %q", got, "second")
	}
}

func TestMergeOrder(t *testing.T) {
	testCases := []struct {
		name        string
		sourceLines []string
		wantK       string
		wantOnly    string
	}{
		{
			name: "own key after the merge wins",
			sourceLines: []string{
				"base: &b {k: from-anchor, only: anchor}",
				"other:",
				"  <<: *b",
				"  k: own",
			},
			wantK:    "own",
			wantOnly: "anchor",
		},
		{
			name: "merge after the own key wins",
			sourceLines: []string{
				"base: &b {k: from-anchor, only: anchor}",
				"other:",
				"  k: own",
				"  <<: *b",
			},
			wantK:    "from-anchor",
			wantOnly: "anchor",
		},
		{
			name: "in a list of merges the first one wins",
			sourceLines: []string{
				"a: &a {k: from-a}",
				"b: &b {k: from-b, only: b}",
				"other:",
				"  <<: [*a, *b]",
			},
			wantK:    "from-a",
			wantOnly: "b",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			source := parseSource(t, testCase.sourceLines...)
			if got := textAt(source, "other", "k"); got != testCase.wantK {
				t.Errorf("k = %q, want %q", got, testCase.wantK)
			}
			if got := textAt(source, "other", "only"); got != testCase.wantOnly {
				t.Errorf("only = %q, want %q", got, testCase.wantOnly)
			}
		})
	}
}

// TestMergeIntoJob: a merge key also works in typed objects, and merged
// values keep the position of their definition (spec/rules/security/CS008/
// merge-key.yml).
func TestMergeIntoJob(t *testing.T) {
	source, err := readSpec("rules/security/CS008/merge-key.yml")
	if err != nil {
		t.Fatal(err)
	}
	pipeline, _ := parse.Parse(source)
	clearEnds(pipeline)
	if len(pipeline.Jobs) != 2 {
		t.Fatalf("%d jobs, want 2", len(pipeline.Jobs))
	}
	merged := pipeline.Jobs[1]
	if want := stringField("build-arm", 14, 3, 9); merged.Name != want {
		t.Errorf("name = %+v, want %+v", merged.Name, want)
	}
	if want := boolField(true, 11, 3, 11); merged.Public != want {
		t.Errorf("public = %+v, want %+v (position of the anchor)", merged.Public, want)
	}
	if want := at(13, 3); merged.Position != want {
		t.Errorf("position = %+v, want %+v (the << line)", merged.Position, want)
	}
	if len(merged.Plan) != 1 {
		t.Errorf("%d plan steps, want 1 (from the anchor)", len(merged.Plan))
	}
}
