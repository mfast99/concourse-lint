package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/mfast99/concourse-lint/internal/lint"
)

// runExplain is "concourse-lint explain <ID>": the page of one rule, with
// its severity under the config file (spec/README.md, "Rules and
// explain").
func runExplain(args []string, stdout, stderr io.Writer) int {
	flags, format, configFlag := ruleFlags("explain", stderr)
	arguments, err := parseFlagsAndPaths(flags, args)
	if err != nil {
		return flagErrorCode(err)
	}
	if *format != "text" && *format != "json" {
		return usageError(stderr, "unknown format %q (want text or json)", *format)
	}
	switch {
	case len(arguments) == 0:
		return usageError(stderr, "explain needs a rule ID, e.g. concourse-lint explain CL002")
	case len(arguments) > 1:
		return usageError(stderr, "explain takes one rule ID, got also %q", arguments[1])
	}
	_, ruleConfig, err := lint.LoadOrFindConfig(*configFlag, ".")
	if err != nil {
		fmt.Fprintf(stderr, "concourse-lint: %v\n", err)
		return exitError
	}
	explanation, err := lint.ExplainRule(arguments[0], ruleConfig)
	if err != nil {
		fmt.Fprintf(stderr, "concourse-lint: %v; concourse-lint rules lists all rules\n", err)
		return exitError
	}

	var output bytes.Buffer
	if *format == "json" {
		err = writeJSON(&output, explanation)
	} else {
		writeExplanation(&output, explanation, styleFor(stdout))
	}
	return writeOutput(stdout, stderr, output.Bytes(), err)
}

// writeExplanation writes the title, the severity, the sections of the
// rule page and the link to the full page.
func writeExplanation(output io.Writer, explanation lint.RuleExplanation, style textStyle) {
	fmt.Fprintln(output, style.bold(explanation.ID+": "+explanation.Title))
	fmt.Fprintf(output, "Severity: %s\n", severityLabel(explanation.Severity, explanation.DefaultSeverity, style))
	sections := []struct{ heading, markdown string }{
		{"What is checked?", explanation.Checked},
		{"Example", explanation.Example},
		{"Fix", explanation.Fix},
		{"Suppress", explanation.Suppress},
	}
	for _, section := range sections {
		if strings.TrimSpace(section.markdown) == "" {
			continue
		}
		fmt.Fprintf(output, "\n%s\n%s\n", renderHeading(section.heading, style), renderMarkdown(section.markdown, style))
	}
	fmt.Fprintf(output, "\nFull page: %s\n", style.link(explanation.Docs))
}
