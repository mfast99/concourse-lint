package lint

import (
	"reflect"
	"testing"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// TestRemoveDuplicates: findings with the same rule and position count once,
// even if the message differs (e.g. it names a different job for each use
// of an anchor). The first one is kept.
func TestRemoveDuplicates(t *testing.T) {
	finding := func(rule string, line, column int, message string) analysis.Finding {
		return analysis.Finding{Rule: rule, Position: model.Position{Line: line, Column: column}, Message: message}
	}
	findings := []analysis.Finding{
		finding("CL003", 16, 13, "used in job staging"),
		finding("CL003", 16, 13, "used in job production"),
		finding("CS001", 16, 13, "other rule, same place"),
		finding("CL003", 16, 14, "same rule, other column"),
		finding("CL003", 17, 13, "same rule, other line"),
	}
	want := []analysis.Finding{findings[0], findings[2], findings[3], findings[4]}
	if got := removeDuplicates(findings); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}
