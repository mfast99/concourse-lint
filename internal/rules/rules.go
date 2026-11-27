// Package rules collects the rules of all areas in one place.
package rules

import (
	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/rules/bestpractice"
	"github.com/mfast99/concourse-lint/internal/rules/correctness"
	"github.com/mfast99/concourse-lint/internal/rules/security"
)

// All returns all rules in a fixed order.
func All() []*analysis.Rule {
	var all []*analysis.Rule
	all = append(all, correctness.All...)
	all = append(all, security.All...)
	all = append(all, bestpractice.All...)
	return all
}
