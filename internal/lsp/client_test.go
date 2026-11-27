package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testClient talks to a server through pipes, like an editor.
type testClient struct {
	t        *testing.T
	toServer *io.PipeWriter
	reader   *bufio.Reader
	nextID   int
	done     chan error
	// messages are read on their own goroutine, so that a server that
	// writes while the test writes cannot block.
	messages chan map[string]any
}

func startServer(t *testing.T, options Options) *testClient {
	t.Helper()
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()
	client := &testClient{
		t: t, toServer: clientWriter, reader: bufio.NewReader(clientReader),
		done: make(chan error, 1), messages: make(chan map[string]any, 1000),
	}
	go func() {
		client.done <- Serve(serverReader, serverWriter, options)
		serverWriter.Close()
	}()
	go client.readLoop()
	t.Cleanup(func() { clientWriter.Close() })
	return client
}

func (client *testClient) readLoop() {
	defer close(client.messages)
	for {
		length := -1
		for {
			line, err := client.reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if value, found := strings.CutPrefix(line, "Content-Length: "); found {
				length, _ = strconv.Atoi(value)
			}
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(client.reader, body); err != nil {
			return
		}
		var message map[string]any
		if err := json.Unmarshal(body, &message); err != nil {
			client.t.Errorf("server sent invalid JSON: %s", body)
			return
		}
		client.messages <- message
	}
}

func (client *testClient) send(message map[string]any) {
	client.t.Helper()
	message["jsonrpc"] = "2.0"
	body, err := json.Marshal(message)
	if err != nil {
		client.t.Fatal(err)
	}
	if _, err := fmt.Fprintf(client.toServer, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		client.t.Fatal(err)
	}
}

func (client *testClient) notify(method string, params any) {
	client.t.Helper()
	client.send(map[string]any{"method": method, "params": params})
}

// request sends a request and returns its response; notifications that
// arrive before it are handed to onNotification.
func (client *testClient) request(method string, params any, onNotification func(map[string]any)) map[string]any {
	client.t.Helper()
	client.nextID++
	id := client.nextID
	client.send(map[string]any{"id": id, "method": method, "params": params})
	for {
		message := client.next()
		if message == nil {
			client.t.Fatalf("%s: the server closed the stream", method)
		}
		if responseID, isResponse := message["id"].(float64); isResponse && int(responseID) == id {
			return message
		}
		if onNotification != nil {
			onNotification(message)
		}
	}
}

func (client *testClient) next() map[string]any {
	select {
	case message := <-client.messages:
		return message
	case <-time.After(5 * time.Second):
		client.t.Fatal("no message from the server within 5 s")
		return nil
	}
}

func (client *testClient) initialize() map[string]any {
	client.t.Helper()
	return client.request("initialize", map[string]any{"capabilities": map[string]any{}}, nil)
}

// sync sends a request the server does not know and collects the
// diagnostics published before its answer. With Delay 0 every check of the
// earlier notifications is done by then.
func (client *testClient) sync() map[string]PublishDiagnosticsParams {
	client.t.Helper()
	published := map[string]PublishDiagnosticsParams{}
	client.request("concourse-lint/sync", nil, func(message map[string]any) {
		if message["method"] != "textDocument/publishDiagnostics" {
			return
		}
		encoded, _ := json.Marshal(message["params"])
		var params PublishDiagnosticsParams
		if err := json.Unmarshal(encoded, &params); err != nil {
			client.t.Fatal(err)
		}
		published[params.URI] = params
	})
	return published
}

func (client *testClient) open(uri, languageID, text string) {
	client.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": languageID, "version": 1, "text": text},
	})
}

func (client *testClient) change(uri string, version int, text string) {
	client.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": version},
		"contentChanges": []any{map[string]any{"text": text}},
	})
}

// shutdown ends the session properly and returns what Serve returned.
func (client *testClient) shutdown() error {
	client.t.Helper()
	client.request("shutdown", nil, nil)
	client.notify("exit", nil)
	select {
	case err := <-client.done:
		return err
	case <-time.After(5 * time.Second):
		client.t.Fatal("the server did not exit")
		return nil
	}
}
