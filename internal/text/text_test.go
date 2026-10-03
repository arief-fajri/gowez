package text

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestDefaultFont(t *testing.T) {
	f, err := Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if f.Family != "Go Regular" {
		t.Errorf("Family = %q, want %q", f.Family, "Go Regular")
	}
	if f.Size != DefaultSizePx {
		t.Errorf("Size = %v, want %v", f.Size, DefaultSizePx)
	}
	if f.face == nil {
		t.Fatal("face is nil")
	}
	// Concurrent-safe sharing: two calls must return usable fonts.
	g, err := Default()
	if err != nil || g.face == nil {
		t.Fatalf("second Default: %v", err)
	}
}

func TestShapeDeterministic(t *testing.T) {
	f, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	const s = "Counter: 42 — deterministic"
	a, err := Shape(s, f)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Shape(s, f)
	if err != nil {
		t.Fatal(err)
	}
	if a.Width != b.Width || a.Ascent != b.Ascent || a.Descent != b.Descent {
		t.Fatalf("metrics differ: (%v,%v,%v) vs (%v,%v,%v)",
			a.Width, a.Ascent, a.Descent, b.Width, b.Ascent, b.Descent)
	}
	if len(a.Glyphs) != len(b.Glyphs) {
		t.Fatalf("glyph count %d vs %d", len(a.Glyphs), len(b.Glyphs))
	}
	for i := range a.Glyphs {
		if a.Glyphs[i] != b.Glyphs[i] {
			t.Fatalf("glyph %d differs: %+v vs %+v", i, a.Glyphs[i], b.Glyphs[i])
		}
	}
}

func TestShapeMetricsSane(t *testing.T) {
	f, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	sh, err := Shape("Hello", f)
	if err != nil {
		t.Fatal(err)
	}
	if len(sh.Glyphs) != 5 {
		t.Fatalf("glyphs = %d, want 5", len(sh.Glyphs))
	}
	// Go Regular "Hello" at 16px advances ~37.9px (measured); allow
	// generous tolerance so font updates do not break the shape test.
	if sh.Width < 30 || sh.Width > 46 {
		t.Errorf("Width = %v, want ~37.9 (30..46)", sh.Width)
	}
	if sh.Ascent <= 0 || sh.Descent <= 0 {
		t.Errorf("Ascent/Descent = %v/%v, want both positive", sh.Ascent, sh.Descent)
	}
	// Double the size must double the advance (linear scaling).
	sh2, err := Shape("Hello", f.WithSize(32))
	if err != nil {
		t.Fatal(err)
	}
	ratio := sh2.Width / sh.Width
	if ratio < 1.95 || ratio > 2.05 {
		t.Errorf("width ratio at 2x size = %v, want ~2", ratio)
	}
	// Pen positions are monotonically increasing.
	var pen float64
	for i, g := range sh.Glyphs {
		if g.X != pen {
			t.Errorf("glyph %d X = %v, want pen %v", i, g.X, pen)
		}
		pen += g.Advance
	}
	if pen != sh.Width {
		t.Errorf("pen after run = %v, Width = %v", pen, sh.Width)
	}
}

func TestShapeEmpty(t *testing.T) {
	f, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	sh, err := Shape("", f)
	if err != nil {
		t.Fatalf("empty string must shape, got %v", err)
	}
	if len(sh.Glyphs) != 0 || sh.Width != 0 {
		t.Errorf("empty shape: %d glyphs, width %v", len(sh.Glyphs), sh.Width)
	}
}

func TestShapeErrors(t *testing.T) {
	if _, err := Shape("x", nil); err != ErrFontRequired {
		t.Errorf("nil font: err = %v, want ErrFontRequired", err)
	}
	f, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Shape("x", &Font{Size: -1, face: f.face}); err != ErrInvalidSize {
		t.Errorf("negative size: err = %v, want ErrInvalidSize", err)
	}
}

func TestWithSize(t *testing.T) {
	f, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	g := f.WithSize(24)
	if g.Size != 24 {
		t.Errorf("Size = %v, want 24", g.Size)
	}
	if g.face != f.face {
		t.Error("WithSize must share the parsed face")
	}
	if h := f.WithSize(0); h.Size != DefaultSizePx {
		t.Errorf("WithSize(0).Size = %v, want %v", h.Size, DefaultSizePx)
	}
	if f.Size != DefaultSizePx {
		t.Error("WithSize must not mutate the receiver")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/font.ttf")
	if err == nil {
		t.Fatal("want error for missing file")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("/nonexistent/font.ttf")) {
		t.Errorf("error must name the file: %v", err)
	}
}

func TestDrawDeterministic(t *testing.T) {
	f, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	sh, err := Shape("Render me", f.WithSize(20))
	if err != nil {
		t.Fatal(err)
	}
	drawOnce := func() *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 200, 60))
		sh.Draw(img, 10, 40, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		return img
	}
	a, b := drawOnce(), drawOnce()
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("two draws of the same shaped run differ")
	}
	ink := 0
	for _, v := range a.Pix {
		if v != 0 {
			ink++
		}
	}
	if ink == 0 {
		t.Fatal("Draw produced an empty image")
	}
}

func TestDrawEmptyRun(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	blank := bytes.Repeat([]byte{0}, len(img.Pix))
	var sh *Shaped
	sh.Draw(img, 0, 0, color.White) // nil run: no panic
	if !bytes.Equal(img.Pix, blank) {
		t.Error("nil run must draw nothing")
	}
}
