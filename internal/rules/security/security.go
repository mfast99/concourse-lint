// Package security contains the rules CS001 to CS009: secrets, TLS,
// supply chain and privileges. One rule = one file, then register it in All here.
package security

import "github.com/mfast99/concourse-lint/internal/analysis"

// All are all rules of this area.
var All = []*analysis.Rule{
	&CS001,
	&CS002,
	&CS003,
	&CS004,
	&CS005,
	&CS006,
	&CS007,
	&CS008,
	&CS009,
}
