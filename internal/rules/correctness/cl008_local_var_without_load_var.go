package correctness

import (
	"fmt"
	"regexp"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/flow"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CL008 reports ((.:name)) without a load_var of that name earlier in the
// job, or outside the across step that defines it. ((.:name.field)) refers
// to name. Cases: spec/rules/correctness/CL008/
var CL008 = analysis.Rule{
	ID:       "CL008",
	Severity: analysis.SeverityError,
	Title:    "((.:var)) without load_var",
	Run:      runCL008,
}

// localVarPattern finds ((.:name)) and ((.:name.field)). A quoted name may
// contain dots, ((.:"local.var")) (vars.ParseReference in Concourse). The
// first group is a quoted name without its quotes, the second a plain name.
var localVarPattern = regexp.MustCompile(`\(\(\s*\.:\s*(?:"([^"]*)"|([^.)\s"]+))`)

func runCL008(pass *analysis.Pass) {
	forEachStepInScope(pass.Pipeline, func(step *model.Step, scope flow.Scope) {
		for _, text := range stepStrings(step) {
			for _, match := range localVarPattern.FindAllStringSubmatch(text.Value, -1) {
				name := match[1] + match[2]
				if scope.HasLocalVar(name) {
					continue
				}
				hint := analysis.DidYouMean(name, scope.LocalVarNames())
				if hint == "" {
					hint = "define it with a load_var earlier in the job"
				}
				pass.Report(text.Position, fmt.Sprintf("local variable %q is not defined at this point", name), hint)
			}
		}
	})
}
