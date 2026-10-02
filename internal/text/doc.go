// Package text owns font loading, glyph shaping, and font fallback
// (Milestone 1).
//
// Text rendering failure must be observable and bounded — a missing font
// degrades explicitly, it never blocks the render loop.
package text
