package correctness

import (
	"fmt"
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/flow"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CL007 reports a file: path (task, load_var, set_pipeline) that does not
// start with an artifact available at this point. Cases:
// spec/rules/correctness/CL007/
var CL007 = analysis.Rule{
	ID:       "CL007",
	Severity: analysis.SeverityError,
	Title:    "file: without an available artifact",
	Run:      runCL007,
}

func runCL007(pass *analysis.Pass) {
	forEachStepInScope(pass.Pipeline, func(step *model.Step, scope flow.Scope) {
		switch {
		case step.Task != nil:
			checkFilePath(pass, step.Task.File, scope)
		case step.LoadVar != nil:
			checkFilePath(pass, step.LoadVar.File, scope)
		case step.SetPipeline != nil:
			checkFilePath(pass, step.SetPipeline.File, scope)
		}
	})
}

func checkFilePath(pass *analysis.Pass, file model.Value[string], scope flow.Scope) {
	if !file.Set || file.Value == "" {
		return
	}
	if strings.HasPrefix(file.Value, "/") {
		pass.Report(file.Position,
			fmt.Sprintf("file: %q is absolute, but it must start with an artifact name", file.Value),
			"remove the leading /")
		return
	}
	artifact := firstPathSegment(file.Value)
	if isVariable(artifact) || scope.HasArtifact(artifact) {
		return
	}
	pass.Report(file.Position,
		fmt.Sprintf("file: starts with %q, but no artifact with that name is available at this point", artifact),
		missingArtifactHint(pass, artifact, scope))
}
