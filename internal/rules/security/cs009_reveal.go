package security

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// CS009 reports a load_var step with reveal: true. Cases:
// spec/rules/security/CS009/
var CS009 = analysis.Rule{
	ID:       "CS009",
	Severity: analysis.SeverityInfo,
	Title:    "load_var reveals the value in the log",
	Run:      runCS009,
}

func runCS009(pass *analysis.Pass) {
	for _, job := range pass.Pipeline.Jobs {
		if job == nil {
			continue
		}
		for step := range job.AllSteps() {
			if step.LoadVar == nil || !step.LoadVar.Reveal.Set || !step.LoadVar.Reveal.Value {
				continue
			}
			pass.Report(step.LoadVar.Reveal.Position,
				fmt.Sprintf("the value of %q is shown in the build log", step.LoadVar.Name.Value),
				"remove reveal: if the value can be secret")
		}
	}
}
