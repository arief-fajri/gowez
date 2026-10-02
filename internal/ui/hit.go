package ui

// HitTest returns the top-most node at the given viewport coordinates, or
// nil when nothing is hit.
//
// Milestone 2: walk laid-out boxes back-to-front. Geometry comes from the
// layout package — the UI tree stores no rectangles of its own.
func (t *Tree) HitTest(x, y float64) *Node {
	_ = x
	_ = y
	// TODO(M2): back-to-front walk over layout geometry.
	return nil
}
