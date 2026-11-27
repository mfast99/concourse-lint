package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// exchange sends the lines to a server, lets it run until the input ends,
// and returns every message it wrote, in order. The server answers one
// request after the other, so the answers come in the order of the lines.
func exchange(t *testing.T, options Options, lines ...string) []map[string]any {
	t.Helper()
	var output bytes.Buffer
	if err := Serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &output, options); err != nil {
		t.Fatalf("Serve = %v, want nil when the input ends", err)
	}
	return readMessages(t, output.String())
}

// readMessages parses the server's output: exactly one JSON object per line.
func readMessages(t *testing.T, output string) []map[string]any {
	t.Helper()
	var messages []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(nil, 64<<20)
	for scanner.Scan() {
		var message map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			t.Fatalf("server wrote a line that is no JSON object: %q", scanner.Text())
		}
		if message["jsonrpc"] != "2.0" {
			t.Errorf("message without jsonrpc 2.0: %v", message)
		}
		messages = append(messages, message)
	}
	return messages
}

// request builds one request line.
func request(id any, method string, params any) string {
	message := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		message["params"] = params
	}
	line, _ := json.Marshal(message)
	return string(line)
}

// notification builds one notification line.
func notification(method string) string {
	line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method})
	return string(line)
}

// initializeRequest asks for a protocol version.
func initializeRequest(id any, protocolVersion string) string {
	return request(id, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1"},
	})
}

// handshake are the first two lines of every session.
func handshake() []string {
	return []string{initializeRequest(0, latestProtocolVersion), notification("notifications/initialized")}
}

// errorCode returns the code of an error response, or 0 for a result.
func errorCode(t *testing.T, message map[string]any) int {
	t.Helper()
	failure, isError := message["error"].(map[string]any)
	if !isError {
		return 0
	}
	code, _ := failure["code"].(float64)
	return int(code)
}

func TestInitializeNegotiatesTheVersion(t *testing.T) {
	cases := map[string]string{
		"2025-11-25": "2025-11-25",
		"2025-06-18": "2025-06-18",
		"2025-03-26": "2025-03-26",
		"2024-11-05": "2024-11-05",
		"2099-01-01": "2025-11-25",
		"":           "2025-11-25",
	}
	for asked, want := range cases {
		messages := exchange(t, Options{Version: "v1.2.3"}, initializeRequest(1, asked))
		if len(messages) != 1 {
			t.Fatalf("asked %q: %d messages, want 1", asked, len(messages))
		}
		result, _ := messages[0]["result"].(map[string]any)
		if result["protocolVersion"] != want {
			t.Errorf("asked %q: protocolVersion = %v, want %s", asked, result["protocolVersion"], want)
		}
	}
}

func TestInitializeResult(t *testing.T) {
	messages := exchange(t, Options{Version: "v1.2.3"}, initializeRequest("first", latestProtocolVersion))
	if messages[0]["id"] != "first" {
		t.Errorf("id = %v, want the id of the request", messages[0]["id"])
	}
	result := messages[0]["result"].(map[string]any)
	serverInfo, _ := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "concourse-lint" || serverInfo["version"] != "v1.2.3" {
		t.Errorf("serverInfo = %v", serverInfo)
	}
	capabilities, _ := result["capabilities"].(map[string]any)
	tools, _ := capabilities["tools"].(map[string]any)
	if listChanged, found := tools["listChanged"]; !found || listChanged != false {
		t.Errorf("capabilities = %v, want tools.listChanged false", capabilities)
	}
	instructions, _ := result["instructions"].(string)
	for _, word := range []string{"check_pipeline", "explain_rule", "suggestion", "error"} {
		if !strings.Contains(instructions, word) {
			t.Errorf("instructions do not mention %s: %q", word, instructions)
		}
	}
}

func TestInitializeTwice(t *testing.T) {
	messages := exchange(t, Options{}, append(handshake(), initializeRequest(1, latestProtocolVersion))...)
	if code := errorCode(t, messages[1]); code != codeInvalidRequest {
		t.Errorf("second initialize: code %d, want %d", code, codeInvalidRequest)
	}
}

func TestInitializeWithoutVersion(t *testing.T) {
	messages := exchange(t, Options{}, request(1, "initialize", map[string]any{"capabilities": map[string]any{}}), request(2, "initialize", "not an object"))
	for index, message := range messages {
		if code := errorCode(t, message); code != codeInvalidParams {
			t.Errorf("message %d: code %d, want %d", index, code, codeInvalidParams)
		}
	}
}

