// Package docs embeds the rule pages of docs/rules/ into the binary, so
// that the editor, concourse-lint explain and the MCP
// tool explain_rule can show a rule without a network connection.
package docs

import (
	"embed"
	"strings"
)

//go:embed rules/*.md
var rulePages embed.FS

// RulePage is the part of a rule page that is shown to users; the section
// "Test cases" is for contributors and left out.
type RulePage struct {
	// From the first line "# CL002: <title>".
	Title string
	// The sections "What is checked?", "Example", "Fix" and "Suppress", as
	// Markdown, with links as written on the page.
	Checked  string
	Example  string
	Fix      string
	Suppress string
}

// Rule returns the page of a rule, e.g. "CL002".
func Rule(id string) (RulePage, bool) {
	content, err := rulePages.ReadFile("rules/" + id + ".md")
	if err != nil {
		return RulePage{}, false
	}
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	firstLine, _, _ := strings.Cut(text, "\n")
	return RulePage{
		Title:    strings.TrimPrefix(firstLine, "# "+id+": "),
		Checked:  section(text, "What is checked?"),
		Example:  section(text, "Example"),
		Fix:      section(text, "Fix"),
		Suppress: section(text, "Suppress"),
	}, true
}

// section returns the text under "## <heading>" up to the next "## ".
func section(text, heading string) string {
	_, rest, found := strings.Cut(text, "\n## "+heading+"\n")
	if !found {
		return ""
	}
	body, _, _ := strings.Cut(rest, "\n## ")
	return strings.TrimSpace(body)
}
