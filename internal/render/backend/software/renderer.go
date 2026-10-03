package software

import (
	"image"

	"github.com/arief-fajri/gowez/internal/render"
)

// Renderer is the CPU reference backend. It records commands into a Frame
// so tests can assert on exact draw output (tests/golden), and
// rasterizes them into an RGBA pixel buffer on EndFrame so the runtime
// can present the frame through internal/window.
type Renderer struct {
	frame render.Frame

	img       *image.RGBA
	ended     bool
	begun     bool
	rasterErr error
}

// Compile-time proof that the backend satisfies the render contract.
var _ render.Renderer = (*Renderer)(nil)

// New creates a software renderer.
func New() *Renderer {
	return &Renderer{}
}

// BeginFrame resets the recorded frame and clears the pixel buffer.
func (r *Renderer) BeginFrame(width, height int) {
	r.frame = render.Frame{Width: width, Height: height}
	r.begun = true
	r.ended = false
	r.rasterErr = nil
	if width <= 0 || height <= 0 {
		r.img = image.NewRGBA(image.Rect(0, 0, 1, 1))
		return
	}
	if r.img == nil || r.img.Bounds().Dx() != width || r.img.Bounds().Dy() != height {
		r.img = image.NewRGBA(image.Rect(0, 0, width, height))
		return
	}
	clear(r.img.Pix)
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

// EndFrame completes the frame: the recorded commands are rasterized
// into the pixel buffer. Rasterization is bounded (linear in commands x
// covered pixels) and never blocks.
func (r *Renderer) EndFrame() {
	if !r.begun || r.ended {
		return
	}
	r.rasterize()
	r.ended = true
}

// Frame returns the commands recorded for the current frame.
func (r *Renderer) Frame() render.Frame {
	return r.frame
}

// Pixels returns the rasterized frame as tightly packed RGBA bytes
// (stride = width*4), valid until the next BeginFrame. The first
// rasterization error (e.g. font failure) is returned here — never
// silently dropped.
func (r *Renderer) Pixels() ([]byte, error) {
	if !r.begun {
		return nil, ErrNoFrame
	}
	if !r.ended {
		return nil, ErrFrameNotEnded
	}
	if r.rasterErr != nil {
		return nil, r.rasterErr
	}
	return r.img.Pix, nil
}

// Size returns the current frame dimensions in pixels.
func (r *Renderer) Size() (width, height int) {
	return r.frame.Width, r.frame.Height
}
