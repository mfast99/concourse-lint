package correctness

import (
	"fmt"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CL005 reports a name that is used twice within a category: resources,
// resource types, prototypes, var sources, jobs, groups, and inside a job the
// names of get steps and load_var steps (fly rejects all of them). Every
// further occurrence is reported. Cases: spec/rules/correctness/CL005/
var CL005 = analysis.Rule{
	ID:       "CL005",
	Severity: analysis.SeverityError,
	Title:    "Duplicate name",
	Run:      runCL005,
}

func runCL005(pass *analysis.Pass) {
	pipeline := pass.Pipeline
	var names []model.Value[string]

	names = nil
	for _, resource := range pipeline.Resources {
		if resource != nil {
			names = append(names, resource.Name)
		}
	}
	reportDuplicates(pass, "resource", names)

	names = nil
	for _, resourceType := range pipeline.ResourceTypes {
		if resourceType != nil {
			names = append(names, resourceType.Name)
		}
	}
	reportDuplicates(pass, "resource type", names)

	names = nil
	for _, prototype := range pipeline.Prototypes {
		if prototype != nil {
			names = append(names, prototype.Name)
		}
	}
	reportDuplicates(pass, "prototype", names)

	names = nil
	for _, varSource := range pipeline.VarSources {
		if varSource != nil {
			names = append(names, varSource.Name)
		}
	}
	reportDuplicates(pass, "var source", names)

	names = nil
	for _, group := range pipeline.Groups {
		if group != nil {
			names = append(names, group.Name)
		}
	}
	reportDuplicates(pass, "group", names)

	names = nil
	for _, job := range pipeline.Jobs {
		if job != nil {
			names = append(names, job.Name)
		}
	}
	reportDuplicates(pass, "job", names)

	for _, job := range pipeline.Jobs {
		if job == nil {
			continue
		}
		var getNames, localVarNames []model.Value[string]
		for step := range job.AllSteps() {
			if !step.HasOneKind() {
				continue
			}
			if step.Get != nil {
				getNames = append(getNames, step.Get.Name)
			}
			if step.LoadVar != nil {
				localVarNames = append(localVarNames, step.LoadVar.Name)
			}
		}
		reportDuplicates(pass, "get step", getNames)
		reportDuplicates(pass, "load_var step", localVarNames)
	}
}

// reportDuplicates reports every occurrence of a name after the first one.
func reportDuplicates(pass *analysis.Pass, category string, names []model.Value[string]) {
	firstOccurrence := map[string]model.Position{}
	for _, name := range names {
		if !checkable(name) {
			continue
		}
		first, seen := firstOccurrence[name.Value]
		if !seen {
			firstOccurrence[name.Value] = name.Position
			continue
		}
		pass.Report(name.Position,
			fmt.Sprintf("%s %q is defined more than once", category, name.Value),
			fmt.Sprintf("first defined in line %d", first.Line))
	}
}
