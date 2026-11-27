package parse_test

import (
	"strings"
	"testing"

	"github.com/mfast99/concourse-lint/internal/model"
	"github.com/mfast99/concourse-lint/internal/parse"
)

// TestBooleanSpellings: a boolean field accepts exactly what fly 8.3.4
// accepts (checked with "fly validate-pipeline", see spec/README.md).
// Everything else is CL001, except an empty value and ((var)).
func TestBooleanSpellings(t *testing.T) {
	const line, keyColumn, valueColumn = 7, 3, 11
	unset := model.Value[bool]{KeyPosition: at(line, keyColumn)}
	testCases := []struct {
		spelling string
		want     model.Value[bool]
		// Empty: no finding. Otherwise a CL001 whose hint contains this text.
		wantHint string
		// Where the CL001 is: the value, or the key for a block value.
		wantAtKey bool
	}{
		{spelling: "true", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "True", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "TRUE", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "yes", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "Yes", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "YES", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "on", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "On", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "ON", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "y", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "Y", want: boolField(true, line, keyColumn, valueColumn)},
		{spelling: "false", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "False", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "FALSE", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "no", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "No", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "NO", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "off", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "Off", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "OFF", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "n", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "N", want: boolField(false, line, keyColumn, valueColumn)},
		{spelling: "", want: unset},
		{spelling: "~", want: unset},
		{spelling: "null", want: unset},
		{spelling: "((flag))", want: unset},
		{spelling: `"((flag))"`, want: unset},
		{spelling: `"true"`, want: unset, wantHint: "remove the quotes"},
		{spelling: `'yes'`, want: unset, wantHint: "remove the quotes"},
		{spelling: "tRUE", want: unset, wantHint: "true or false"},
		{spelling: "yEs", want: unset, wantHint: "true or false"},
		{spelling: `"sure"`, want: unset, wantHint: "true or false"},
		{spelling: "1", want: unset, wantHint: "true or false"},
		{spelling: "0", want: unset, wantHint: "true or false"},
		{spelling: "1.5", want: unset, wantHint: "true or false"},
		{spelling: "[true]", want: unset, wantHint: "true or false", wantAtKey: true},
		{spelling: "{value: true}", want: unset, wantHint: "true or false", wantAtKey: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.spelling, func(t *testing.T) {
			source := "resources:\n- name: r\n  type: mock\n  source: {}\n" +
				"jobs:\n- name: j\n  serial: " + testCase.spelling + "\n  plan:\n  - get: r\n"
			pipeline, findings := parse.Parse([]byte(source))
			clearEnds(pipeline)
			if got := pipeline.Jobs[0].Serial; got != testCase.want {
				t.Errorf("serial = %+v, want %+v", got, testCase.want)
			}
			if testCase.wantHint == "" {
				if len(findings) != 0 {
					t.Errorf("findings: %+v, want none", findings)
				}
				return
			}
			if len(findings) != 1 || findings[0].Rule != "CL001" {
				t.Fatalf("findings: %+v, want one CL001", findings)
			}
			finding := findings[0]
			wantPosition := at(line, valueColumn)
			if testCase.wantAtKey {
				wantPosition = at(line, keyColumn)
			}
			if finding.Position.Start() != wantPosition {
				t.Errorf("position %+v, want %+v", finding.Position, wantPosition)
			}
			if !strings.Contains(finding.Message, "serial") {
				t.Errorf("message %q does not name the field", finding.Message)
			}
			if !strings.Contains(finding.Hint, testCase.wantHint) {
				t.Errorf("hint %q does not contain %q", finding.Hint, testCase.wantHint)
			}
		})
	}
}
