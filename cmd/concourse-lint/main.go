// Command concourse-lint checks Concourse pipelines.
//
//	concourse-lint check [--format text|json|sarif|github] [--fail-on error|warning|info] [--config <file>] <file|folder|->...
//	concourse-lint rules [--format text|json] [--config <file>]
//	concourse-lint explain [--format text|json] [--config <file>] <ID>
//	concourse-lint lsp       language server on stdin/stdout
//	concourse-lint mcp       MCP server for AI agents on stdin/stdout
//	concourse-lint licenses  license texts of the third-party software in the binary
//	concourse-lint version   (also -v, --version)
//	concourse-lint help      (also -h, --help)
//
// Exit codes: 0 = nothing at or above --fail-on, 1 = findings at or above
// --fail-on, 2 = usage error, a broken config file, or an input could not be
// read.
package main

import (
	"fmt"
	"io"
	"os"
)

const (
	exitOK       = 0
	exitFindings = 1
	// Usage error, or an input could not be read.
	exitError = 2
)

const usage = `Usage:
  concourse-lint check [flags] <file|folder|->...
                                             check pipelines ("-" reads stdin; a
                                             folder: all pipelines in it)
  concourse-lint rules [flags]               list every rule with its severity in
                                             this project
  concourse-lint explain [flags] <ID>        explain a rule: what it checks, an
                                             example, how to fix and suppress it
  concourse-lint lsp                         language server for editors (LSP on
                                             stdin/stdout; exit 0 after shutdown)
  concourse-lint mcp                         MCP server for AI agents (stdin/stdout;
                                             tools check_pipeline, list_rules,
                                             explain_rule; reads only inside the
                                             working directory)
  concourse-lint licenses                    print the licenses of the third-party
                                             software in this binary
  concourse-lint version                     print the version (also -v, --version)
  concourse-lint help                        print this help (also -h, --help)

Flags of check (before, between or after the files; after -- only files):
  --format text|json|sarif|github   output format (default text; github:
                                    annotations and step summary in
                                    GitHub Actions)
  --fail-on error|warning|info      lowest severity that gives exit code 1 (default error)
  --config <file>                   config file to use (default: the nearest
                                    .concourse-lint.yml up to the Git root)

Flags of rules and explain:
  --format text|json                output format (default text)
  --config <file>                   config file to use (default: the nearest
                                    .concourse-lint.yml from the current folder)

Exit codes: 0 = nothing at or above --fail-on, 1 = findings at or above
--fail-on, 2 = usage error, a broken config file, or an input could not be read.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes one command and returns the exit code. It gets stdin, stdout
// and stderr as parameters instead of using os.Stdin etc., so it never ends
// the process itself and could also be called from a test.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitError
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdin, stdout, stderr)
	case "rules":
		return runRules(args[1:], stdout, stderr)
	case "explain":
		return runExplain(args[1:], stdout, stderr)
	case "lsp":
		return runLSP(args[1:], stdin, stdout, stderr)
	case "mcp":
		return runMCP(args[1:], stdin, stdout, stderr)
	case "licenses":
		return runLicenses(args[1:], stdout, stderr)
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "concourse-lint %s\n", currentVersion())
		return exitOK
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "concourse-lint: unknown command %q\n\n%s", args[0], usage)
		return exitError
	}
}
