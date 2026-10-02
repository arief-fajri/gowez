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
	// Parent is the owning node; nil for roots.
	Parent *Node
	// Children are child nodes in document order.
	Children []*Node
}
