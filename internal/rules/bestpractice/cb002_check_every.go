package bestpractice

import (
	"fmt"
	"strings"
	"time"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CB002 reports a check_every below one minute on a resource or resource
// type. Cases: spec/rules/bestpractice/CB002/
var CB002 = analysis.Rule{
	ID:       "CB002",
	Severity: analysis.SeverityWarning,
	Title:    "check_every below one minute",
	Run:      runCB002,
}

// defaultMinimumCheckInterval is the shortest check_every that CB002
// accepts unless the config sets min_interval.
const defaultMinimumCheckInterval = time.Minute

func runCB002(pass *analysis.Pass) {
	minimum := defaultMinimumCheckInterval
	if pass.Settings.MinCheckInterval > 0 {
		minimum = pass.Settings.MinCheckInterval
	}
	for _, resource := range pass.Pipeline.Resources {
		if resource != nil {
			checkInterval(pass, resource.CheckEvery, minimum)
		}
	}
	for _, resourceType := range pass.Pipeline.ResourceTypes {
		if resourceType != nil {
			checkInterval(pass, resourceType.CheckEvery, minimum)
		}
	}
}

// checkInterval reports the value if it is a valid duration below the
// minimum. Invalid values are CL001 (parser), ((vars)) are unknown.
func checkInterval(pass *analysis.Pass, checkEvery model.Value[string], minimum time.Duration) {
	if !checkEvery.Set || strings.Contains(checkEvery.Value, "((") {
		return
	}
	interval, err := time.ParseDuration(checkEvery.Value)
	if err != nil || interval >= minimum {
		return
	}
	pass.Report(checkEvery.Position,
		fmt.Sprintf("check_every %s is below %s and puts load on the external system", checkEvery.Value, minimum),
		fmt.Sprintf("use %s or more, or a webhook", minimum))
}
