// Package correctness contains the correctness rules (CL): errors for which
// Concourse rejects the pipeline, that fail at runtime, or that are almost
// certainly a mistake.
//
// One rule = one file, e.g. cl002_unknown_resource.go, with exactly one
// variable of type analysis.Rule. Then register it in All here.
// CL000, CL001, CL018 to CL021 and CL017 for string fields come from the
// parser (parse.Rules), CL015 from lint.
package correctness

import "github.com/mfast99/concourse-lint/internal/analysis"

// All are all rules of this area.
var All = []*analysis.Rule{
	&CL002,
	&CL003,
	&CL004,
	&CL005,
	&CL006,
	&CL007,
	&CL008,
	&CL009,
	&CL010,
	&CL011,
	&CL012,
	&CL013,
	&CL014,
	&CL016,
	&CL017,
}
