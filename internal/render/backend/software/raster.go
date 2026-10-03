package software

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/arief-fajri/gowez/internal/render"
	"github.com/arief-fajri/gowez/internal/text"
)

// Errors surfaced by the rasterizer.
var (
	// ErrFrameNotEnded is returned by Pixels before EndFrame.
	ErrFrameNotEnded = errors.New("software: frame not ended")
	// ErrNoFrame is returned by Pixels before the first BeginFrame.
	ErrNoFrame = errors.New("software: no frame begun")
)

// rasterize replays the recorded command list into the pixel buffer.
//
// Semantics (documented contract of this backend):
//   - ClipRect intersects the current clip region with the given rect;
//     the clip resets at BeginFrame. There is no pop within a frame
//     because the command set has no matching pop.
//   - DrawRect replaces the destination pixels (no blending).
//   - DrawText alpha-blends the shaped run; Y is the text baseline.
//   - Transform carries no fields and is a no-op by design.
//
// The first rasterization error is kept and surfaced by Pixels; a frame
// never silently drops a command (Hard rule 6).
func (r *Renderer) rasterize() {
	clip := r.img.Bounds()
	for _, c := range r.frame.Commands {
		switch c := c.(type) {
		case render.DrawRect:
			r.fillRect(clip, c)
		case render.ClipRect:
			clip = clip.Intersect(pxRect(c.X, c.Y, c.W, c.H))
		case render.DrawText:
			if err := r.drawText(clip, c); err != nil {
				if r.rasterErr == nil {
					r.rasterErr = err
				}
				return
			}
		case render.Transform:
		}
	}
}

func (r *Renderer) fillRect(clip image.Rectangle, c render.DrawRect) {
	b := pxRect(c.X, c.Y, c.W, c.H).Intersect(clip)
	if b.Empty() {
		return
	}
	cr, cg, cb, ca := rgbaBytes(c.Color)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := y * r.img.Stride
		for x := b.Min.X; x < b.Max.X; x++ {
			o := row + x*4
			r.img.Pix[o+0] = cr
			r.img.Pix[o+1] = cg
			r.img.Pix[o+2] = cb
			r.img.Pix[o+3] = ca
		}
	}
}

func (r *Renderer) drawText(clip image.Rectangle, c render.DrawText) error {
	if c.Text == "" {
		return nil
	}
	f, err := text.Default()
	if err != nil {
		return err
	}
	sh, err := text.Shape(c.Text, f.WithSize(c.Options.FontSize))
	if err != nil {
		return err
	}
	sh.Draw(r.dstFor(clip), int(c.X), int(c.Y), goColor(c.Options.Color))
	return nil
}

// dstFor returns the draw target: the raw buffer when no clip is
// active, a Set-guarded wrapper otherwise. The wrapper deliberately does
// not expose *image.RGBA so image/draw fast paths cannot bypass the clip.
func (r *Renderer) dstFor(clip image.Rectangle) draw.Image {
	if clip == r.img.Bounds() {
		return r.img
	}
	return &clipImage{img: r.img, clip: clip}
}

// clipImage guards Set outside the clip region; At reads through so
// alpha blending inside the clip composites against the real pixels.
type clipImage struct {
	img  *image.RGBA
	clip image.Rectangle
}

func (c *clipImage) Bounds() image.Rectangle { return c.img.Bounds() }

func (c *clipImage) ColorModel() color.Model { return c.img.ColorModel() }

func (c *clipImage) At(x, y int) color.Color { return c.img.At(x, y) }

func (c *clipImage) Set(x, y int, col color.Color) {
	if !image.Pt(x, y).In(c.clip) {
		return
	}
	c.img.Set(x, y, col)
}

// pxRect converts a float rect to the pixel rectangle it covers:
// floor at the start edge, ceil at the end edge.
func pxRect(x, y, w, h float64) image.Rectangle {
	return image.Rect(
		int(math.Floor(x)), int(math.Floor(y)),
		int(math.Ceil(x+w)), int(math.Ceil(y+h)),
	)
}

// rgbaBytes converts a render color to 8-bit bytes for the pixel
// buffer. image.RGBA stores premultiplied alpha, so the fill path
// premultiplies; clamping is to [0, 1] with round-to-nearest.
func rgbaBytes(c render.Color) (r, g, b, a byte) {
	a = clamp255(c.A)
	rs, gs, bs := clamp255(c.R), clamp255(c.G), clamp255(c.B)
	if a == 255 {
		return rs, gs, bs, 255
	}
	return premul(rs, a), premul(gs, a), premul(bs, a), a
}

// premul multiplies a straight-alpha byte by alpha with round-to-nearest
// (deterministic integer math, no float drift).
func premul(v, a byte) byte {
	return byte((int(v)*int(a) + 127) / 255)
}

func clamp255(v float32) byte {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return byte(v*255 + 0.5)
}

// goColor adapts a render color for the text rasterizer.
func goColor(c render.Color) color.Color {
	return color.NRGBA{
		R: clamp255(c.R),
		G: clamp255(c.G),
		B: clamp255(c.B),
		A: clamp255(c.A),
	}
}
