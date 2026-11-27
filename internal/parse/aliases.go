package parse

import (
	"fmt"

	"github.com/mfast99/go-yaml/ast"

	"github.com/mfast99/concourse-lint/internal/analysis"
)

// expansionLimit is the largest number of values the file may have once
// every alias is expanded. Real pipelines stay far below it; an alias bomb
// ("billion laughs") goes far above it with a few lines.
const expansionLimit = 1_000_000

// aliasResolver walks the AST once, in file order, before the decoder runs.
// It finds the anchor every alias refers to and computes how large the file
// would be with all aliases expanded, without expanding anything. goccy does
// not check aliases itself.
type aliasResolver struct {
	// The latest definition of every anchor name seen so far. An alias
	// refers to the latest definition above it, as in Concourse.
	anchors map[string]*ast.AnchorNode
	// Anchors whose value is being walked right now; an alias to one of them
	// would contain itself.
	open map[*ast.AnchorNode]bool
	// Expanded size of every anchor value that is complete.
	sizes map[*ast.AnchorNode]int
	// The result: the value every alias stands for.
	targets map[*ast.AliasNode]ast.Node
	// Values counted so far, aliases expanded.
	expandedTotal int
	// The first problem found; the walk stops there.
	problem *analysis.Finding
}

// resolveAliases returns the value every alias stands for. If an alias
// cannot be resolved or the aliases expand too far, it returns a CL000
// finding instead: Concourse rejects such a file as a whole.
func resolveAliases(body ast.Node) (map[*ast.AliasNode]ast.Node, *analysis.Finding) {
	resolver := &aliasResolver{
		anchors: map[string]*ast.AnchorNode{},
		open:    map[*ast.AnchorNode]bool{},
		sizes:   map[*ast.AnchorNode]int{},
		targets: map[*ast.AliasNode]ast.Node{},
	}
	resolver.walk(body)
	return resolver.targets, resolver.problem
}

// walk visits a node and everything below it and returns its size with all
// aliases expanded.
func (resolver *aliasResolver) walk(node ast.Node) int {
	if node == nil || resolver.problem != nil {
		return 0
	}
	switch typed := node.(type) {
	case *ast.AnchorNode:
		return resolver.anchor(typed)
	case *ast.AliasNode:
		return resolver.alias(typed)
	}
	size := 1
	resolver.count(1, node)
	switch typed := node.(type) {
	case *ast.MappingNode:
		for _, pair := range typed.Values {
			size += resolver.walk(pair)
		}
	case *ast.MappingValueNode:
		size += resolver.walk(typed.Key) + resolver.walk(typed.Value)
	case *ast.MappingKeyNode:
		size += resolver.walk(typed.Value)
	case *ast.SequenceNode:
		for _, item := range typed.Values {
			size += resolver.walk(item)
		}
	case *ast.TagNode:
		size += resolver.walk(typed.Value)
	}
	return size
}

// anchor registers the anchor before walking its value: an alias inside the
// value to the same name refers to this anchor, and so contains itself.
func (resolver *aliasResolver) anchor(anchor *ast.AnchorNode) int {
	resolver.anchors[anchor.Name.GetToken().Value] = anchor
	resolver.open[anchor] = true
	size := resolver.walk(anchor.Value)
	delete(resolver.open, anchor)
	resolver.sizes[anchor] = size
	return size
}

func (resolver *aliasResolver) alias(alias *ast.AliasNode) int {
	name := alias.Value.GetToken().Value
	anchor, known := resolver.anchors[name]
	switch {
	case !known:
		resolver.fail(alias, fmt.Sprintf("unknown anchor %q: an alias needs an anchor (&%s) defined above it", name, name))
		return 0
	case resolver.open[anchor]:
		resolver.fail(alias, fmt.Sprintf("anchor %q contains an alias to itself", name))
		return 0
	}
	resolver.targets[alias] = anchor.Value
	size := resolver.sizes[anchor]
	resolver.count(size, alias)
	return size
}

// count adds expanded values to the total and stops the walk at the node
// where the total goes over the limit.
func (resolver *aliasResolver) count(values int, node ast.Node) {
	resolver.expandedTotal += values
	if resolver.expandedTotal > expansionLimit && resolver.problem == nil {
		resolver.fail(node, fmt.Sprintf("aliases expand to more than %d values (alias bomb)", expansionLimit))
	}
}

func (resolver *aliasResolver) fail(node ast.Node, message string) {
	if resolver.problem != nil {
		return
	}
	resolver.problem = &analysis.Finding{
		Rule:     "CL000",
		Severity: analysis.SeverityError,
		Position: position(node),
		Message:  "YAML error: " + message,
	}
}
