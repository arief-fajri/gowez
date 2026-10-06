package ui

// NodeID identifies a node within a Tree.
type NodeID int

// NodeKind distinguishes element and text nodes.
type NodeKind int

const (
	// ElementNode is a tag-bearing element.
	ElementNode NodeKind = iota
	// TextNode is a leaf holding text content.
	TextNode
)

// StateBits are the interaction states a node can carry (Milestone 3).
// The runtime mutates them from pointer/keyboard input; the style
// package matches pseudo-classes (docs/CSS-SUBSET.md) against them.
type StateBits uint8

const (
	// StateHovered is set on the node under the pointer and its
	// ancestors, cleared when the pointer leaves.
	StateHovered StateBits = 1 << iota
	// StatePressed is set on the press target and its ancestors until
	// the button is released.
	StatePressed
	// StateFocused is set on the single node holding keyboard focus.
	StateFocused
)

// Node is one element of the UI tree.
type Node struct {
	// ID is unique within the owning Tree.
	ID NodeID
	// Kind selects element vs text semantics.
	Kind NodeKind
	// Tag is the element name (element nodes only).
	Tag string
	// Text is the content (text nodes only).
	Text string
	// Attrs are the node's attributes ("class", "id", "style", …).
	// Lazily allocated; nil means no attributes.
	Attrs map[string]string
	// Parent is the owning node; nil for roots.
	Parent *Node
	// Children are child nodes in document order.
	Children []*Node
	// State holds hover/press/focus bits (Milestone 3). It is
	// runtime-owned: input methods mutate it, style reads it.
	State StateBits
}

// SetAttribute sets attribute name to value, replacing any previous value.
func (n *Node) SetAttribute(name, value string) {
	if n.Attrs == nil {
		n.Attrs = make(map[string]string, 2)
	}
	n.Attrs[name] = value
}

// GetAttribute returns the value of attribute name, or "" when absent.
func (n *Node) GetAttribute(name string) string {
	if n.Attrs == nil {
		return ""
	}
	return n.Attrs[name]
}
