package ui

import "strings"

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

// HasClass reports whether the node's class attribute contains token.
//
// The class attribute is a whitespace-separated token list, not a single value,
// so it must be compared per token: an equality test on the whole attribute
// silently stops matching as soon as anything else joins the list. Component
// style scoping appends a scope class to every element, which is exactly what
// breaks such a test. Matching mirrors internal/style, which tokenizes the same
// way.
func (n *Node) HasClass(token string) bool {
	if token == "" {
		return false
	}
	for _, have := range strings.Fields(n.GetAttribute("class")) {
		if have == token {
			return true
		}
	}
	return false
}
