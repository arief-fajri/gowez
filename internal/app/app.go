package app

import (
	"fmt"

	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/render"
	"github.com/arief-fajri/gowez/internal/window"
)

// State is the application lifecycle state.
type State int

// Lifecycle states. Transitions only move forward; StateFailed is terminal
// (Module 2 §2.4: startup failures are explicit, never half-initialized).
const (
	StateCreated State = iota
	StateStarting
	StateReady
	StateStopping
	StateStopped
	StateFailed
)

// String returns the state name for diagnostics.
func (s State) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateStarting:
		return "starting"
	case StateReady:
		return "ready"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateFailed:
		return "failed"
	default:
		return fmt.Sprintf("state(%d)", int(s))
	}
}

// App coordinates one application instance.
type App struct {
	opts     Options
	state    State
	metrics  *observe.Recorder
	reporter observe.Reporter
}

// scene is the per-frame surface a mounted UI provides. Two implementations
// exist: the demo scene (M1–M4, hand-built in Go) and the bundle scene (M5,
// built by JavaScript through ui.apply). The frame loop only sees this
// interface, so the two paths cannot drift apart.
type scene interface {
	// Draw paints one frame at framebuffer size pw×ph for the given logical
	// size logicalW×logicalH.
	Draw(r render.Renderer, pw, ph, logicalW, logicalH int) error
	// Tick advances per-frame state.
	Tick()
	// HandleInput feeds one window input event into the UI tree.
	HandleInput(ev window.Event)
	// FocusEditable reports whether the focused node accepts text input, and
	// is how the loop decides to start or stop platform text input (M5).
	FocusEditable() bool
	// SetText writes committed or preedit text to the focused editable node.
	SetText(text string)
}

// New validates options and creates an application in StateCreated.
func New(opts Options) (*App, error) {
	opts = opts.withDefaults()
	if opts.Width <= 0 || opts.Height <= 0 {
		return nil, fmt.Errorf("app: invalid window size %dx%d", opts.Width, opts.Height)
	}
	return &App{
		opts:     opts,
		state:    StateCreated,
		metrics:  observe.NewRecorder(),
		reporter: NewReporter(),
	}, nil
}

// Metrics returns the runtime measurements collected so far (Module 5
// §5.1; P5 — behavior that is not observed is not proven).
func (a *App) Metrics() observe.Metrics {
	return a.metrics.Snapshot()
}

// State reports the current lifecycle state.
func (a *App) State() State {
	return a.state
}

// Options reports the effective (defaulted) options.
func (a *App) Options() Options {
	return a.opts
}

// transition advances the lifecycle one step. Backward transitions are
// rejected so observers always see a monotonic sequence (invariant I1).
func (a *App) transition(to State) error {
	if a.state == StateFailed {
		return fmt.Errorf("app: terminal state reached, cannot go from %s to %s", a.state, to)
	}
	if to <= a.state {
		return fmt.Errorf("app: invalid transition %s -> %s", a.state, to)
	}
	a.state = to
	return nil
}
