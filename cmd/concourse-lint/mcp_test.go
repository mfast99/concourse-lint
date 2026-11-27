package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// runIn runs the binary in a folder (the MCP server only reads inside its
// working directory).
func runIn(t *testing.T, folder, stdin string, args ...string) cliResult {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Dir = folder
	command.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	exitCode := 0
	if exitError, exited := err.(*exec.ExitError); exited {
		exitCode = exitError.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return cliResult{exitCode, stdout.String(), stderr.String()}
}

// mcpLine builds one JSON-RPC message for the server's stdin.
func mcpLine(t *testing.T, message map[string]any) string {
	t.Helper()
	message["jsonrpc"] = "2.0"
	line, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	return string(line) + "\n"
}

// mcpSession runs `concourse-lint mcp` in the repository root with an
// initialize, the given tool calls (ids 1, 2, …), and the end of the input.
// It returns the answers to the calls by id.
func mcpSession(t *testing.T, calls ...map[string]any) map[int]map[string]any {
	t.Helper()
	return mcpSessionIn(t, repositoryRoot(t), calls...)
}

// mcpSessionIn is mcpSession in another folder.
func mcpSessionIn(t *testing.T, folder string, calls ...map[string]any) map[int]map[string]any {
	t.Helper()
	input := mcpLine(t, map[string]any{"id": 0, "method": "initialize", "params": map[string]any{
		"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test", "version": "1"},
	}})
	input += mcpLine(t, map[string]any{"method": "notifications/initialized"})
	for index, call := range calls {
		input += mcpLine(t, map[string]any{"id": index + 1, "method": "tools/call", "params": call})
	}
	result := runIn(t, folder, input, "mcp")
	if result.exitCode != 0 {
		t.Fatalf("exit code %d, want 0 when stdin ends; stderr: %s", result.exitCode, result.stderr)
	}
	answers := map[int]map[string]any{}
	scanner := bufio.NewScanner(strings.NewReader(result.stdout))
	scanner.Buffer(nil, 64<<20)
	for scanner.Scan() {
		var message map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			t.Fatalf("a line that is no JSON: %q", scanner.Text())
		}
		id, _ := message["id"].(float64)
		answers[int(id)] = message
	}
	if len(answers) != len(calls)+1 {
		t.Fatalf("%d answers, want %d: %s", len(answers), len(calls)+1, result.stdout)
	}
	return answers
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// textAndStructured returns the text block and the structuredContent of a
// tool result.
func textAndStructured(t *testing.T, answer map[string]any) (string, any) {
	t.Helper()
	result, _ := answer["result"].(map[string]any)
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("tool error: %v", result["content"])
	}
	content, _ := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content = %v, want one block", result["content"])
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	return text, result["structuredContent"]
}

// TestMCPSameAsCLI: check_pipeline gives byte for byte the JSON of
// `check --format json` for the same paths, and structuredContent is the
// same document.
func TestMCPSameAsCLI(t *testing.T) {
	root := repositoryRoot(t)
	paths := []string{"spec/rules/correctness/CL002", "spec/valid", "spec/config/severity/pipeline.yml", "spec/config/exclude", "missing.yml"}
	single := "spec/config/severity/pipeline.yml"
	pipelineText := runIn(t, root, "", "check", "--format", "json", single)
	if !strings.Contains(pipelineText.stdout, `"rule"`) {
		t.Fatalf("the single file has no findings, the comparison would prove little: %s", pipelineText.stdout)
	}
	answers := mcpSession(t,
		map[string]any{"name": "check_pipeline", "arguments": map[string]any{"paths": paths}},
		map[string]any{"name": "check_pipeline", "arguments": map[string]any{"text": readFile(t, filepath.Join(root, single)), "filename": single}},
	)

	cli := runIn(t, root, "", append([]string{"check", "--format", "json"}, paths...)...)
	if cli.exitCode != 2 {
		t.Fatalf("CLI exit code %d, want 2 (missing.yml)", cli.exitCode)
	}
	for id, want := range map[int]string{1: cli.stdout, 2: pipelineText.stdout} {
		text, structured := textAndStructured(t, answers[id])
		if text != want {
			t.Errorf("call %d: text differs from the CLI\ngot:\n%s\nwant:\n%s", id, text, want)
		}
		var fromCLI any
		if err := json.Unmarshal([]byte(want), &fromCLI); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(structured, fromCLI) {
			t.Errorf("call %d: structuredContent differs from the CLI's JSON", id)
		}
	}
}

// TestMCPRuleTools: list_rules and explain_rule through the binary.
func TestMCPRuleTools(t *testing.T) {
	answers := mcpSession(t,
		map[string]any{"name": "list_rules"},
		map[string]any{"name": "explain_rule", "arguments": map[string]any{"id": "CS001"}},
	)
	rulesText, _ := textAndStructured(t, answers[1])
	var list struct{ Rules []struct{ ID string } }
	if err := json.Unmarshal([]byte(rulesText), &list); err != nil || len(list.Rules) != 37 {
		t.Errorf("list_rules: %d rules (%v), want 37", len(list.Rules), err)
	}
	explanation, _ := textAndStructured(t, answers[2])
	if !strings.Contains(explanation, `"id": "CS001"`) || !strings.Contains(explanation, `"fix"`) {
		t.Errorf("explain_rule CS001 = %s", explanation)
	}
}

func TestMCPUnknownArgument(t *testing.T) {
	result := runCLI(t, "", "mcp", "--stdio")
	if result.exitCode != 2 || !strings.Contains(result.stderr, `unknown argument "--stdio"`) {
		t.Errorf("got %+v, want exit 2 with a message", result)
	}
}

func TestMCPInHelp(t *testing.T) {
	result := runCLI(t, "", "help")
	if !strings.Contains(result.stdout, "concourse-lint mcp") {
		t.Errorf("help does not mention mcp:\n%s", result.stdout)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
