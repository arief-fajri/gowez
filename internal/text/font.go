package text

import "errors"

// ErrNotImplemented is returned until font loading lands (Milestone 1).
var ErrNotImplemented = errors.New("text: font loading not implemented yet (Milestone 1)")

// Font is a loaded font face ready for shaping.
type Font struct {
	// Family is the resolved family name.
	Family string
	// Size is the nominal size in logical pixels.
	Size float64
}

// Load reads a font face from disk.
//
// Milestone 1 covers a minimal stack with explicit fallback (Open Question
// 6); fallback rules are part of the documented contract, not silent
// substitution.
func Load(path string) (*Font, error) {
	_ = path
	return nil, ErrNotImplemented
}
