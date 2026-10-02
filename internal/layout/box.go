package layout

// Box is a resolved rectangle in viewport coordinates.
type Box struct {
	// X, Y are the top-left corner in logical pixels.
	X, Y float64
	// W, H are width and height in logical pixels.
	W, H float64
}
