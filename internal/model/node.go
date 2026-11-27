package model

// NodeKind says which shape a free-form value has.
type NodeKind int

const (
	MappingNode NodeKind = iota + 1
	ListNode
	ScalarNode
)

// Node is a free-form value such as source:, params: or vars:. Concourse
// passes these on to resources and tasks without
// knowing their fields, so the model keeps them as a tree.
type Node struct {
	Kind NodeKind
	// Start of the value: the first key of a block mapping, the "-" of the
	// first item of a block list, the "{" or "[" of a flow collection, or the
	// scalar itself.
	Position Position

	// MappingNode: the key/value pairs in file order.
	Pairs []*NodePair
	// ListNode: the items.
	Items []*Node
	// ScalarNode: the text without quotes, and whether it was quoted.
	Text   string
	Quoted bool
}

// Boolean reads the node the way Concourse does: an unquoted scalar from the
// YAML 1.1 table, e.g. "skip_ssl_verification: yes". A quoted value or a
// missing node is no boolean.
func (node *Node) Boolean() (value bool, isBoolean bool) {
	if node == nil || node.Kind != ScalarNode || node.Quoted {
		return false, false
	}
	return YAML11Boolean(node.Text)
}

// NodePair is one key/value pair of a mapping node.
type NodePair struct {
	Key         string
	KeyPosition Position
	Value       *Node
}
