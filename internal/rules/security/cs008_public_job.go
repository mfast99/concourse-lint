package security

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// CS008 reports a job with public: true. Cases: spec/rules/security/CS008/
var CS008 = analysis.Rule{
	ID:       "CS008",
	Severity: analysis.SeverityWarning,
	Title:    "Public build logs",
	Run:      runCS008,
}

func runCS008(pass *analysis.Pass) {
	for _, job := range pass.Pipeline.Jobs {
		if job == nil || !job.Public.Set || !job.Public.Value {
			continue
		}
		pass.Report(job.Public.Position,
			fmt.Sprintf("job %q is public: anyone can read its build logs", job.Name.Value),
			"make sure the logs contain no secrets, or remove public:")
	}
}
