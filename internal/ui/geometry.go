package ui

// Box is a resolved rectangle in logical pixels.
type Box struct {
	// X, Y are the top-left corner.
	X, Y float64
	// W, H are width and height.
	W, H float64
}

// Contains reports whether the box covers the point. The right and bottom
// edges are exclusive so adjacent boxes never both claim the same pixel
// (hit testing must return exactly one node).
func (b Box) Contains(x, y float64) bool {
	return x >= b.X && x < b.X+b.W && y >= b.Y && y < b.Y+b.H
}

// Geometry is the layout result for one node. The border box includes
// padding and borders; the content box is what text and children lay out
// against (content-box sizing, docs/CSS-SUBSET.md).
type Geometry struct {
	// Border is the border box in viewport coordinates.
	Border Box
	// Content is the content box in viewport coordinates.
	Content Box
}

// Size is a width/height pair in logical pixels.
type Size struct {
	W, H float64
}
