package window

import "errors"

// ErrUnsupported is returned when the current platform has no backend yet.
var ErrUnsupported = errors.New("window: no backend implemented for this platform yet")

// Options describe the native window to create.
type Options struct {
	// Title is the window title.
	Title string
	// Width and Height are the initial size in logical pixels.
	Width  int
	Height int
}

// Event is one window-level event (input, resize, close). Milestone 3
// delivers these into the UI tree for hit testing and dispatch.
type Event interface {
	isWindowEvent()
}

// CloseEvent is emitted when the user asks the window to close.
type CloseEvent struct{}

func (CloseEvent) isWindowEvent() {}

// ResizeEvent is emitted after the window is resized; layout must rerun
// (Module 6: resize triggers relayout). Width and Height are in logical
// pixels; the pixel size is read through Window.PixelSize.
type ResizeEvent struct {
	Width  int
	Height int
}

func (ResizeEvent) isWindowEvent() {}

// Pointer button identifiers (portable numbering; matches the SDL
// values the backend maps from). Only ButtonLeft drives click
// synthesis (docs/EVENTS.md).
const (
	ButtonLeft   = 1
	ButtonMiddle = 2
	ButtonRight  = 3
)

// Modifier bits for KeyEvent.Modifier — a portable subset instead of
// the platform's raw modifier mask (docs/EVENTS.md).
const (
	ModShift = 1 << iota
	ModCtrl
	ModAlt
	ModMeta
)

// PointerEvent carries mouse/pointer input in window coordinates
// (logical pixels — the same space layout and hit testing use).
//
// Press/Button are meaningful only on button events: motion events
// carry Press=false and Button=0, because the press edge is defined
// exclusively by a button down event (docs/EVENTS.md).
type PointerEvent struct {
	X, Y   float64
	Press  bool
	Button int
}

func (PointerEvent) isWindowEvent() {}

// KeyEvent carries keyboard input. Key is the portable key name
// (docs/EVENTS.md §key naming — unknown keys are reported as
// "#<keycode>", never dropped silently); Modifier is a ModShift|ModCtrl|
// ModAlt|ModMeta bitfield; Repeat reports an OS key-repeat press.
type KeyEvent struct {
	Key      string
	Press    bool
	Modifier int
	Repeat   bool
}

func (KeyEvent) isWindowEvent() {}

// TextInputEvent carries committed Unicode text produced by the platform
// text-input source (Milestone 5).
//
// It is deliberately text, not a keycode: characters never come from layout.
// IME composition arrives separately as TextEditingEvent, so a committed
// value is never contaminated by preedit. Events are only produced while
// text input is active for the window (StartTextInput).
type TextInputEvent struct {
	Text string
}

func (TextInputEvent) isWindowEvent() {}

// textInputSupport records whether a backend can deliver platform text input.
// A backend that cannot must report it explicitly rather than accepting a
// Start that produces no events, which would look like a keyboard fault in the
// application (P4).
type textInputSupport interface {
	textInputSupported() bool
}

// TextInputSupported reports whether the platform behind this window can
// deliver text input and IME composition. It answers "can this platform do
// text input at all", not "is text input currently active" — the latter is
// internal to the frame loop.
func TextInputSupported(w Window) bool {
	s, ok := w.(textInputSupport)
	return !ok || s.textInputSupported()
}

// TextEditingEvent carries IME composition (preedit) text for the focused
// editable node (Milestone 5).
//
// Start and Length describe the composition window's cursor/selection extent
// inside Text. The runtime renders the preedit and discards it; only
// TextInputEvent commits a value. This is the documented divergence from
// browsers, where the candidate window is part of the platform UI — the
// candidate window is out of scope for M5 (docs/EVENTS.md).
type TextEditingEvent struct {
	Text   string
	Start  int
	Length int
}

func (TextEditingEvent) isWindowEvent() {}

// Window is the contract every platform backend implements.
//
// All methods must be called from the goroutine that created the window
// (the OS main thread — SDL requirement); Pump and Present are
// non-blocking and bounded (G-REL-01: no operation may block forever).
type Window interface {
	// Title returns the current window title.
	Title() string
	// SetTitle updates the window title.
	SetTitle(title string)
	// Size returns the current window size in logical pixels.
	Size() (width, height int)
	// PixelSize returns the framebuffer size in physical pixels
	// (HiDPI-aware). Layout uses this to size the raster buffer.
	PixelSize() (width, height int)
	// Show makes the window visible.
	Show()
	// Close releases all native resources; it is safe to call more
	// than once (invariant I12, guard rail G-REL-04).
	Close()
	// Pump drains the pending OS events and returns them in order.
	// It never blocks and processes a bounded batch per call.
	Pump() []Event
	// Present uploads one frame. pixels is tightly packed RGBA8888
	// (stride = width*4), premultiplied alpha; because the OS present
	// path treats alpha as straight, every pixel must be opaque
	// (A=255) — scenes paint an opaque background (Milestone 1
	// invariant). A size mismatch against the current pixel size is
	// resolved by stretching (1:1 when the scene lays out in pixel
	// units).
	Present(pixels []byte, width, height int) error
	// StartTextInput begins delivering TextInputEvent and TextEditingEvent
	// for the window. The runtime calls it when an editable node gains
	// focus and StopTextInput when it loses it or the window closes
	// (Milestone 5, docs/EVENTS.md §text input).
	//
	// Both are non-blocking and idempotent: a backend that cannot deliver
	// text reports an explicit error rather than silently dropping input.
	StartTextInput() error
	// StopTextInput stops text input delivery. Safe to call when text input
	// is not active.
	StopTextInput() error
}

// New creates the platform window backend (Milestone 1).
//
// It must be called from the OS main thread; see package docs.
func New(opts Options) (Window, error) {
	return newPlatformWindow(opts)
}
