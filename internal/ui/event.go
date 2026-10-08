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
	// TextInput is committed Unicode text from the platform text-input
	// source (Milestone 5). It carries text, never a keycode: layout never
	// derives characters from keys.
	TextInput
	// TextEditing is IME composition (preedit) text plus the composition
	// window's cursor/selection extent. Preedit is transient — it is
	// rendered by the host and never committed as a value.
	TextEditing
)

// String returns the event kind name for diagnostics.
func (k EventKind) String() string {
	switch k {
	case Click:
		return "click"
	case PointerDown:
		return "pointer-down"
	case PointerUp:
		return "pointer-up"
	case PointerMove:
		return "pointer-move"
	case KeyDown:
		return "key-down"
	case KeyUp:
		return "key-up"
	case TextInput:
		return "textinput"
	case TextEditing:
		return "textediting"
	default:
		return "event"
	}
}

// Pointer buttons — portable numbering; the values match the
// window package's ButtonLeft/ButtonMiddle/ButtonRight constants
// (docs/EVENTS.md).
const (
	ButtonLeft   = 1
	ButtonMiddle = 2
	ButtonRight  = 3
)

// Modifier bits — portable subset; the values match the window
// package's ModShift/ModCtrl/ModAlt/ModMeta constants
// (docs/EVENTS.md).
const (
	ModShift = 1 << iota
	ModCtrl
	ModAlt
	ModMeta
)

// ListenerID identifies one registered handler (0 means "none"). It is
// the runtime-side handle the script layer (Milestone 4) will keep for
// the protocol's removeEventListener op.
type ListenerID int

// listener is one registry entry; entries run in registration order.
type listener struct {
	id   ListenerID
	kind EventKind
	fn   Handler
}

// Event is one UI event delivered to a target node.
type Event struct {
	// Kind classifies the event.
	Kind EventKind
	// Target is the node the event was aimed at (post hit-test); it
	// does not change while the event bubbles.
	Target *Node
	// CurrentTarget is the node whose listener is running right now —
	// it advances up the ancestor chain during bubbling.
	CurrentTarget *Node
	// Key holds the key name for keyboard events.
	Key string
	// X, Y are viewport coordinates for pointer events.
	X, Y float64
	// Button holds the pointer button for down/up/click events.
	Button int
	// Modifier is a ModShift|ModCtrl|ModAlt|ModMeta bitfield.
	Modifier int
	// Text carries the committed or preedit text for TextInput and
	// TextEditing events (Milestone 5). It is empty for every other kind:
	// characters are never derived from a keycode.
	Text string
	// TextStart and TextLength describe the IME composition window's extent
	// within Text. They are zero when no composition is active.
	TextStart  int
	TextLength int
	// stopped is set by StopPropagation; dispatch stops bubbling.
	stopped bool
}

// StopPropagation prevents the event from bubbling past the current
// target. Later listeners on the same node still run (docs/EVENTS.md).
func (e *Event) StopPropagation() { e.stopped = true }

// PropagationStopped reports whether StopPropagation was called.
func (e *Event) PropagationStopped() bool { return e.stopped }

// Handler processes one event. Handlers run on the UI goroutine; they
// must not block (guard rail G-REL-01). A panicking handler is
// recovered by Dispatch and reported through Tree.OnPanic — it never
// takes down the UI loop (docs/EVENTS.md §handler contract).
type Handler func(*Event)

// AddEventListener registers h for events of kind on n and returns its
// handle. Registering on a nil node or with a nil handler is a no-op
// that returns 0. Handlers on one node run in registration order;
// across nodes they run target-first, then up the ancestor chain.
func (t *Tree) AddEventListener(n *Node, kind EventKind, h Handler) ListenerID {
	if n == nil || h == nil {
		return 0
	}
	t.nextListenerID++
	if t.listeners == nil {
		t.listeners = make(map[NodeID][]listener, 1)
	}
	t.listeners[n.ID] = append(t.listeners[n.ID], listener{id: t.nextListenerID, kind: kind, fn: h})
	return t.nextListenerID
}

// SetListenerHandler replaces the handler behind an existing registration
// and reports whether the registration existed.
//
// It exists for the instruction applier: ApplyOps registers listeners
// without knowing what should run on them, because the caller owns the
// binding to the script layer (internal/script). The stream therefore fixes
// which (node, kind) pair is live, and the caller supplies the behavior
// afterwards. Replacing rather than removing-and-readding keeps the
// registration order stable (docs/EVENTS.md §dispatch).
func (t *Tree) SetListenerHandler(n *Node, id ListenerID, h Handler) bool {
	if n == nil || id == 0 || h == nil {
		return false
	}
	for i, l := range t.listeners[n.ID] {
		if l.id == id {
			t.listeners[n.ID][i].fn = h
			return true
		}
	}
	return false
}

// RemoveEventListener removes the registration id from n and reports
// whether it existed.
func (t *Tree) RemoveEventListener(n *Node, id ListenerID) bool {
	if n == nil || id == 0 {
		return false
	}
	ls := t.listeners[n.ID]
	for i, l := range ls {
		if l.id == id {
			t.listeners[n.ID] = append(ls[:i], ls[i+1:]...)
			if len(t.listeners[n.ID]) == 0 {
				delete(t.listeners, n.ID)
			}
			return true
		}
	}
	return false
}

// Dispatch delivers e to its target, then bubbles it up the ancestor
// chain until an ancestor is nil or StopPropagation was called. It
// returns how many handlers ran and how many panicked (recovered).
//
// Event semantics never change silently (invariant I6): a node without
// a matching listener is not an error — the event simply goes
// unhandled. The registry snapshot per node means a handler may add or
// remove listeners while an event is in flight.
func (t *Tree) Dispatch(e *Event) (dispatched, panics int) {
	if e == nil || e.Target == nil {
		return 0, 0
	}
	for cur := e.Target; cur != nil && !e.stopped; cur = cur.Parent {
		e.CurrentTarget = cur
		snapshot := append([]listener(nil), t.listeners[cur.ID]...)
		for _, l := range snapshot {
			if l.kind != e.Kind {
				continue
			}
			dispatched++
			if t.invoke(l.fn, e) {
				panics++
			}
		}
	}
	e.CurrentTarget = nil
	return dispatched, panics
}

// invoke runs one handler with panic recovery: a fault in one handler
// is observable (Tree.OnPanic) but bounded — the dispatch loop, the
// frame, and the process survive (P3/P4).
func (t *Tree) invoke(h Handler, e *Event) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			if t.OnPanic != nil {
				t.OnPanic(e, r)
			}
		}
	}()
	h(e)
	return false
}
