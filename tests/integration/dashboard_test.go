//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/arief-fajri/gowez/internal/app"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/window"
)

// dashboardBundle is the committed M5 slice bundle. It is committed so this
// check runs offline without npm (DEVELOPMENT_GUIDE.md §1), and so the check
// exercises the artifact the adapter actually produced rather than a fixture.
var dashboardBundle = filepath.Join("..", "..", "examples", "gowez-dashboard", "dist")

// dashboardFrames is how many frames the run presents before closing. Three is
// enough to prove the present path is exercised repeatedly and keeps the check
// well under a second.
const dashboardFrames = 3

// dashboardDeadline bounds the run. The window closes itself after
// dashboardFrames presents, so hitting this means something upstream stopped
// progressing (G-REL-01: a hang is an error, not a stall).
const dashboardDeadline = 30 * time.Second

// autoCloseWindow decorates a real SDL window and closes it after a fixed number
// of presented frames, so app.Run terminates deterministically.
//
// Embedding window.Window forwards the whole contract (ten methods including
// StartTextInput/StopTextInput); only Present and Pump are overridden. That is
// deliberate: if the interface grows, this file stops compiling rather than
// silently losing a method.
type autoCloseWindow struct {
	window.Window

	presents     int
	presentsLeft int
	started      time.Time
	deadline     time.Time
	// forced records that the deadline fired rather than the frame budget,
	// which turns a near-hang into a visible failure instead of a stall.
	forced bool
}

// Present records one presented frame and spends one unit of the budget.
func (w *autoCloseWindow) Present(px []byte, width, height int) error {
	err := w.Window.Present(px, width, height)
	w.presents++
	if w.presentsLeft > 0 {
		w.presentsLeft--
	}
	return err
}

// Pump forwards platform events and injects the close that ends the run.
//
// The loop pumps *before* presenting, so the close lands on the pump after the
// last budgeted present — the run therefore presents exactly dashboardFrames
// frames before shutting down.
func (w *autoCloseWindow) Pump() []window.Event {
	evs := w.Window.Pump()
	if w.presentsLeft > 0 {
		return evs
	}
	if time.Now().After(w.deadline) {
		// The run is still alive but has blown its budget: close anyway so the
		// test fails on the assertion below instead of hanging.
		w.forced = true
	}
	return append(evs, window.CloseEvent{})
}

// captureReporter records diagnostics for assertions. The bundle path must
// produce no "ui" rejection (a rejected op batch) and no "ipc" failure.
type captureReporter struct {
	mu sync.Mutex
	d  []observe.Diagnostic
}

func (c *captureReporter) Report(d observe.Diagnostic) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.d = append(c.d, d)
}

func (c *captureReporter) snapshot() []observe.Diagnostic {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]observe.Diagnostic(nil), c.d...)
}

// countByComponent returns how many captured diagnostics carry the component.
func (c *captureReporter) countByComponent(component string) int {
	n := 0
	for _, d := range c.snapshot() {
		if d.Component == component {
			n++
		}
	}
	return n
}

// firstFor returns the first captured diagnostic for a component.
func (c *captureReporter) firstFor(component string) (observe.Diagnostic, bool) {
	for _, d := range c.snapshot() {
		if d.Component == component {
			return d, true
		}
	}
	return observe.Diagnostic{}, false
}

// runDashboardWindow opens a real OS window, mounts the compiled Svelte bundle
// through the full startup sequence, presents frames, and shuts down.
//
// This is the Milestone 5 platform proof: real SDL window (DRR-001) + real
// bundle (DRR-006) + real sandbox (docs/SCRIPT.md) + real style/layout/paint,
// end to end. The golden test proves the pixels; this proves the pixels reach
// an OS window.
//
// It must run on the OS main thread — SDL/Cocoa refuses to initialize
// elsewhere — so it is called from TestMain, which pins itself with
// runtime.LockOSThread.
func runDashboardWindow() error {
	if _, err := os.Stat(filepath.Join(dashboardBundle, "manifest.json")); err != nil {
		return fmt.Errorf("compiled bundle missing at %s (%v); run "+
			"`npm run build -w @gowez/example-gowez-dashboard`", dashboardBundle, err)
	}

	savedWindow, savedReporter := app.NewWindow, app.NewReporter
	defer func() {
		app.NewWindow = savedWindow
		app.NewReporter = savedReporter
	}()

	var win *autoCloseWindow
	capture := &captureReporter{}

	app.NewWindow = func(o window.Options) (window.Window, error) {
		real, err := window.New(o)
		if err != nil {
			return nil, err
		}
		win = &autoCloseWindow{
			Window:       real,
			presentsLeft: dashboardFrames,
			started:      time.Now(),
			deadline:     time.Now().Add(dashboardDeadline),
		}
		return win, nil
	}
	app.NewReporter = func() observe.Reporter { return capture }

	start := time.Now()
	err := app.Run(app.Options{
		Title:  "gowez-dashboard",
		Width:  900,
		Height: 640,
		UI:     os.DirFS(dashboardBundle),
	})
	elapsed := time.Since(start)

	if err != nil {
		// A mount failure lands here: the bundle step is a hard startup
		// failure, so a nil error already proves manifest + CSS + eval +
		// ui.apply all succeeded.
		return fmt.Errorf("app.Run: %w", err)
	}
	if win == nil {
		return fmt.Errorf("window was never created: the run cannot have presented anything")
	}
	if win.forced {
		return fmt.Errorf("run exceeded its %s budget and was closed early", dashboardDeadline)
	}
	if win.presents < dashboardFrames {
		return fmt.Errorf("presented %d frames, want %d", win.presents, dashboardFrames)
	}

	// A rejected op batch would mean the runtime refused instructions the
	// adapter emitted — the failure mode G-DATA-02 exists to make visible.
	if d, bad := capture.firstFor("ui"); bad {
		return fmt.Errorf("UI diagnostic during the run: %s", d.String())
	}
	if d, bad := capture.firstFor("ipc"); bad {
		return fmt.Errorf("IPC diagnostic during the run: %s", d.String())
	}

	fmt.Fprintf(os.Stderr,
		"dashboard window: OK (%d frames presented in %s, no rejected batches)\n",
		win.presents, elapsed.Round(time.Millisecond))
	return nil
}
