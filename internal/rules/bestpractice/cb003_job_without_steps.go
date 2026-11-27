package bestpractice

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// CB003 reports a job whose plan is missing or empty. Concourse accepts it,
// but the job does nothing. Cases: spec/rules/bestpractice/CB003/
var CB003 = analysis.Rule{
	ID:       "CB003",
	Severity: analysis.SeverityWarning,
	Title:    "Job without steps",
	Run:      runCB003,
}

func runCB003(pass *analysis.Pass) {
	for _, job := range pass.Pipeline.Jobs {
		if job == nil || !job.Name.Set || job.PlanUnknown || len(job.Plan) > 0 {
			continue
		}
		pass.Report(job.Name.Position, fmt.Sprintf("job %q has no steps", job.Name.Value), "add steps under plan:, or remove the job")
	}
}
