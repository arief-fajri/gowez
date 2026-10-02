package style

// Display is the layout participation mode of a node.
type Display int

const (
	// DisplayBlock participates as a block box.
	DisplayBlock Display = iota
	// DisplayInline participates as an inline box.
	DisplayInline
	// DisplayFlex lays out children with the flex algorithm.
	DisplayFlex
	// DisplayNone removes the node from layout and rendering.
	DisplayNone
)

// ComputedStyle is the resolved style of one node after cascade and value
// computation (Milestone 2).
type ComputedStyle struct {
	// Display selects the layout mode.
	Display Display
	// Width and Height are resolved content-box sizes; zero means auto.
	Width, Height float64
	// Color is the text/background color as 0xRRGGBBAA.
	Color uint32
	// FontSize is the resolved font size in logical pixels.
	FontSize float64
}
