package spectest

import (
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// findingAt builds a finding at the given line with the catalog severity.
func findingAt(line int, rule string) analysis.Finding {
	return analysis.Finding{Rule: rule, Severity: testCatalog[rule], Position: model.Position{Line: line, Column: 1}}
}

var testCatalog = Catalog{
	"CL000": analysis.SeverityError,
	"CL001": analysis.SeverityError,
	"CL002": analysis.SeverityError,
}

// TestCompare checks that findings are counted, not just ticked off: every
// "# want" stands for exactly one finding.
func TestCompare(t *testing.T) {
	testCases := []struct {
		name      string
		source    string
		got       []analysis.Finding
		wantMatch bool
	}{
		{
			name:      "one want, one finding",
			source:    "a: 1  # want CL002\n",
			got:       []analysis.Finding{findingAt(1, "CL002")},
			wantMatch: true,
		},
		{
			name:   "one want, the same finding twice (e.g. once per anchor use)",
			source: "a: 1  # want CL002\n",
			got:    []analysis.Finding{findingAt(1, "CL002"), findingAt(1, "CL002")},
		},
		{
			name:      "two wants on one line, two findings",
			source:    "a: 1  # want CL002 CL002\n",
			got:       []analysis.Finding{findingAt(1, "CL002"), findingAt(1, "CL002")},
			wantMatch: true,
		},
		{
			name:   "two wants on one line, one finding",
			source: "a: 1  # want CL002 CL002\n",
			got:    []analysis.Finding{findingAt(1, "CL002")},
		},
		{
			name:   "one finding cannot satisfy a line want and want-anywhere",
			source: "# want-anywhere CL001\njob: 1  # want CL001\n",
			got:    []analysis.Finding{findingAt(2, "CL001")},
		},
		{
			name:      "line want and want-anywhere, two findings",
			source:    "# want-anywhere CL001\njob: 1  # want CL001\n",
			got:       []analysis.Finding{findingAt(2, "CL001"), findingAt(1, "CL001")},
			wantMatch: true,
		},
		{
			name:   "want-anywhere stands for exactly one finding",
			source: "# want-anywhere CL000\n",
			got:    []analysis.Finding{findingAt(3, "CL000"), findingAt(5, "CL000")},
		},
		{
			name:   "finding on the wrong line",
			source: "a: 1  # want CL002\nb: 2\n",
			got:    []analysis.Finding{findingAt(2, "CL002")},
		},
		{
			name:   "wrong severity",
			source: "a: 1  # want CL002\n",
			got:    []analysis.Finding{{Rule: "CL002", Severity: analysis.SeverityWarning, Position: model.Position{Line: 1}}},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			problems := Compare(ParseExpectations([]byte(testCase.source)), testCatalog, testCase.got)
			if matches := len(problems) == 0; matches != testCase.wantMatch {
				t.Errorf("match = %v, want %v; problems: %v", matches, testCase.wantMatch, problems)
			}
		})
	}
}
