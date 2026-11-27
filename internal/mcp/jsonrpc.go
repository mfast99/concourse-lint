package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// maxLineSize guards against a client that never sends a newline: no
// pipeline comes near it.
const maxLineSize = 64 << 20

// JSON-RPC error codes used by the server.
const (
	codeParseError           = -32700
	codeInvalidRequest       = -32600
	codeMethodNotFound       = -32601
	codeInvalidParams        = -32602
	codeServerNotInitialized = -32002
)

// errLineTooLong marks a line over the limit. It is dropped; the stream is
// still usable.
var errLineTooLong = errors.New("message longer than 64 MiB")

// nullID is the id of an answer to a message whose id could not be read.
var nullID = json.RawMessage("null")

// incoming is a request (with id), a notification (without) or a response
// from the client (result or error, ignored).
type incoming struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

func (message incoming) isRequest() bool {
	return len(message.ID) > 0 && string(message.ID) != "null"
}

func (message incoming) isResponse() bool {
	return message.Method == "" && (len(message.Result) > 0 || len(message.Error) > 0)
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type successResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result"`
}

type errorResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   *responseError  `json:"error"`
}

// connection reads and writes MCP messages over stdio: one JSON-RPC
// message per line, no header (unlike the LSP).
type connection struct {
	reader *bufio.Reader
	writer io.Writer
	// limit is the longest line in bytes (maxLineSize; smaller in tests).
	limit int
}

func newConnection(input io.Reader, output io.Writer, limit int) *connection {
	return &connection{reader: bufio.NewReader(input), writer: output, limit: limit}
}

// read returns the next line without its line break. A last line without a
// line break counts too. io.EOF means the client closed the stream.
func (connection *connection) read() ([]byte, error) {
	var line []byte
	tooLong := false
	for {
		chunk, err := connection.reader.ReadSlice('\n')
		if !tooLong {
			if len(line)+len(chunk) > connection.limit {
				tooLong = true
				line = nil
			} else {
				line = append(line, chunk...)
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if tooLong {
			return nil, errLineTooLong
		}
		if errors.Is(err, io.EOF) && len(line) == 0 {
			return nil, io.EOF
		}
		return bytes.TrimRight(line, "\r\n"), nil
	}
}

// write sends one message on its own line. Without indentation,
// encoding/json never writes a line break inside a value (a raw message is
// compacted too); Encode adds the line break at the end. < > & stay as
// written, as in the CLI's JSON.
func (connection *connection) write(message any) error {
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(message); err != nil {
		return err
	}
	_, err := connection.writer.Write(body.Bytes())
	return err
}

func (connection *connection) respond(id json.RawMessage, result any) error {
	return connection.write(successResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (connection *connection) respondError(id json.RawMessage, failure *responseError) error {
	if len(id) == 0 {
		id = nullID
	}
	return connection.write(errorResponse{JSONRPC: "2.0", ID: id, Error: failure})
}

// decode reads one line into a message. A line that is no JSON is a parse
// error; JSON that is no single object (e.g. a batch) is an invalid request.
func decode(line []byte) (incoming, *responseError) {
	var message incoming
	if !json.Valid(line) {
		return message, &responseError{Code: codeParseError, Message: "the message is no valid JSON"}
	}
	if line[0] != '{' {
		return message, &responseError{Code: codeInvalidRequest, Message: "the message must be one JSON object (batches are not supported)"}
	}
	if err := json.Unmarshal(line, &message); err != nil {
		return incoming{}, &responseError{Code: codeInvalidRequest, Message: "the message is no JSON-RPC message: " + err.Error()}
	}
	return message, nil
}
