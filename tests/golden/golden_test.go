// Golden tests pin the software backend's pixel output.
//
// Regenerate after an intentional rendering change with:
//
//	go test ./tests/golden -update
//
// Goldens are exact RGBA comparisons (decoded), so PNG encoder
// differences across Go versions do not matter. The text golden relies
// on go-text/rasterx being deterministic pure-Go float math; if a future
// arch mismatch ever appears it must be investigated, not tolerated —
// determinism is acceptance criterion Module 6.
package golden

import (
	"bytes"
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/arief-fajri/gowez/internal/render"
	"github.com/arief-fajri/gowez/internal/render/backend/software"
)

var update = flag.Bool("update", false, "rewrite golden PNG files")

// run rasterizes the frame built by build and returns the pixels.
func run(t *testing.T, build func(r render.Renderer)) *image.RGBA {
	t.Helper()
	r := software.New()
	build(r)
	r.EndFrame()
	px, err := r.Pixels()
	if err != nil {
		t.Fatalf("Pixels: %v", err)
	}
	w, h := r.Size()
	if len(px) != w*h*4 {
		t.Fatalf("pixel buffer %d bytes, want %d (%dx%d)", len(px), w*h*4, w, h)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, px)
	return img
}

func checkGolden(t *testing.T, name string, got *image.RGBA) {
	t.Helper()
	path := filepath.Join("testdata", name+".png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, got); err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", path)
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run: go test ./tests/golden -update): %v", path, err)
	}
	// Byte-exact comparison of the encoded PNG: the golden was produced
	// by this same encoder, so both sides went through an identical
	// premultiplied-RGBA → PNG conversion. (Comparing decoded pixels
	// across color models — NRGBA vs RGBA — is lossy for alpha < 255
	// and produced false mismatches; see evidence/learnings.md.)
	if bytes.Equal(buf.Bytes(), raw) {
		return
	}
	// Decode both only to report a useful coordinate on failure.
	want, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode golden %s: %v", path, err)
	}
	if want.Bounds() != got.Bounds() {
		t.Fatalf("golden bounds %v, got %v", want.Bounds(), got.Bounds())
	}
	for y := got.Bounds().Min.Y; y < got.Bounds().Max.Y; y++ {
		for x := got.Bounds().Min.X; x < got.Bounds().Max.X; x++ {
			wr, wg, wb, wa := want.At(x, y).RGBA()
			gr, gg, gb, ga := got.At(x, y).RGBA()
			if wr != gr || wg != gg || wb != gb || wa != ga {
				t.Fatalf("pixel (%d,%d): golden rgba(%d,%d,%d,%d), got rgba(%d,%d,%d,%d)",
					x, y, wr>>8, wg>>8, wb>>8, wa>>8, gr>>8, gg>>8, gb>>8, ga>>8)
			}
		}
	}
	t.Fatalf("golden %s differs but no pixel mismatch found (PNG encoding drift?)", path)
}

func TestRects(t *testing.T) {
	got := run(t, func(r render.Renderer) {
		r.BeginFrame(128, 96)
		r.DrawRect(0, 0, 128, 96, render.Color{R: 0.12, G: 0.13, B: 0.17, A: 1})
		r.DrawRect(16, 16, 64, 64, render.Color{R: 0.94, G: 0.35, B: 0.16, A: 1})
		r.DrawRect(48, 48, 64, 40, render.Color{R: 0.2, G: 0.5, B: 0.9, A: 0.5})
		r.ClipRect(10, 10, 50, 50)
		r.DrawRect(0, 0, 128, 96, render.Color{R: 0.2, G: 0.8, B: 0.4, A: 1})
	})
	checkGolden(t, "rects", got)
}

func TestText(t *testing.T) {
	got := run(t, func(r render.Renderer) {
		r.BeginFrame(320, 80)
		r.DrawRect(0, 0, 320, 80, render.Color{R: 0.09, G: 0.1, B: 0.13, A: 1})
		r.DrawText(16, 36, "GoWEZ 42", render.TextOptions{
			FontSize: 24,
			Color:    render.Color{R: 1, G: 1, B: 1, A: 1},
		})
		r.ClipRect(0, 0, 150, 80)
		r.DrawText(100, 66, "clipped text", render.TextOptions{
			FontSize: 16,
			Color:    render.Color{R: 0.4, G: 0.9, B: 0.6, A: 1},
		})
	})
	checkGolden(t, "text", got)
}

// TestDeterminism renders twice and asserts byte-identical output —
// the core determinism criterion (Module 6) in miniature.
func TestDeterminism(t *testing.T) {
	build := func(r render.Renderer) {
		r.BeginFrame(160, 64)
		r.DrawRect(0, 0, 160, 64, render.Color{R: 0.1, G: 0.1, B: 0.1, A: 1})
		r.DrawText(8, 40, "same input, same pixels", render.TextOptions{
			FontSize: 14,
			Color:    render.Color{R: 1, G: 1, B: 1, A: 1},
		})
	}
	a := run(t, build).Pix
	b := run(t, build).Pix
	if !bytes.Equal(a, b) {
		t.Fatal("two renders of identical commands differ")
	}
}
