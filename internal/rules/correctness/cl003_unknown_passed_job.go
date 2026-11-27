package correctness

import (
	"fmt"
	"slices"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// CL003 reports a passed: entry naming a job that does not exist. Cases:
// spec/rules/correctness/CL003/
var CL003 = analysis.Rule{
	ID:       "CL003",
	Severity: analysis.SeverityError,
	Title:    "passed: names a missing job",
	Run:      runCL003,
}

func runCL003(pass *analysis.Pass) {
	jobs := navigate.JobNames(pass.Pipeline)
	for _, job := range pass.Pipeline.Jobs {
		if job == nil {
			continue
		}
		for step := range job.AllSteps() {
			if step.Get == nil {
				continue
			}
			for _, passedJob := range step.Get.Passed {
				if !checkable(passedJob) || slices.Contains(jobs, passedJob.Value) {
					continue
				}
				pass.ReportTypo(passedJob.Position,
					fmt.Sprintf("job %q in passed: is not defined", passedJob.Value),
					passedJob.Value, jobs)
			}
		}
	}
}