func TestRequestBeforeInitialize(t *testing.T) {
	messages := exchange(t, Options{}, request(1, "tools/list", nil), request(2, "ping", nil))
	if code := errorCode(t, messages[0]); code != codeServerNotInitialized {
		t.Errorf("tools/list before initialize: code %d, want %d", code, codeServerNotInitialized)
	}
	if result, isMap := messages[1]["result"].(map[string]any); !isMap || len(result) != 0 {
		t.Errorf("ping before initialize = %v, want {}", messages[1])
	}
}

func TestPing(t *testing.T) {
	messages := exchange(t, Options{}, append(handshake(), request(7, "ping", nil))...)
	if result, isMap := messages[1]["result"].(map[string]any); !isMap || len(result) != 0 || messages[1]["id"] != float64(7) {
		t.Errorf("ping = %v, want {} for id 7", messages[1])
	}
}

func TestUnknownMethod(t *testing.T) {
	messages := exchange(t, Options{}, append(handshake(), request(1, "resources/list", nil))...)
	if code := errorCode(t, messages[1]); code != codeMethodNotFound {
		t.Errorf("code %d, want %d", code, codeMethodNotFound)
	}
}

// TestNotificationsAreNotAnswered: also unknown ones and cancellations;
// a response from the client is ignored too.
func TestNotificationsAreNotAnswered(t *testing.T) {
	messages := exchange(t, Options{}, append(handshake(),
		notification("notifications/cancelled"),
		notification("notifications/something"),
		`{"jsonrpc":"2.0","id":99,"result":{}}`,
		"",
		"   ",
		request(1, "ping", nil),
	)...)
	if len(messages) != 2 || messages[1]["id"] != float64(1) {
		t.Errorf("messages = %v, want only the answers to initialize and ping", messages)
	}
}

func TestMalformedMessages(t *testing.T) {
	cases := []struct {
		line string
		want int
		// wantID is the id of the answer: nil when the request is unreadable.
		wantID any
	}{
		{line: "not json", want: codeParseError},
		{line: `{"jsonrpc":"2.0","id":1,`, want: codeParseError},
		{line: `[{"jsonrpc":"2.0","id":1,"method":"ping"}]`, want: codeInvalidRequest},
		{line: `"just a string"`, want: codeInvalidRequest},
		{line: `{"jsonrpc":"2.0","id":3}`, want: codeInvalidRequest, wantID: float64(3)},
	}
	for _, testCase := range cases {
		messages := exchange(t, Options{}, append(handshake(), testCase.line, request(2, "ping", nil))...)
		if len(messages) != 3 {
			t.Fatalf("%s: %d messages, want 3 (the server goes on after a broken line)", testCase.line, len(messages))
		}
		if code := errorCode(t, messages[1]); code != testCase.want {
			t.Errorf("%s: code %d, want %d", testCase.line, code, testCase.want)
		}
		if id, found := messages[1]["id"]; !found || id != testCase.wantID {
			t.Errorf("%s: id = %v, want %v", testCase.line, id, testCase.wantID)
		}
	}
}

// TestLastLineWithoutNewline: a request at the very end of the input is
// still answered.
func TestLastLineWithoutNewline(t *testing.T) {
	var output bytes.Buffer
	input := strings.Join(handshake(), "\n") + "\n" + request(1, "ping", nil)
	if err := Serve(strings.NewReader(input), &output, Options{}); err != nil {
		t.Fatal(err)
	}
	if messages := readMessages(t, output.String()); len(messages) != 2 {
		t.Errorf("%d messages, want 2", len(messages))
	}
}

// TestLineTooLong: a line over the limit is dropped with a parse error, and
// the next line is read normally.
func TestLineTooLong(t *testing.T) {
	ping := request(1, "ping", nil)
	input := strings.Repeat("x", 300) + "\n" + ping + "\n" + ping
	connection := newConnection(strings.NewReader(input), &bytes.Buffer{}, 100)
	if _, err := connection.read(); err != errLineTooLong {
		t.Errorf("first read: err = %v, want errLineTooLong", err)
	}
	for index := range 2 {
		line, err := connection.read()
		if err != nil || string(line) != ping {
			t.Errorf("read %d = %q, %v; want the ping", index, line, err)
		}
	}
}
