package render

// Command is one immutable draw instruction. Commands are the boundary
// between the UI/layout engines and any graphics backend.
type Command interface {
	isCommand()
}

// DrawRect fills an axis-aligned rectangle.
type DrawRect struct {
	// X, Y, W, H are in logical pixels.
	X, Y, W, H float64
	// Color is the fill color.
	Color Color
}

func (DrawRect) isCommand() {}

// DrawText draws a shaped text run at the given pen position.
type DrawText struct {
	// X, Y are the pen position in logical pixels.
	X, Y float64
	// Text is the string to draw.
	Text string
	// Options carry per-draw text attributes.
	Options TextOptions
}

func (DrawText) isCommand() {}

// ClipRect restricts subsequent drawing to a rectangle.
type ClipRect struct {
	// X, Y, W, H are in logical pixels.
	X, Y, W, H float64
}

func (ClipRect) isCommand() {}

// Transform applies a coordinate transform to subsequent drawing.
// Milestone 2+: translate/scale fields arrive with animation support
// (complex animation is explicitly out of MVP scope).
type Transform struct{}

func (Transform) isCommand() {}
