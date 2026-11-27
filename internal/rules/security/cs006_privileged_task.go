package security

import (
	"github.com/mfast99/concourse-lint/internal/analysis"
)

// CS006 reports a task step with privileged: true. Cases:
// spec/rules/security/CS006/
var CS006 = analysis.Rule{
	ID:       "CS006",
	Severity: analysis.SeverityWarning,
	Title:    "Privileged task",
	Run:      runCS006,
}

func runCS006(pass *analysis.Pass) {
	for _, job := range pass.Pipeline.Jobs {
		if job == nil {
			continue
		}
		for step := range job.AllSteps() {
			if step.Task == nil || !step.Task.Privileged.Set || !step.Task.Privileged.Value {
				continue
			}
			pass.Report(step.Task.Privileged.Position,
				"task runs privileged, with full root access to the worker",
				"remove privileged: unless the task really needs it (e.g. building images)")
		}
	}
}
