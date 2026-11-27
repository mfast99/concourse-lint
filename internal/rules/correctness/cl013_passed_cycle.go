package correctness

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// CL013 reports every passed: entry that is part of a cycle in the graph of
// jobs (across all resources), also a job that waits for itself. Cases:
// spec/rules/correctness/CL013/
var CL013 = analysis.Rule{
	ID:       "CL013",
	Severity: analysis.SeverityError,
	Title:    "Cycle via passed:",
	Run:      runCL013,
}

func runCL013(pass *analysis.Pass) {
	// waitsFor[job] are the jobs named in passed: of any get of job.
	waitsFor := map[string][]string{}
	for _, job := range pass.Pipeline.Jobs {
		if job == nil || !checkable(job.Name) {
			continue
		}
		for step := range job.AllSteps() {
			if step.Get == nil {
				continue
			}
			for _, passedJob := range step.Get.Passed {
				if checkable(passedJob) {
					waitsFor[job.Name.Value] = append(waitsFor[job.Name.Value], passedJob.Value)
				}
			}
		}
	}
	for _, job := range pass.Pipeline.Jobs {
		if job == nil || !checkable(job.Name) {
			continue
		}
		for step := range job.AllSteps() {
			if step.Get == nil {
				continue
			}
			for _, passedJob := range step.Get.Passed {
				// The entry job -> passedJob closes a cycle if passedJob waits,
				// directly or indirectly, for job.
				if !checkable(passedJob) || !waitsTransitively(waitsFor, passedJob.Value, job.Name.Value) {
					continue
				}
				pass.Report(passedJob.Position,
					fmt.Sprintf("passed: %q is part of a cycle: job %q waits for itself", passedJob.Value, job.Name.Value),
					"remove one passed: entry of the cycle")
			}
		}
	}
}

// waitsTransitively reports whether start is target or waits for it through
// a chain of passed: entries.
func waitsTransitively(waitsFor map[string][]string, start, target string) bool {
	visited := map[string]bool{}
	pending := []string{start}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if current == target {
			return true
		}
		if visited[current] {
			continue
		}
		visited[current] = true
		pending = append(pending, waitsFor[current]...)
	}
	return false
}
