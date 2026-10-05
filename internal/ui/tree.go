package ui

import "fmt"

// Tree owns node allocation and structural mutation. A tree is not safe for
// concurrent mutation; the runtime serializes UI updates on one goroutine.
type Tree struct {
	nextID NodeID
	roots  []*Node
	geom   map[NodeID]Geometry
}

// NewTree creates an empty UI tree.
func NewTree() *Tree {
	return &Tree{nextID: 1}
}

// Roots returns the root nodes in document order.
func (t *Tree) Roots() []*Node {
	return t.roots
}

// CreateElement allocates an element node owned by the tree.
func (t *Tree) CreateElement(tag string) *Node {
	n := &Node{ID: t.nextID, Kind: ElementNode, Tag: tag}
	t.nextID++
	return n
}

// CreateText allocates a text node owned by the tree.
func (t *Tree) CreateText(text string) *Node {
	n := &Node{ID: t.nextID, Kind: TextNode, Text: text}
	t.nextID++
	return n
}

// AppendRoot adds a node as a root of the tree.
func (t *Tree) AppendRoot(n *Node) error {
	if n == nil {
		return fmt.Errorf("ui: cannot append nil root")
	}
	if n.Parent != nil {
		return fmt.Errorf("ui: node %d already has a parent", n.ID)
	}
	t.roots = append(t.roots, n)
	return nil
}

// Append attaches child under parent. A node has at most one parent and
// may not be both a root and a child; reparenting must go through Detach
// first so the tree cannot corrupt (invariant I1: failed updates never
// leave partial structure).
func (t *Tree) Append(parent, child *Node) error {
	if parent == nil || child == nil {
		return fmt.Errorf("ui: Append requires non-nil parent and child")
	}
	if child.Parent != nil {
		return fmt.Errorf("ui: node %d already has a parent; detach first", child.ID)
	}
	if t.isRoot(child) {
		return fmt.Errorf("ui: node %d is a root; a node cannot be a root and a child", child.ID)
	}
	if isAncestor(child, parent) {
		return fmt.Errorf("ui: cannot append ancestor %d under %d", child.ID, parent.ID)
	}
	parent.Children = append(parent.Children, child)
	child.Parent = parent
	return nil
}

// isRoot reports whether n is in the tree's root list.
func (t *Tree) isRoot(n *Node) bool {
	for _, r := range t.roots {
		if r == n {
			return true
		}
	}
	return false
}

// Detach removes n from its parent. Detaching a root is a no-op error.
func (t *Tree) Detach(n *Node) error {
	if n == nil {
		return fmt.Errorf("ui: cannot detach nil node")
	}
	if n.Parent == nil {
		return fmt.Errorf("ui: node %d has no parent", n.ID)
	}
	p := n.Parent
	for i, c := range p.Children {
		if c == n {
			p.Children = append(p.Children[:i], p.Children[i+1:]...)
			n.Parent = nil
			return nil
		}
	}
	return fmt.Errorf("ui: node %d not found under parent %d", n.ID, p.ID)
}

// isAncestor reports whether ancestor appears in n's parent chain.
func isAncestor(ancestor, n *Node) bool {
	for cur := n; cur != nil; cur = cur.Parent {
		if cur == ancestor {
			return true
		}
	}
	return false
}
