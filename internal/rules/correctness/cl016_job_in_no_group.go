package correctness

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// CL016 reports a job that no group covers, when groups exist. Concourse
// rejects this. Cases: spec/rules/correctness/CL016/
var CL016 = analysis.Rule{
	ID:       "CL016",
	Severity: analysis.SeverityError,
	Title:    "Job in no group",
	Run:      runCL016,
}

func runCL016(pass *analysis.Pass) {
	if len(pass.Pipeline.Groups) == 0 {
		return
	}
	patterns := navigate.GroupPatterns(pass.Pipeline)
	for _, job := range pass.Pipeline.Jobs {
		if job == nil || !checkable(job.Name) || inAnyGroup(job.Name.Value, patterns) {
			continue
		}
		pass.Report(job.Name.Position,
			fmt.Sprintf("job %q belongs to no group", job.Name.Value),
			"add it to a group")
	}
}

// inAnyGroup reports whether one of the group entries covers the job.
func inAnyGroup(jobName string, patterns []navigate.GroupPattern) bool {
	for _, pattern := range patterns {
		if pattern.Matches(jobName) {
			return true
		}
	}
	return false
}
