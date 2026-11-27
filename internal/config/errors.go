package config

import (
	"errors"
	"fmt"

	"github.com/mfast99/go-yaml"
	"github.com/mfast99/go-yaml/ast"
	"github.com/mfast99/go-yaml/parser"
)

// Error is a problem in a config file. Its text starts with
// file:line:col, like a finding.
type Error struct {
	// Path of the config file, "" for Parse.
	Path string
	// Line and Column of the offending key or value, 0 when unknown.
	Line, Column int
	Message      string
}

func (configError *Error) Error() string {
	text := configError.Message
	if configError.Line > 0 {
		text = fmt.Sprintf("%d:%d: %s", configError.Line, configError.Column, text)
	}
	if configError.Path != "" {
		text = configError.Path + ":" + text
	}
	return text
}

// problem is an error in the content of the config file, before its
// position is known.
type problem struct {
	// path leads from the top of the file to the offending node: a string
	// is a mapping key, an int a list index.
	path []any
	// atKey points at the key of the last path element instead of its
	// value.
	atKey   bool
	message string
}

func (located *problem) Error() string {
	return located.message
}

func problemAt(path []any, format string, arguments ...any) *problem {
	return &problem{path: path, message: fmt.Sprintf(format, arguments...)}
}

func problemAtKey(path []any, format string, arguments ...any) *problem {
	return &problem{path: path, atKey: true, message: fmt.Sprintf(format, arguments...)}
}

// yamlError turns an error of the YAML library into an Error at the
// position it reports.
func yamlError(err error) *Error {
	var libraryError yaml.Error
	if errors.As(err, &libraryError) {
		result := &Error{Message: libraryError.GetMessage()}
		if errorToken := libraryError.GetToken(); errorToken != nil && errorToken.Position != nil {
			result.Line, result.Column = errorToken.Position.Line, errorToken.Position.Column
		}
		return result
	}
	return &Error{Message: err.Error()}
}

// locate gives a problem the position of its node in the source. Without a
// node it stays without a position.
func locate(source []byte, located *problem) *Error {
	result := &Error{Message: located.message}
	file, err := parser.ParseBytes(source, 0)
	if err != nil {
		return result
	}
	key, value := find(documentBody(file), located.path)
	node := value
	if located.atKey && key != nil {
		node = key
	}
	if node != nil && node.GetToken() != nil && node.GetToken().Position != nil {
		result.Line, result.Column = node.GetToken().Position.Line, node.GetToken().Position.Column
	}
	return result
}

// documentBody returns the body of the first document that is not a
// directive: goccy returns a directive (%YAML, %TAG) as a document of its
// own. The same as in internal/parse, which config must not import.
func documentBody(file *ast.File) ast.Node {
	for _, document := range file.Docs {
		if _, isDirective := document.Body.(*ast.DirectiveNode); !isDirective {
			return document.Body
		}
	}
	return nil
}

// find follows the path from node and returns the key (nil for a list
// item) and the value of its last element.
func find(node ast.Node, path []any) (key ast.Node, value ast.Node) {
	value = node
	for _, step := range path {
		key, value = child(value, step)
		if value == nil {
			return nil, nil
		}
	}
	return key, value
}

// child returns the entry with the key (a string) or index (an int) in a
// mapping or list.
func child(node ast.Node, step any) (key ast.Node, value ast.Node) {
	switch typed := node.(type) {
	case *ast.TagNode:
		return child(typed.Value, step)
	case *ast.AnchorNode:
		return child(typed.Value, step)
	case *ast.MappingNode:
		for _, pair := range typed.Values {
			if pairKey, pairValue := child(pair, step); pairValue != nil {
				return pairKey, pairValue
			}
		}
	case *ast.MappingValueNode:
		if name, isName := step.(string); isName && typed.Key != nil && typed.Key.GetToken() != nil && typed.Key.GetToken().Value == name {
			return typed.Key, typed.Value
		}
	case *ast.SequenceNode:
		if index, isIndex := step.(int); isIndex && index >= 0 && index < len(typed.Values) {
			return nil, typed.Values[index]
		}
	}
	return nil, nil
}
