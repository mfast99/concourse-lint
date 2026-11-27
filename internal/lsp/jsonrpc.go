package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// maxMessageSize guards against a broken Content-Length: no pipeline comes
// near it.
const maxMessageSize = 64 << 20

// JSON-RPC error codes used by the server.
const (
	codeParseError           = -32700
	codeInvalidRequest       = -32600
	codeMethodNotFound       = -32601
	codeInvalidParams        = -32602
	codeServerNotInitialized = -32002
)

// incoming is a request (with ID) or a notification (without) from the
// client. Responses from the client are read the same way: they have no
// method and are ignored.
type incoming struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

func (message incoming) isRequest() bool {
	return len(message.ID) > 0 && string(message.ID) != "null"
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// successResponse always has a result, also null (e.g. for shutdown).
type successResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
}

type errorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   responseError   `json:"error"`
}

// request is a request from the server to the client.
type request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// connection reads and writes LSP messages: a header with Content-Length,
// an empty line, then the JSON body. Writing is safe from several
// goroutines (the debounce timers publish on their own).
type connection struct {
	reader      *bufio.Reader
	writer      io.Writer
	writerMutex sync.Mutex
}

func newConnection(input io.Reader, output io.Writer) *connection {
	return &connection{reader: bufio.NewReader(input), writer: output}
}

// errMalformed marks a message that could not be read; the stream itself is
// still usable.
var errMalformed = errors.New("malformed message")

// read returns the next message. io.EOF means the client closed the stream.
func (connection *connection) read() (incoming, error) {
	length := -1
	for {
		line, err := connection.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" && length < 0 {
				return incoming{}, io.EOF
			}
			return incoming{}, fmt.Errorf("reading header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if found && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || parsed < 0 || parsed > maxMessageSize {
				return incoming{}, fmt.Errorf("invalid Content-Length %q", value)
			}
			length = parsed
		}
	}
	if length < 0 {
		return incoming{}, errors.New("header without Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(connection.reader, body); err != nil {
		return incoming{}, fmt.Errorf("reading body: %w", err)
	}
	var message incoming
	if err := json.Unmarshal(body, &message); err != nil {
		return incoming{}, fmt.Errorf("%w: %v", errMalformed, err)
	}
	return message, nil
}

func (connection *connection) write(message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	connection.writerMutex.Lock()
	defer connection.writerMutex.Unlock()
	if _, err := fmt.Fprintf(connection.writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = connection.writer.Write(body)
	return err
}

func (connection *connection) respond(id json.RawMessage, result any) error {
	return connection.write(successResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (connection *connection) respondError(id json.RawMessage, code int, message string) error {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return connection.write(errorResponse{JSONRPC: "2.0", ID: id, Error: responseError{Code: code, Message: message}})
}

// request sends a request to the client. The server does not wait for the
// response; it is read like any message and dropped.
func (connection *connection) request(id, method string, params any) error {
	return connection.write(request{JSONRPC: "2.0", ID: id, Method: method, Params: params})
}

func (connection *connection) notify(method string, params any) error {
	return connection.write(notification{JSONRPC: "2.0", Method: method, Params: params})
}
