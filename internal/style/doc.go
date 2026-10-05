// Package style implements the CSS subset: parsing, selector matching,
// and style resolution.
//
// Only the documented subset (docs/CSS-SUBSET.md) is ever supported.
// Unsupported constructs fail explicitly with a position — never silently
// misrender (Module 2 §2.4, G-UPG-04).
//
// Dependency direction: style reads the ui tree (nodes and attributes)
// and produces ComputedStyle values; layout consumes both. Nothing above
// style depends on how styles are parsed.
package style
