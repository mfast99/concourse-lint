package security

import (
	"slices"
	"strings"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/model"
)

// CS004 reports a value in a source that starts with http:// or git://.
// Loopback hosts and the allowed_hosts of the config are exempt. Cases: spec/rules/security/CS004/
var CS004 = analysis.Rule{
	ID:       "CS004",
	Severity: analysis.SeverityWarning,
	Title:    "Unencrypted URL",
	Run:      runCS004,
}

var loopbackHosts = []string{"localhost", "127.0.0.1", "[::1]"}

func runCS004(pass *analysis.Pass) {
	for _, sourceBlock := range allSources(pass.Pipeline) {
		forEachScalar(sourceBlock.node, func(scalar *model.Node) {
			scheme, rest, found := strings.Cut(scalar.Text, "://")
			if !found {
				return
			}
			scheme = strings.ToLower(scheme)
			if scheme != "http" && scheme != "git" {
				return
			}
			host := strings.ToLower(urlHost(rest))
			if slices.Contains(loopbackHosts, host) || slices.Contains(pass.Settings.AllowedHosts, host) {
				return
			}
			pass.Report(scalar.Position, scheme+":// is unencrypted", "use https:// or ssh")
		})
	}
}

// urlHost returns the host of the part of a URL after "://", without user
// and port; "[::1]" keeps its brackets.
func urlHost(rest string) string {
	authority := rest
	if end := strings.IndexAny(authority, "/?#"); end >= 0 {
		authority = authority[:end]
	}
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		authority = authority[at+1:]
	}
	if strings.HasPrefix(authority, "[") {
		if end := strings.Index(authority, "]"); end >= 0 {
			return authority[:end+1]
		}
		return authority
	}
	host, _, _ := strings.Cut(authority, ":")
	return host
}
