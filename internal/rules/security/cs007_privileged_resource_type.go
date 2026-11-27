package security

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// CS007 reports a resource type with privileged: true. Cases:
// spec/rules/security/CS007/
var CS007 = analysis.Rule{
	ID:       "CS007",
	Severity: analysis.SeverityWarning,
	Title:    "Privileged resource type",
	Run:      runCS007,
}

func runCS007(pass *analysis.Pass) {
	for _, resourceType := range pass.Pipeline.ResourceTypes {
		if resourceType == nil || !resourceType.Privileged.Set || !resourceType.Privileged.Value {
			continue
		}
		pass.Report(resourceType.Privileged.Position,
			fmt.Sprintf("resource type %q runs privileged, with full root access to the worker", resourceType.Name.Value),
			"remove privileged: unless the resource really needs it")
	}
}
