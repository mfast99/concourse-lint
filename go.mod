module github.com/mfast99/concourse-lint

go 1.27.0

require (
	github.com/gobwas/glob v0.2.3
	github.com/mfast99/go-yaml v1.19.3-concourselint.1
)

// Published before the first public release (0.2.0); use 0.2.0 or newer.
retract [v0.1.0, v0.1.1]
