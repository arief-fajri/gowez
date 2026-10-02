package style

// Selector matches nodes during style resolution (Milestone 2).
type Selector struct {
	// Raw is the original selector text, kept for diagnostics.
	Raw string
	// TODO(M2): typed selector AST — type, class, id, descendant combinator.
}

// Matches reports whether the selector applies to a node identified by its
// tag, class attribute, and id.
//
// Milestone 2: only the supported subset matches; anything outside it must
// be rejected at parse time, not silently ignored.
func (s Selector) Matches(tag, class, id string) bool {
	_ = tag
	_ = class
	_ = id
	return false
}
