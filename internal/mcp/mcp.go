// Package mcp is the MCP server for AI agents: JSON-RPC on
// stdin/stdout, one message per line, with the tools check_pipeline,
// list_rules and explain_rule. It checks through lint like the CLI, so an
// agent gets the same findings. The contract is in spec/README.md, "MCP
// server contract".
package mcp
