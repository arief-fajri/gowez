package render

// Renderer executes one frame of render commands against a graphics backend.
//
// Implementations must be bounded: BeginFrame/EndFrame always terminate,
// and a backend failure surfaces as an observable error — never a hang
// (guard rails G-REL-01, G-REL-03).
type Renderer interface {
	// BeginFrame starts a frame of the given size in logical pixels.
	BeginFrame(width, height int)
	// DrawRect fills an axis-aligned rectangle.
	DrawRect(x, y, w, h float64, color Color)
	// DrawText draws a text run at the pen position.
	DrawText(x, y float64, s string, opts TextOptions)
	// ClipRect restricts subsequent drawing to a rectangle.
	ClipRect(x, y, w, h float64)
	// EndFrame completes the frame and presents it.
	EndFrame()
}

// Color is a straight-alpha RGBA color; components are in [0, 1].
type Color struct {
	R, G, B, A float32
}

// TextOptions carries per-draw text attributes.
type TextOptions struct {
	// FontSize is the size in logical pixels.
	FontSize float64
	// Color is the text color.
	Color Color
}
