// Package ui owns the runtime UI tree: nodes, structural mutation, event
// dispatch, hit testing, and the interaction state machine (hover,
// press, focus — Milestone 3).
//
// The renderer never consumes this tree directly — it consumes render
// commands derived from layout.
package ui
