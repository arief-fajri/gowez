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
// (Module 6: resize triggers relayout).
type ResizeEvent struct {
	Width  int
	Height int
}

func (ResizeEvent) isWindowEvent() {}

// PointerEvent carries mouse/pointer input in window coordinates.
type PointerEvent struct {
	X, Y   float64
	Press  bool
	Button int
}

func (PointerEvent) isWindowEvent() {}

// KeyEvent carries keyboard input.
type KeyEvent struct {
	Key      string
	Press    bool
	Modifier int
}

func (KeyEvent) isWindowEvent() {}

// Window is the contract every platform backend implements.
type Window interface {
	// Title returns the current window title.
	Title() string
	// SetTitle updates the window title.
	SetTitle(title string)
	// Size returns the current window size in logical pixels.
	Size() (width, height int)
	// Show makes the window visible.
	Show()
	// Close requests window shutdown; native resources are released
	// predictably (invariant I12, guard rail G-REL-04).
	Close()
	// Events returns the window event stream.
	Events() <-chan Event
}

// New creates the platform window backend (Milestone 1).
func New(opts Options) (Window, error) {
	return newPlatformWindow(opts)
}
