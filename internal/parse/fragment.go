package parse

import (
	"bytes"
	"regexp"
	"slices"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// FragmentRule is CL023: the file is a fragment that a tool (spruce,
// aviator) merges into a pipeline: jobs with get or put steps, but no
// resources. lint reports it instead of all other findings, which would be
// a CL002 for every get and put.
const FragmentRule = "CL023"

var (
	// topLevelJobsKey matches the top-level jobs: key.
	topLevelJobsKey = regexp.MustCompile(`(?m)^jobs[ \t]*:`)
	// topLevelResourcesOrMerge matches a top-level resources: key or a
	// top-level merge key, which may bring resources along.
	topLevelResourcesOrMerge = regexp.MustCompile(`(?m)^(?:resources|<<)[ \t]*:`)
)

// Fragment reports whether the first document of the source is a pipeline
// fragment and returns the CL023 finding at its jobs: key. Broken YAML is
// no fragment: its CL000 stays.
func Fragment(source []byte) (analysis.Finding, bool) {
	first, _ := firstDocument(source)
	jobsKey := topLevelJobsKey.FindIndex(first)
	if jobsKey == nil || topLevelResourcesOrMerge.Match(first) {
		return analysis.Finding{}, false
	}
	pipeline, findings := Parse(source)
	if slices.ContainsFunc(findings, func(finding analysis.Finding) bool { return finding.Rule == "CL000" }) || !hasResourceStep(pipeline) {
		return analysis.Finding{}, false
	}
	line := bytes.Count(first[:jobsKey[0]], []byte("\n")) + 1
	return analysis.Finding{
		Rule:     FragmentRule,
		Severity: analysis.SeverityInfo,
		Position: model.Position{Line: line, Column: 1, EndLine: line, EndColumn: 5},
		Message:  "file has get or put steps but no resources:, so it looks like a fragment of a pipeline; it is not checked",
		Hint:     "check the merged pipeline, e.g. spruce merge <files> | concourse-lint check -",
	}, true
}

// hasResourceStep reports whether a job of the pipeline has a get or put
// step, hooks and nested steps included.
func hasResourceStep(pipeline *model.Pipeline) bool {
	for _, job := range pipeline.Jobs {
		if job == nil {
			continue
		}
		for step := range job.AllSteps() {
			if step.Get != nil || step.Put != nil {
				return true
			}
		}
	}
	return false
}
