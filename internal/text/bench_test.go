package text

import (
	"image"
	"image/color"
	"testing"
)

func newTestImage(w, h int) *image.RGBA { return image.NewRGBA(image.Rect(0, 0, w, h)) }

var white = color.NRGBA{R: 255, G: 255, B: 255, A: 255}

func BenchmarkShape(b *testing.B) {
	f, err := Default()
	if err != nil {
		b.Fatal(err)
	}
	const s = "The quick brown fox jumps over the lazy dog — 0123456789"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Shape(s, f); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShapeShort(b *testing.B) {
	f, err := Default()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Shape("frame 000123", f.WithSize(20)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDraw(b *testing.B) {
	f, err := Default()
	if err != nil {
		b.Fatal(err)
	}
	sh, err := Shape("The quick brown fox jumps over the lazy dog", f.WithSize(16))
	if err != nil {
		b.Fatal(err)
	}
	dst := newTestImage(512, 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sh.Draw(dst, 4, 40, white)
	}
}
