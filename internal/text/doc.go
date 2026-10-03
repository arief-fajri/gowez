// Package text owns font loading, glyph shaping, and font fallback
// (Milestone 1).
//
// Text rendering failure must be observable and bounded — a missing font
// degrades explicitly, it never blocks the render loop.
//
// Coverage boundary (Hard rule 6, G-UPG-04): Milestone 1 shapes Latin
// and other simple scripts with a single face. Runes the face does not
// cover render as the font's notdef glyph (visible tofu) — there is no
// silent substitution between fonts and no script is dropped without a
// visible artifact. Complex-script shaping guarantees are validated
// before being claimed (Milestone 4+).
package text
