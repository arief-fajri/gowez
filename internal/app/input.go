package app

import (
	"github.com/arief-fajri/gowez/internal/ui"
	"github.com/arief-fajri/gowez/internal/window"
)

// HandleInput feeds one window input event into the UI tree: pointer
// and key events drive hit testing, dispatch, and the interaction
// state machine; a visual-state change marks the scene dirty so the
// next frame restyles before painting (input → hit test → node →
// handler → state → layout, docs/PLATFORM.md). Every fed event is
// recorded — interaction is observable (P5).
//
// Close and resize are handled by the caller: closing ends the loop,
// and resizing needs no event handling because the loop re-reads the
// window size every frame.
func (s *uiScene) HandleInput(ev window.Event) { s.handleInput(ev) }

// FocusEditable reports false for the demo scene: it has no text input
// widget, so the platform must not start text input for it.
func (s *uiScene) FocusEditable() bool { return false }

// SetText is a no-op for the demo scene, which has no editable node.
func (s *uiScene) SetText(string) {}

// handleInput feeds one window input event into the UI tree: pointer
// and key events drive hit testing, dispatch, and the interaction
// state machine; a visual-state change marks the scene dirty so the
// next frame restyles before painting (input → hit test → node →
// handler → state → layout, docs/PLATFORM.md). Every fed event is
// recorded — interaction is observable (P5).
//
// Close and resize are handled by the caller: closing ends the loop,
// and resizing needs no event handling because the loop re-reads the
// window size every frame.
func (s *uiScene) handleInput(ev window.Event) {
	var res ui.InputResult
	switch e := ev.(type) {
	case window.PointerEvent:
		switch {
		case e.Button != 0 && e.Press:
			res = s.tree.PointerDown(e.X, e.Y, e.Button)
		case e.Button != 0:
			res = s.tree.PointerUp(e.X, e.Y, e.Button)
		default:
			res = s.tree.PointerMove(e.X, e.Y)
		}
	case window.KeyEvent:
		if e.Press {
			res = s.tree.KeyDown(e.Key, e.Modifier)
		} else {
			res = s.tree.KeyUp(e.Key, e.Modifier)
		}
	default:
		return
	}
	if res.Changed {
		s.dirty = true
	}
	if s.metrics != nil {
		s.metrics.RecordInput(res.Dispatched, res.Panics)
	}
}
