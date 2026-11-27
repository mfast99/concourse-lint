package main

import (
	"fmt"
	"io"
	"time"

	"github.com/mfast99/concourse-lint/internal/lsp"
)

// checkDelay is the time between the last key press and the check.
const checkDelay = 150 * time.Millisecond

// runLSP runs the language server on stdin/stdout until the editor ends the
// session. --stdio is accepted because editors pass it; it is the only
// transport.
func runLSP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	for _, arg := range args {
		if arg != "--stdio" {
			fmt.Fprintf(stderr, "concourse-lint lsp: unknown argument %q (the server speaks LSP on stdin/stdout)\n", arg)
			return exitError
		}
	}
	err := lsp.Serve(stdin, stdout, lsp.Options{Delay: checkDelay, Version: currentVersion(), Log: stderr})
	if err != nil {
		fmt.Fprintf(stderr, "concourse-lint lsp: %v\n", err)
		return exitFindings
	}
	return exitOK
}
