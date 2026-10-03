package software

import (
	"image"
	"image/color"
	"testing"

	"github.com/arief-fajri/gowez/internal/render"
)

// helpers shared by benchmarks.
func newTestImage(w, h int) *image.RGBA { return image.NewRGBA(image.Rect(0, 0, w, h)) }

var white = color.NRGBA{R: 255, G: 255, B: 255, A: 255}

// BenchmarkFrame measures a scene-shaped frame: background fill, three
// text runs, and a moving block at 640×420 — the gowez-hello workload.
func BenchmarkFrame(b *testing.B) {
	const w, h = 640, 420
	r := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.BeginFrame(w, h)
		r.DrawRect(0, 0, w, h, render.Color{R: 0.09, G: 0.10, B: 0.13, A: 1})
		r.DrawText(210, 142, "GoWEZ", render.TextOptions{FontSize: 48, Color: render.Color{R: 1, G: 1, B: 1, A: 1}})
		r.DrawText(230, 218, "frame 000123 · 00:00:02", render.TextOptions{FontSize: 20, Color: render.Color{R: 0.75, G: 0.78, B: 0.85, A: 1}})
		r.DrawText(250, 268, "close the window to quit", render.TextOptions{FontSize: 15, Color: render.Color{R: 0.45, G: 0.48, B: 0.56, A: 1}})
		r.DrawRect(100, 319, 64, 64, render.Color{R: 0.94, G: 0.35, B: 0.16, A: 1})
		r.EndFrame()
		if _, err := r.Pixels(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRecordRect isolates the command-recording path (no
// rasterization happens until EndFrame).
func BenchmarkRecordRect(b *testing.B) {
	r := New()
	r.BeginFrame(640, 420)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.DrawRect(0, 0, 640, 420, render.Color{R: 1, G: 1, B: 1, A: 1})
	}
}
