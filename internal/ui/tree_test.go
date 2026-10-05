package ui

import "testing"

// TestTreeMutation covers the structural invariants of the tree — a
// failed update never leaves partial structure (I1, G-DATA-01).
func TestTreeMutation(t *testing.T) {
	t.Parallel()
	tree := NewTree()
	root := tree.CreateElement("div")
	child := tree.CreateElement("span")

	if err := tree.AppendRoot(root); err != nil {
		t.Fatalf("AppendRoot: %v", err)
	}
	if err := tree.AppendRoot(nil); err == nil {
		t.Error("AppendRoot(nil) must fail")
	}
	if err := tree.Append(root, nil); err == nil {
		t.Error("Append(nil child) must fail")
	}
	if err := tree.Append(nil, child); err == nil {
		t.Error("Append(nil parent) must fail")
	}

	if err := tree.Append(root, child); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if child.Parent != root || len(root.Children) != 1 {
		t.Fatalf("append did not link: parent=%v children=%d", child.Parent, len(root.Children))
	}
	// An attached node can never become a root.
	if err := tree.AppendRoot(child); err == nil {
		t.Error("AppendRoot of an attached node must fail")
	}
	// A root can never become a child (a node is one or the other).
	other := tree.CreateElement("aside")
	if err := tree.AppendRoot(other); err != nil {
		t.Fatalf("AppendRoot other: %v", err)
	}
	if err := tree.Append(other, root); err == nil {
		t.Error("appending a root node as a child must fail")
	}
	if len(other.Children) != 0 {
		t.Fatalf("failed root-as-child attempt mutated children: %d", len(other.Children))
	}
	// A node has at most one parent: appending again must fail and leave
	// the existing structure untouched.
	if err := tree.Append(root, child); err == nil {
		t.Error("double Append must fail")
	}
	if len(root.Children) != 1 {
		t.Fatalf("failed append mutated children: %d", len(root.Children))
	}

	// Cycles are rejected: a node may not become its own ancestor.
	grand := tree.CreateElement("b")
	if err := tree.Append(child, grand); err != nil {
		t.Fatalf("Append grand: %v", err)
	}
	if err := tree.Append(grand, root); err == nil {
		t.Error("appending an ancestor under its descendant must fail")
	}
	if len(grand.Children) != 0 {
		t.Fatalf("cycle attempt mutated children: %d", len(grand.Children))
	}

	// Detach, then reparent — the only legal route.
	if err := tree.Detach(child); err != nil {
		t.Fatalf("Detach: %v", err)
	}
	if child.Parent != nil || len(root.Children) != 0 {
		t.Fatalf("detach left links: parent=%v children=%d", child.Parent, len(root.Children))
	}
	if err := tree.Detach(child); err == nil {
		t.Error("detaching a parentless node must fail")
	}
	if err := tree.Detach(root); err == nil {
		t.Error("detaching a root must fail")
	}
	second := tree.CreateElement("section")
	if err := tree.AppendRoot(second); err != nil {
		t.Fatalf("AppendRoot second: %v", err)
	}
	if err := tree.Append(second, child); err != nil {
		t.Fatalf("reparent after detach: %v", err)
	}
	if child.Parent != second {
		t.Fatalf("child parent = %v, want second", child.Parent)
	}
}
