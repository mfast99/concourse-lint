package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mfast99/concourse-lint/internal/lint"
)

// ruleListDocument is the JSON of rules, the same as the MCP tool
// list_rules.
type ruleListDocument struct {
	Rules []lint.RuleInfo `json:"rules"`
}

// runRules is "concourse-lint rules": every rule with its severity under
// the config file (spec/README.md, "Rules and explain").
func runRules(args []string, stdout, stderr io.Writer) int {
	flags, format, configFlag := ruleFlags("rules", stderr)
	arguments, err := parseFlagsAndPaths(flags, args)
	if err != nil {
		return flagErrorCode(err)
	}
	if *format != "text" && *format != "json" {
		return usageError(stderr, "unknown format %q (want text or json)", *format)
	}
	if len(arguments) > 0 {
		return usageError(stderr, "rules takes no arguments, got %q (concourse-lint explain <ID> explains a rule)", arguments[0])
	}
	configPath, ruleConfig, err := lint.LoadOrFindConfig(*configFlag, ".")
	if err != nil {
		fmt.Fprintf(stderr, "concourse-lint: %v\n", err)
		return exitError
	}
	list := lint.RuleList(ruleConfig)

	var output bytes.Buffer
	if *format == "json" {
		err = writeJSON(&output, ruleListDocument{Rules: list})
	} else {
		writeRuleTable(&output, list, displayConfigPath(configPath, *configFlag), styleFor(stdout))
	}
	return writeOutput(stdout, stderr, output.Bytes(), err)
}

// ruleFlags declares the flags of rules and explain.
func ruleFlags(command string, stderr io.Writer) (flags *flag.FlagSet, format, configFlag *string) {
	flags = flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage) }
	format = flags.String("format", "text", "output format: text or json")
	configFlag = flags.String("config", "", "config file to use instead of searching for .concourse-lint.yml")
	return flags, format, configFlag
}

// flagErrorCode is the exit code after a failed flag parse: -h and --help
// are no error, the flag package has already printed the usage.
func flagErrorCode(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}
	return exitError
}

// writeRuleTable writes one line per rule with aligned columns, then where
// the severities come from and how to see more.
func writeRuleTable(output io.Writer, list []lint.RuleInfo, configPath string, style textStyle) {
	labels := make([]string, len(list))
	width := 0
	for index, rule := range list {
		labels[index] = severityLabel(rule.Severity, rule.DefaultSeverity, textStyle{})
		width = max(width, len(labels[index]))
	}
	for index, rule := range list {
		padding := strings.Repeat(" ", width-len(labels[index]))
		fmt.Fprintf(output, "%s  %s%s  %s\n", style.bold(rule.ID), severityLabel(rule.Severity, rule.DefaultSeverity, style), padding, rule.Title)
	}
	fmt.Fprintln(output)
	if configPath != "" {
		fmt.Fprintf(output, "Severities from %s.\n", configPath)
	}
	fmt.Fprintln(output, "Details: concourse-lint explain <ID>")
}

// severityLabel is the severity, with the default when the config changed
// it: "error (default: warning)".
func severityLabel(severity, defaultSeverity string, style textStyle) string {
	label := style.severity(severity)
	if severity != defaultSeverity {
		label += " (default: " + defaultSeverity + ")"
	}
	return label
}

// displayConfigPath is the config file as given with --config, or the one
// the search found relative to the current folder.
func displayConfigPath(configPath, configFlag string) string {
	if configPath == "" || configFlag != "" {
		return configPath
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return configPath
	}
	relative, err := filepath.Rel(workingDirectory, configPath)
	if err != nil {
		return configPath
	}
	return relative
}

// writeJSON writes the value as the MCP server does: indented by two
// spaces, HTML characters unescaped, with a final newline.
func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

// writeOutput writes the finished output, so that nothing reaches stdout
// when building it failed.
func writeOutput(stdout, stderr io.Writer, output []byte, buildError error) int {
	if buildError == nil {
		_, buildError = stdout.Write(output)
	}
	if buildError != nil {
		fmt.Fprintf(stderr, "concourse-lint: writing the output failed: %v\n", buildError)
		return exitError
	}
	return exitOK
}
