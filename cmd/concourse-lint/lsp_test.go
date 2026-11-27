package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// lspSession runs the real binary as a language server.
type lspSession struct {
	t       *testing.T
	command *exec.Cmd
	input   io.WriteCloser
	output  *bufio.Reader
}

func startLSP(t *testing.T, args ...string) *lspSession {
	t.Helper()
	command := exec.Command(binary, append([]string{"lsp"}, args...)...)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { command.Process.Kill() })
	return &lspSession{t: t, command: command, input: input, output: bufio.NewReader(output)}
}

func (session *lspSession) send(message string) {
	session.t.Helper()
	if _, err := fmt.Fprintf(session.input, "Content-Length: %d\r\n\r\n%s", len(message), message); err != nil {
		session.t.Fatal(err)
	}
}

func (session *lspSession) receive() map[string]any {
	session.t.Helper()
	length := 0
	for {
		line, err := session.output.ReadString('\n')
		if err != nil {
			session.t.Fatalf("reading from the server: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if value, found := strings.CutPrefix(line, "Content-Length: "); found {
			length, _ = strconv.Atoi(value)
		}
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(session.output, body); err != nil {
		session.t.Fatal(err)
	}
	var message map[string]any
	if err := json.Unmarshal(body, &message); err != nil {
		session.t.Fatal(err)
	}
	return message
}

func (session *lspSession) wait() int {
	session.t.Helper()
	done := make(chan error, 1)
	go func() { done <- session.command.Wait() }()
	select {
	case err := <-done:
		if exitError, exited := err.(*exec.ExitError); exited {
			return exitError.ExitCode()
		}
		if err != nil {
			session.t.Fatal(err)
		}
		return 0
	case <-time.After(5 * time.Second):
		session.t.Fatal("the server did not exit")
		return -1
	}
}

// TestLSP: a whole session with the binary: initialize, open a pipeline,
// get its findings (after the delay), shutdown, exit with 0.
func TestLSP(t *testing.T) {
	session := startLSP(t, "--stdio")
	session.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}`)
	if response := session.receive(); response["result"] == nil {
		t.Fatalf("initialize: %v", response)
	}
	session.send(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)
	pipeline, _ := json.Marshal("jobs:\n- name: build\n  plan:\n  - get: repo\nresources: []\n")
	session.send(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"untitled:p","languageId":"yaml","version":1,"text":` + string(pipeline) + `}}}`)
	published := session.receive()
	if published["method"] != "textDocument/publishDiagnostics" {
		t.Fatalf("got %v, want diagnostics", published)
	}
	diagnostics := published["params"].(map[string]any)["diagnostics"].([]any)
	if len(diagnostics) != 1 || diagnostics[0].(map[string]any)["code"] != "CL002" {
		t.Errorf("diagnostics = %v, want CL002", diagnostics)
	}
	session.send(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)
	if response := session.receive(); response["id"] != float64(2) {
		t.Errorf("shutdown: %v", response)
	}
	session.send(`{"jsonrpc":"2.0","method":"exit"}`)
	if code := session.wait(); code != 0 {
		t.Errorf("exit code %d, want 0", code)
	}
}

// TestLSPExitWithoutShutdown: the LSP specification wants exit code 1, also
// when the editor just closes the stream.
func TestLSPExitWithoutShutdown(t *testing.T) {
	session := startLSP(t)
	session.input.Close()
	if code := session.wait(); code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
}

func TestLSPUnknownArgument(t *testing.T) {
	result := runCLI(t, "", "lsp", "--tcp")
	if result.exitCode != 2 || !strings.Contains(result.stderr, `unknown argument "--tcp"`) {
		t.Errorf("got %+v, want exit 2 with a message", result)
	}
}
