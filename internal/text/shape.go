package text

import (
	"errors"
	"math"
	"sync"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// Errors returned by Shape.
var (
	// ErrFontRequired is returned when shaping is requested without a
	// loaded font face.
	ErrFontRequired = errors.New("text: no font face loaded")
	// ErrInvalidSize is returned when the font size is not positive.
	ErrInvalidSize = errors.New("text: font size must be positive")
)

// Glyph is one positioned element of a shaped run.
//
// X is the pen position at the start of the glyph, Y is the pen offset
// from the baseline; for horizontal LTR text Y is 0.
type Glyph struct {
	// Rune is the source character.
	Rune rune
	// X, Y are the pen position in logical pixels.
	X, Y float64
	// Advance is the horizontal advance in logical pixels.
	Advance float64
}

// Shaped is the deterministic result of shaping one string with one font.
type Shaped struct {
	// Glyphs are the positioned glyphs, in visual order.
	Glyphs []Glyph
	// Width is the advance of the whole run in logical pixels.
	Width float64
	// Ascent is the height above the baseline in logical pixels.
	Ascent float64
	// Descent is the depth below the baseline in logical pixels
	// (positive).
	Descent float64

	run    shaping.Output
	sizePx float64
}

// The shaper is stateful (internal font cache) and not safe for
// concurrent use; the render loop is single-goroutine but tests are not.
var (
	shaperMu sync.Mutex
	shaper   shaping.HarfbuzzShaper
)

// Shape converts text into positioned glyphs for a font.
//
// Identical input must shape identically: layout depends on advances, and
// nondeterministic text geometry breaks the determinism criterion
// (Module 6). Runes the face does not cover resolve to the font's notdef
// glyph — visibly broken, never silently substituted (Hard rule 6,
// documented in doc.go).
func Shape(s string, f *Font) (*Shaped, error) {
	if f == nil || f.face == nil {
		return nil, ErrFontRequired
	}
	if f.Size <= 0 {
		return nil, ErrInvalidSize
	}
	rs := []rune(s)
	in := shaping.Input{
		Text:      rs,
		RunStart:  0,
		RunEnd:    len(rs),
		Direction: di.DirectionLTR,
		Face:      f.face,
		Size:      fixed.Int26_6(math.Round(f.Size * 64)),
		Script:    scriptFor(rs),
		Language:  language.NewLanguage("EN"),
	}
	shaperMu.Lock()
	out := shaper.Shape(in)
	shaperMu.Unlock()

	sh := &Shaped{
		Glyphs:  make([]Glyph, 0, len(out.Glyphs)),
		Width:   px(out.Advance),
		Ascent:  px(out.LineBounds.Ascent),
		Descent: -px(out.LineBounds.Descent),
		run:     out,
		sizePx:  f.Size,
	}
	var pen float64
	for _, g := range out.Glyphs {
		r := rune(0)
		if i := g.ClusterIndex; i >= 0 && i < len(rs) {
			r = rs[i]
		}
		sh.Glyphs = append(sh.Glyphs, Glyph{
			Rune:    r,
			X:       pen,
			Y:       0,
			Advance: px(g.Advance),
		})
		pen += px(g.Advance)
	}
	return sh, nil
}

// scriptFor picks the shaping script for a run: the first rune with a
// definite script, falling back to Latin for scriptless input (digits,
// punctuation, whitespace).
func scriptFor(rs []rune) language.Script {
	for _, r := range rs {
		sc := language.LookupScript(r)
		if sc != language.Common && sc != language.Unknown {
			return sc
		}
	}
	return language.Latin
}

func px(v fixed.Int26_6) float64 { return float64(v) / 64 }
