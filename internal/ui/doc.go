// Package ui owns the runtime UI tree: nodes, structural mutation, event
// dispatch, and hit testing.
//
// The renderer never consumes this tree directly — it consumes render
// commands derived from layout.
package ui
