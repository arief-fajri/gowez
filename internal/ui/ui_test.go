package ui

import "testing"

func TestAttributes(t *testing.T) {
	t.Parallel()
	n := &Node{ID: 1}
	if got := n.GetAttribute("class"); got != "" {
		t.Fatalf("GetAttribute on empty node = %q, want \"\"", got)
	}
	n.SetAttribute("class", "card primary")
	n.SetAttribute("id", "title")
	if got := n.GetAttribute("class"); got != "card primary" {
		t.Fatalf("class = %q, want %q", got, "card primary")
	}
	n.SetAttribute("class", "plain")
	if got := n.GetAttribute("class"); got != "plain" {
		t.Fatalf("overwritten class = %q, want %q", got, "plain")
	}
	if got := n.GetAttribute("id"); got != "title" {
		t.Fatalf("id = %q, want %q", got, "title")
	}
}

// TestGeometrySnapshot replaces the whole map at once; a hit test never
// sees a mix of old and new boxes (invariant I1).
func TestHasClass(t *testing.T) {
	t.Parallel()
	n := &Node{ID: 1}

	// An empty or absent class attribute contains no token.
	if n.HasClass("card") {
		t.Fatal("HasClass matched on a node with no class attribute")
	}
	// The empty token must never match: it would otherwise match every node.
	if n.HasClass("") {
		t.Fatal("HasClass(\"\") matched; the empty token must never match")
	}

	n.SetAttribute("class", "card primary")
	if !n.HasClass("card") {
		t.Error("HasClass(card) = false, want true")
	}
	if !n.HasClass("primary") {
		t.Error("HasClass(primary) = false, want true")
	}
	// A class must match a whole token, not a prefix or a substring.
	if n.HasClass("car") {
		t.Error("HasClass(car) matched the prefix of card")
	}
	if n.HasClass("primary-extra") {
		t.Error("HasClass(primary-extra) matched a superstring of primary")
	}
	if n.HasClass("missing") {
		t.Error("HasClass(missing) = true, want false")
	}

	// The scoped form the adapter emits: the scope is an added token, not a
	// replacement, so an author's class must still match.
	n.SetAttribute("class", "card s-App")
	if !n.HasClass("card") {
		t.Error("HasClass(card) = false after scoping appended s-App")
	}
	if !n.HasClass("s-App") {
		t.Error("HasClass(s-App) = false, want true")
	}
	if n.HasClass("card s-App") {
		t.Error("HasClass matched the whole attribute value as one token")
	}

	// Extra whitespace is a separator, not part of a token.
	n.SetAttribute("class", "  card   s-App ")
	if !n.HasClass("card") || !n.HasClass("s-App") {
		t.Error("HasClass did not tolerate surrounding/duplicated whitespace")
	}
}

func TestGeometrySnapshot(t *testing.T) {
	t.Parallel()
	tree := NewTree()
	root := tree.CreateElement("div")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	if tree.Geometry() != nil {
		t.Fatal("Geometry before layout = non-nil, want nil")
	}
	geom := map[NodeID]Geometry{
		root.ID: {Border: Box{X: 0, Y: 0, W: 100, H: 50}},
	}
	tree.SetGeometry(geom)
	if got := tree.Geometry(); len(got) != 1 {
		t.Fatalf("Geometry len = %d, want 1", len(got))
	}
	// A second pass replaces, never merges.
	tree.SetGeometry(map[NodeID]Geometry{
		root.ID: {Border: Box{X: 10, Y: 10, W: 20, H: 20}},
	})
	if got := tree.Geometry()[root.ID]; got.Border.W != 20 {
		t.Fatalf("geometry not replaced: %+v", got)
	}
}

// buildHitTree creates:
//
//	root
//	├── a (0,0 100x50)
//	│   └── achild (10,10 40x20)
//	└── b (60,0 40x50)   ← sibling painted after a, so b wins the overlap
func buildHitTree(t *testing.T) (*Tree, *Node, *Node, *Node) {
	t.Helper()
	tree := NewTree()
	root := tree.CreateElement("div")
	a := tree.CreateElement("div")
	achild := tree.CreateElement("button")
	b := tree.CreateElement("div")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]*Node{{root, a}, {a, achild}, {root, b}} {
		if err := tree.Append(pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	tree.SetGeometry(map[NodeID]Geometry{
		root.ID:   {Border: Box{X: 0, Y: 0, W: 100, H: 50}},
		a.ID:      {Border: Box{X: 0, Y: 0, W: 100, H: 50}},
		achild.ID: {Border: Box{X: 10, Y: 10, W: 40, H: 20}},
		b.ID:      {Border: Box{X: 60, Y: 0, W: 40, H: 50}},
	})
	return tree, root, a, b
}

func TestHitTestTopMostWins(t *testing.T) {
	t.Parallel()
	tree, _, a, b := buildHitTree(t)

	if got := tree.HitTest(20, 20); got == nil || got.Tag != "button" {
		t.Fatalf("HitTest(20,20) = %v, want the button child", got)
	}
	// b is painted after a, so in the a/b overlap b is on top.
	if got := tree.HitTest(70, 40); got != b {
		t.Fatalf("HitTest(70,40) = %v, want b", got)
	}
	// Inside a but outside its child.
	if got := tree.HitTest(5, 40); got != a {
		t.Fatalf("HitTest(5,40) = %v, want a", got)
	}
	// Outside everything.
	if got := tree.HitTest(150, 150); got != nil {
		t.Fatalf("HitTest(150,150) = %v, want nil", got)
	}
}

func TestHitTestEdgesExclusive(t *testing.T) {
	t.Parallel()
	tree, _, _, b := buildHitTree(t)

	// Right/bottom edges belong to nobody: adjacent boxes must not both
	// claim a pixel.
	if got := tree.HitTest(100, 25); got != nil {
		t.Fatalf("HitTest(100,25) = %v, want nil (right edge exclusive)", got)
	}
	if got := tree.HitTest(50, 50); got != nil {
		t.Fatalf("HitTest(50,50) = %v, want nil (bottom edge exclusive)", got)
	}
	// Last interior pixel falls inside b (painted after root's own box).
	if got := tree.HitTest(99.9, 49.9); got != b {
		t.Fatalf("HitTest(99.9,49.9) = %v, want b", got)
	}
}

// TestHitTestSkipsUnlaidOutNodes proves display:none nodes (no geometry
// entry) are invisible to hit testing even when they cover the point.
func TestHitTestSkipsUnlaidOutNodes(t *testing.T) {
	t.Parallel()
	tree, root, a, b := buildHitTree(t)
	// Simulate display:none on a: layout emits no geometry for a node or
	// anything below it, so both entries disappear.
	geom := tree.Geometry()
	delete(geom, a.ID)
	delete(geom, a.Children[0].ID)
	tree.SetGeometry(geom)

	if got := tree.HitTest(20, 20); got != root {
		t.Fatalf("HitTest(20,20) = %v, want root (a has no geometry)", got)
	}
	if got := tree.HitTest(70, 40); got != b {
		t.Fatalf("HitTest(70,40) = %v, want b", got)
	}
}

func TestHitTestWithoutLayoutReturnsNil(t *testing.T) {
	t.Parallel()
	tree := NewTree()
	root := tree.CreateElement("div")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	if got := tree.HitTest(1, 1); got != nil {
		t.Fatalf("HitTest before layout = %v, want nil", got)
	}
}
