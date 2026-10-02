package text

// Glyph is one positioned element of a shaped run.
type Glyph struct {
	// Rune is the source character.
	Rune rune
	// X, Y are the pen position in logical pixels.
	X, Y float64
	// Advance is the horizontal advance in logical pixels.
	Advance float64
}

// Shape converts text into positioned glyphs for a font (Milestone 1).
//
// Identical input must shape identically: layout depends on advances, and
// nondeterministic text geometry breaks the determinism criterion (Module 6).
func Shape(s string, f *Font) ([]Glyph, error) {
	_ = s
	_ = f
	return nil, ErrNotImplemented
}
