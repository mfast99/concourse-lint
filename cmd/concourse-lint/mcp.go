package main

import (
	"fmt"
	"io"

	"github.com/mfast99/concourse-lint/internal/mcp"
)

// runMCP runs the MCP server on stdin/stdout until the client closes stdin.
// stdio is the only transport, so there are no arguments.
func runMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "concourse-lint mcp: unknown argument %q (the server speaks MCP on stdin/stdout and takes no arguments)\n", args[0])
		return exitError
	}
	if err := mcp.Serve(stdin, stdout, mcp.Options{Version: currentVersion(), Log: stderr}); err != nil {
		fmt.Fprintf(stderr, "concourse-lint mcp: %v\n", err)
		return exitFindings
	}
	return exitOK
}
