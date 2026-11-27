package analysis

// DocsBaseURL is where the rule pages docs/rules/<ID>.md are published. It
// is the one place that knows the address.
const DocsBaseURL = "https://github.com/mfast99/concourse-lint/blob/main/docs/rules/"

// DocsURL returns the link to the page of a rule, e.g. for CL002.
func DocsURL(ruleID string) string {
	return DocsBaseURL + ruleID + ".md"
}
