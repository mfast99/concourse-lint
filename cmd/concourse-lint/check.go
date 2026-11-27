package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/mfast99/concourse-lint/internal/analysis"
	"github.com/mfast99/concourse-lint/internal/lint"
	"github.com/mfast99/concourse-lint/internal/report"
)

// formats maps the value of --format to its writer.
var formats = map[string]func(io.Writer, report.Result) error{
	"text":  report.Text,
	"json":  report.JSON,
	"sarif": report.SARIF,
	"github": func(writer io.Writer, result report.Result) error {
		return report.GitHub(writer, result, gitHubEnvironment())
	},
}

// severityRank orders the severities for --fail-on: a finding fails the run
// if its rank is at least the rank of the --fail-on value.
var severityRank = map[analysis.Severity]int{
	analysis.SeverityInfo:    1,
	analysis.SeverityWarning: 2,
	analysis.SeverityError:   3,
}

// runCheck is "concourse-lint check". Usage errors stop before any file is
// read. An input that cannot be read is reported, and the other files are
// still checked (spec/README.md, "CLI contract").
func runCheck(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage) }
	format := flags.String("format", "text", "output format: text, json, sarif or github")
	failOn := flags.String("fail-on", "error", "lowest severity that gives exit code 1: error, warning or info")
	configPath := flags.String("config", "", "config file to use instead of searching for .concourse-lint.yml")
	paths, err := parseFlagsAndPaths(flags, args)
	if err != nil {
		// -h and --help are not an error; the flag package has already
		// printed the usage.
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}

	writeReport, knownFormat := formats[*format]
	if !knownFormat {
		return usageError(stderr, "unknown format %q (want text, json, sarif or github)", *format)
	}
	failRank, knownSeverity := severityRank[analysis.Severity(*failOn)]
	if !knownSeverity {
		return usageError(stderr, "unknown --fail-on value %q (want error, warning or info)", *failOn)
	}
	if len(paths) == 0 {
		return usageError(stderr, "no file given (use - to read stdin)")
	}
	stdinCount := 0
	for _, path := range paths {
		if path == "-" {
			stdinCount++
		}
	}
	if stdinCount > 1 {
		return usageError(stderr, "- (stdin) can only be given once")
	}

	checked, err := lint.CheckPaths(paths, lint.PathOptions{ConfigPath: *configPath, Stdin: stdin, StdinName: report.StdinPath})
	if err != nil {
		fmt.Fprintf(stderr, "concourse-lint: %v\n", err)
		return exitError
	}
	result := toReport(checked)

	for _, inputError := range result.Errors {
		fmt.Fprintf(stderr, "concourse-lint: %s\n", inputError.Message)
	}
	if err := writeReport(stdout, result); err != nil {
		fmt.Fprintf(stderr, "concourse-lint: writing the report failed: %v\n", err)
		return exitError
	}
	if *format == "github" {
		if err := appendStepSummary(result); err != nil {
			fmt.Fprintf(stderr, "concourse-lint: writing the step summary failed: %v\n", err)
			return exitError
		}
	}
	return exitCode(result, failRank)
}

// gitHubEnvironment reads the variables of a GitHub Actions run. Outside
// of one they are empty, and the GitHub writers keep paths as given.
func gitHubEnvironment() report.GitHubEnvironment {
	// Without a working directory, relative paths stay as given.
	workingDirectory, _ := os.Getwd()
	return report.GitHubEnvironment{
		Workspace:        os.Getenv("GITHUB_WORKSPACE"),
		WorkingDirectory: workingDirectory,
		ServerURL:        os.Getenv("GITHUB_SERVER_URL"),
		Repository:       os.Getenv("GITHUB_REPOSITORY"),
		Commit:           os.Getenv("GITHUB_SHA"),
	}
}

// appendStepSummary appends the summary of all findings to the file named
// by GITHUB_STEP_SUMMARY, if it is set. GitHub collects the file of every
// step; appending keeps what earlier commands of the same step wrote.
func appendStepSummary(result report.Result) error {
	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		return nil
	}
	summaryFile, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := report.GitHubSummary(summaryFile, result, gitHubEnvironment()); err != nil {
		summaryFile.Close()
		return err
	}
	return summaryFile.Close()
}

// parseFlagsAndPaths reads the flags and returns the file names. Flags may
// come before, between or after the files. The flag package stops at the
// first argument that is not a flag, so this parses again after every file.
// Everything after "--" is a file name, even if it starts with "-".
func parseFlagsAndPaths(flags *flag.FlagSet, args []string) ([]string, error) {
	var afterDoubleDash []string
	if index := slices.Index(args, "--"); index >= 0 {
		args, afterDoubleDash = args[:index], args[index+1:]
	}
	var paths []string
	for {
		if err := flags.Parse(args); err != nil {
			return nil, err
		}
		args = flags.Args()
		if len(args) == 0 {
			return append(paths, afterDoubleDash...), nil
		}
		paths = append(paths, args[0])
		args = args[1:]
	}
}

// toReport turns the result of lint.CheckPaths into the input of the
// report writers.
func toReport(checked lint.PathsResult) report.Result {
	result := report.Result{ToolVersion: currentVersion()}
	for _, file := range checked.Files {
		result.Files = append(result.Files, report.File{Path: file.Path, Findings: file.Findings})
	}
	for _, inputError := range checked.Errors {
		result.Errors = append(result.Errors, report.InputError{Path: inputError.Path, Message: inputError.Message})
	}
	return result
}

// exitCode is exitError if an input could not be read, else exitFindings if a
// finding is at or above failRank, else exitOK.
func exitCode(result report.Result, failRank int) int {
	if len(result.Errors) > 0 {
		return exitError
	}
	for _, file := range result.Files {
		for _, finding := range file.Findings {
			if severityRank[finding.Severity] >= failRank {
				return exitFindings
			}
		}
	}
	return exitOK
}

// usageError prints a message and the usage to stderr and returns exitError.
func usageError(stderr io.Writer, format string, values ...any) int {
	fmt.Fprintf(stderr, "concourse-lint: "+format+"\n\n%s", append(values, usage)...)
	return exitError
}
