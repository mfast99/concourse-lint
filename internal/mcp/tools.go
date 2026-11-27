package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Protocol versions from which a feature exists.
const (
	// annotationsSince: tool annotations (readOnlyHint, …).
	annotationsSince = "2025-03-26"
	// structuredSince: outputSchema and structuredContent.
	structuredSince = "2025-06-18"
)

// tool is one tool of the server. call returns the result as JSON, or an
// error that the agent sees as a tool error (isError).
type tool struct {
	name        string
	description string
	// inputSchema and outputSchema are JSON Schema objects.
	inputSchema  string
	outputSchema string
	call         func(server *server, arguments json.RawMessage) ([]byte, error)
}

// tools are listed in this order.
func (server *server) tools() []tool {
	return []tool{checkPipelineTool, listRulesTool, explainRuleTool}
}

type toolDefinition struct {
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	InputSchema  json.RawMessage  `json:"inputSchema"`
	OutputSchema json.RawMessage  `json:"outputSchema,omitempty"`
	Annotations  *toolAnnotations `json:"annotations,omitempty"`
}

// toolAnnotations: all tools only read and never reach the network.
type toolAnnotations struct {
	ReadOnlyHint   bool `json:"readOnlyHint"`
	IdempotentHint bool `json:"idempotentHint"`
	OpenWorldHint  bool `json:"openWorldHint"`
}

type toolsListResult struct {
	Tools []toolDefinition `json:"tools"`
}

// listTools answers tools/list with what the agreed protocol version knows.
func (server *server) listTools() toolsListResult {
	result := toolsListResult{Tools: []toolDefinition{}}
	for _, tool := range server.tools() {
		definition := toolDefinition{Name: tool.name, Description: tool.description, InputSchema: json.RawMessage(tool.inputSchema)}
		if server.protocolVersion >= annotationsSince {
			definition.Annotations = &toolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: false}
		}
		if server.protocolVersion >= structuredSince {
			definition.OutputSchema = json.RawMessage(tool.outputSchema)
		}
		result.Tools = append(result.Tools, definition)
	}
	return result
}

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolsCallResult struct {
	Content           []textContent   `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

// callTool answers tools/call. An unknown tool is a protocol error; wrong
// arguments and other failures are a tool error, so the agent can correct
// itself.
func (server *server) callTool(rawParams json.RawMessage) (any, *responseError) {
	var params toolsCallParams
	if err := json.Unmarshal(rawParams, &params); err != nil || params.Name == "" {
		return nil, &responseError{Code: codeInvalidParams, Message: "tools/call needs params with a tool name"}
	}
	for _, tool := range server.tools() {
		if tool.name != params.Name {
			continue
		}
		output, err := tool.call(server, params.Arguments)
		if err != nil {
			return toolsCallResult{Content: []textContent{{Type: "text", Text: err.Error()}}, IsError: true}, nil
		}
		result := toolsCallResult{Content: []textContent{{Type: "text", Text: string(output)}}}
		if server.protocolVersion >= structuredSince {
			result.StructuredContent = output
		}
		return result, nil
	}
	return nil, &responseError{Code: codeInvalidParams, Message: fmt.Sprintf("unknown tool %q", params.Name)}
}

// decodeArguments reads the arguments of a tool into target. Missing
// arguments count as {}; unknown ones and wrong types are an error that
// names the argument.
func decodeArguments(raw json.RawMessage, target any) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		raw = json.RawMessage("{}")
	}
	if raw[0] != '{' {
		return errors.New("the arguments must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	var typeError *json.UnmarshalTypeError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &typeError):
		return fmt.Errorf("argument %q must be of type %s", typeError.Field, jsonTypeName(typeError.Type.Kind().String()))
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return fmt.Errorf("unknown argument %s", strings.TrimPrefix(err.Error(), "json: unknown field "))
	default:
		return fmt.Errorf("the arguments cannot be read: %v", err)
	}
}

// jsonTypeName names a Go kind as the JSON type an agent has to send.
func jsonTypeName(kind string) string {
	switch kind {
	case "slice":
		return "array"
	case "struct", "map":
		return "object"
	case "bool":
		return "boolean"
	default:
		return kind
	}
}

// encodeJSON writes a result like the CLI writes JSON: indented by two
// spaces, < > & as written, a line break at the end.
func encodeJSON(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(value)
	return output.Bytes(), err
}
