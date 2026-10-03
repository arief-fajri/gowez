package software

import (
	"bytes"
	"testing"

	"github.com/arief-fajri/gowez/internal/render"
)

func pxAt(t *testing.T, pix []byte, stride, x, y int) [4]byte {
	t.Helper()
	o := y*stride + x*4
	return [4]byte{pix[o], pix[o+1], pix[o+2], pix[o+3]}
}

func TestPixelsLifecycle(t *testing.T) {
	r := New()
	if _, err := r.Pixels(); err != ErrNoFrame {
		t.Errorf("before BeginFrame: err = %v, want ErrNoFrame", err)
	}
	r.BeginFrame(4, 4)
	if _, err := r.Pixels(); err != ErrFrameNotEnded {
		t.Errorf("before EndFrame: err = %v, want ErrFrameNotEnded", err)
	}
	r.EndFrame()
	px, err := r.Pixels()
	if err != nil {
		t.Fatal(err)
	}
	if len(px) != 4*4*4 {
		t.Errorf("len = %d, want %d", len(px), 4*4*4)
	}
}

func TestFillRectOpaque(t *testing.T) {
	r := New()
	r.BeginFrame(4, 4)
	r.DrawRect(0, 0, 4, 4, render.Color{R: 1, G: 0, B: 0, A: 1})
	r.EndFrame()
	px, err := r.Pixels()
	if err != nil {
		t.Fatal(err)
	}
	want := [4]byte{255, 0, 0, 255}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if got := pxAt(t, px, 16, x, y); got != want {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want)
			}
		}
	}
}

// TestFillRectPremultiplied pins the buffer's alpha convention:
// image.RGBA stores premultiplied bytes, so a half-transparent white
// fill must be {128,128,128,128}, not {255,255,255,128}.
func TestFillRectPremultiplied(t *testing.T) {
	r := New()
	r.BeginFrame(2, 1)
	r.DrawRect(0, 0, 2, 1, render.Color{R: 1, G: 1, B: 1, A: 0.5})
	r.EndFrame()
	px, err := r.Pixels()
	if err != nil {
		t.Fatal(err)
	}
	if got := pxAt(t, px, 8, 0, 0); got != [4]byte{128, 128, 128, 128} {
		t.Errorf("pixel = %v, want [128 128 128 128]", got)
	}
}

func TestClipIntersect(t *testing.T) {
	r := New()
	r.BeginFrame(8, 8)
	r.DrawRect(0, 0, 8, 8, render.Color{R: 1, G: 1, B: 1, A: 1}) // white bg
	r.ClipRect(2, 2, 4, 4)
	r.DrawRect(0, 0, 8, 8, render.Color{R: 0, G: 0, B: 1, A: 1}) // blue, clipped
	r.EndFrame()
	px, err := r.Pixels()
	if err != nil {
		t.Fatal(err)
	}
	inside := pxAt(t, px, 32, 3, 3)
	if inside != [4]byte{0, 0, 255, 255} {
		t.Errorf("inside clip = %v, want blue", inside)
	}
	outside := pxAt(t, px, 32, 1, 1)
	if outside != [4]byte{255, 255, 255, 255} {
		t.Errorf("outside clip = %v, want white", outside)
	}
	edge := pxAt(t, px, 32, 2, 2) // clip rect starts at (2,2)
	if edge != [4]byte{0, 0, 255, 255} {
		t.Errorf("clip origin = %v, want blue", edge)
	}
}

func TestFractionalRectCoverage(t *testing.T) {
	r := New()
	r.BeginFrame(4, 4)
	// Covers pixels with x in [floor(0.5), ceil(2.5)) = [0, 3).
	r.DrawRect(0.5, 0, 2, 1, render.Color{R: 1, G: 0, B: 0, A: 1})
	r.EndFrame()
	px, err := r.Pixels()
	if err != nil {
		t.Fatal(err)
	}
	red := [4]byte{255, 0, 0, 255}
	clear := [4]byte{0, 0, 0, 0}
	if pxAt(t, px, 16, 0, 0) != red {
		t.Error("pixel x=0 must be covered (floor edge)")
	}
	if pxAt(t, px, 16, 2, 0) != red {
		t.Error("pixel x=2 must be covered (ceil edge)")
	}
	if pxAt(t, px, 16, 3, 0) != clear {
		t.Error("pixel x=3 must stay uncovered")
	}
}

func TestCommandRecording(t *testing.T) {
	r := New()
	r.BeginFrame(10, 10)
	r.DrawRect(1, 2, 3, 4, render.Color{R: 0.5, A: 1})
	r.DrawText(5, 6, "hi", render.TextOptions{FontSize: 12})
	r.ClipRect(0, 0, 5, 5)
	r.EndFrame()

	f := r.Frame()
	if f.Width != 10 || f.Height != 10 {
		t.Errorf("frame size %dx%d, want 10x10", f.Width, f.Height)
	}
	if len(f.Commands) != 3 {
		t.Fatalf("commands = %d, want 3", len(f.Commands))
	}
	dr, ok := f.Commands[0].(render.DrawRect)
	if !ok || dr.X != 1 || dr.Y != 2 || dr.W != 3 || dr.H != 4 {
		t.Errorf("DrawRect recorded as %+v", f.Commands[0])
	}
	dt, ok := f.Commands[1].(render.DrawText)
	if !ok || dt.Text != "hi" || dt.Options.FontSize != 12 {
		t.Errorf("DrawText recorded as %+v", f.Commands[1])
	}
	if _, ok := f.Commands[2].(render.ClipRect); !ok {
		t.Errorf("ClipRect recorded as %+v", f.Commands[2])
	}
}

func TestTextDrawsPixels(t *testing.T) {
	r := New()
	r.BeginFrame(120, 40)
	r.DrawRect(0, 0, 120, 40, render.Color{R: 0, G: 0, B: 0, A: 1})
	r.DrawText(8, 30, "GoWEZ", render.TextOptions{
		FontSize: 18,
		Color:    render.Color{R: 1, G: 1, B: 1, A: 1},
	})
	r.EndFrame()
	px, err := r.Pixels()
	if err != nil {
		t.Fatal(err)
	}
	inked := 0
	for i := 0; i < len(px); i += 4 {
		if px[i] > 0 {
			inked++
		}
	}
	if inked == 0 {
		t.Fatal("text produced no lit pixels")
	}
}

// TestPixelsStableAcrossFrames ensures buffer reuse does not leak
// content from the previous frame.
func TestPixelsStableAcrossFrames(t *testing.T) {
	r := New()
	r.BeginFrame(4, 4)
	r.DrawRect(0, 0, 4, 4, render.Color{R: 1, G: 1, B: 1, A: 1})
	r.EndFrame()
	// Pixels returns the live buffer (valid until the next BeginFrame),
	// so the painted frame must be copied before the reuse check.
	first, err := r.Pixels()
	if err != nil {
		t.Fatal(err)
	}
	first = append([]byte(nil), first...)

	r.BeginFrame(4, 4) // same size: buffer is reused and cleared
	px, err := r.Pixels()
	_ = px
	if err != ErrFrameNotEnded {
		t.Fatalf("second frame before EndFrame: %v", err)
	}
	r.EndFrame()
	second, _ := r.Pixels()
	if bytes.Equal(first, second) {
		t.Fatal("cleared frame must differ from the painted frame")
	}
	empty := make([]byte, 4*4*4)
	if !bytes.Equal(second, empty) {
		t.Fatal("cleared frame must be all zero before drawing")
	}
}
