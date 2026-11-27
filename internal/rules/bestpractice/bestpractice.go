// Package bestpractice contains the rules CB001 to CB003: hints that make the
// pipeline better but don't break anything. One rule = one file, then
// register it in All here.
package bestpractice

import "github.com/mfast99/concourse-lint/internal/analysis"

// All are all rules of this area.
var All = []*analysis.Rule{
	&CB001,
	&CB002,
	&CB003,
}
