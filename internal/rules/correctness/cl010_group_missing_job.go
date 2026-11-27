package correctness

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/navigate"
)

// CL010 reports a group entry that names a missing job, or a glob that
// matches no job. Cases: spec/rules/correctness/CL010/
var CL010 = analysis.Rule{
	ID:       "CL010",
	Severity: analysis.SeverityError,
	Title:    "Group names a missing job",
	Run:      runCL010,
}

func runCL010(pass *analysis.Pass) {
	jobs := navigate.JobNames(pass.Pipeline)
	for _, pattern := range navigate.GroupPatterns(pass.Pipeline) {
		if pattern.Crashes {
			pass.Report(pattern.Entry.Position, fmt.Sprintf("glob %q crashes fly", pattern.Entry.Value),
				"an unclosed or empty {} alternative at the end makes the glob library of Concourse panic; close or remove the braces")
			continue
		}
		if !pattern.Valid {
			pass.Report(pattern.Entry.Position, fmt.Sprintf("%q is no valid glob", pattern.Entry.Value), "")
			continue
		}
		if matchesAny(pattern, jobs) {
			continue
		}
		if navigate.IsGlob(pattern.Entry.Value) {
			pass.Report(pattern.Entry.Position, fmt.Sprintf("glob %q matches no job", pattern.Entry.Value), "")
			continue
		}
		pass.ReportTypo(pattern.Entry.Position,
			fmt.Sprintf("job %q in group is not defined", pattern.Entry.Value),
			pattern.Entry.Value, jobs)
	}
}

// matchesAny reports whether the pattern covers at least one of the jobs.
func matchesAny(pattern navigate.GroupPattern, jobs []string) bool {
	for _, job := range jobs {
		if pattern.Matches(job) {
			return true
		}
	}
	return false
}
