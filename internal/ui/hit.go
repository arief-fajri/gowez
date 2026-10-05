package ui

// SetGeometry stores the layout result for the tree. The layout package
// produces geometry keyed by NodeID; hit testing reads it back, so the
// tree never holds rectangles of its own beyond this snapshot.
//
// Replacing the whole map at once keeps hit testing consistent with one
// layout pass (invariant I1: no half-applied update).
func (t *Tree) SetGeometry(geom map[NodeID]Geometry) {
	t.geom = geom
}

// Geometry returns the stored layout snapshot (nil before the first
// layout pass).
func (t *Tree) Geometry() map[NodeID]Geometry {
	return t.geom
}

// HitTest returns the top-most node at the given viewport coordinates, or
// nil when nothing is hit.
//
// The walk visits children before their parent and later siblings before
// earlier ones — the reverse of paint order — so the first node whose
// border box covers the point is the one rendered on top. Nodes without
// geometry (display: none, or never laid out) are skipped.
func (t *Tree) HitTest(x, y float64) *Node {
	for i := len(t.roots) - 1; i >= 0; i-- {
		if n := hitNode(t.roots[i], t.geom, x, y); n != nil {
			return n
		}
	}
	return nil
}

// hitNode performs the reverse paint-order descent described in HitTest.
func hitNode(n *Node, geom map[NodeID]Geometry, x, y float64) *Node {
	for i := len(n.Children) - 1; i >= 0; i-- {
		if hit := hitNode(n.Children[i], geom, x, y); hit != nil {
			return hit
		}
	}
	g, ok := geom[n.ID]
	if !ok {
		return nil
	}
	if g.Border.Contains(x, y) {
		return n
	}
	return nil
}
