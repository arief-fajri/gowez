package ui

// EventKind classifies events dispatched into the UI tree (Milestone 3).
type EventKind int

const (
	// Click is a completed pointer press+release on a node.
	Click EventKind = iota
	// PointerDown is a pointer press.
	PointerDown
	// PointerUp is a pointer release.
	PointerUp
	// PointerMove is pointer motion.
	PointerMove
	// KeyDown is a key press.
	KeyDown
	// KeyUp is a key release.
	KeyUp
)

// Event is one UI event delivered to a target node.
type Event struct {
	// Kind classifies the event.
	Kind EventKind
	// Target is the node that received the event (post hit-test).
	Target *Node
	// Key holds the key name for keyboard events.
	Key string
	// X, Y are viewport coordinates for pointer events.
	X, Y float64
	// Modifier is a bitfield of held modifiers.
	Modifier int
}

// Handler processes one event. Handlers run on the UI goroutine; they must
// not block (guard rail G-REL-01).
type Handler func(*Event)

// Dispatch delivers e to its target node. Event semantics never change
// silently (invariant I6); unhandled events are dropped, not guessed at.
//
// Milestone 3 wires the listener registry produced by the script layer.
func (t *Tree) Dispatch(e *Event) {
	if e == nil || e.Target == nil {
		return
	}
	// TODO(M3): walk the target's listener registry and invoke handlers.
}
