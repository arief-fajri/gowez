package software

import "github.com/volantisfrontend/gowez/internal/render"

// Renderer is the CPU reference backend. It records commands into a Frame
// so tests can assert on exact draw output (tests/golden).
type Renderer struct {
	frame render.Frame
}

// Compile-time proof that the backend satisfies the render contract.
var _ render.Renderer = (*Renderer)(nil)

// New creates a software renderer.
func New() *Renderer {
	return &Renderer{}
}

// BeginFrame resets the recorded frame.
func (r *Renderer) BeginFrame(width, height int) {
	r.frame = render.Frame{Width: width, Height: height}
}

// DrawRect records a rectangle fill.
func (r *Renderer) DrawRect(x, y, w, h float64, color render.Color) {
	r.frame.Commands = append(r.frame.Commands, render.DrawRect{
		X: x, Y: y, W: w, H: h,
		Color: color,
	})
}

// DrawText records a text draw.
func (r *Renderer) DrawText(x, y float64, s string, opts render.TextOptions) {
	r.frame.Commands = append(r.frame.Commands, render.DrawText{
		X: x, Y: y,
		Text:    s,
		Options: opts,
	})
}

// ClipRect records a clip region.
func (r *Renderer) ClipRect(x, y, w, h float64) {
	r.frame.Commands = append(r.frame.Commands, render.ClipRect{
		X: x, Y: y, W: w, H: h,
	})
}

// EndFrame completes the frame. The software backend has nothing to
// present; pixel rasterization arrives with the golden-test harness.
func (r *Renderer) EndFrame() {}

// Frame returns the commands recorded for the current frame.
func (r *Renderer) Frame() render.Frame {
	return r.frame
}
