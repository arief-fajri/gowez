package failure

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arief-fajri/gowez/internal/app"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/window"
)

// captureReporter collects diagnostics for assertions (experiment A:
// "diagnostic available").
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

// TestAStartupFailureIsExplicit is failure experiment A
// (tests/failure/README.md): a bring-up step fails at startup and the
// application must abort with an explicit diagnostic, never reach a
// half-initialized state, and never hang (G-REL-01).
//
// Induction: the window bring-up seam returns an injected error. M1's
// software renderer constructor cannot fail, so the experiment injects
// at the first failable startup step after config/font — the same
// sequence Module 2 §2.3 describes. The run is executed with a hard
// deadline: a hang fails the test instead of blocking CI.
func TestAStartupFailureIsExplicit(t *testing.T) {
	savedWindow, savedReporter := app.NewWindow, app.NewReporter
	defer func() {
		app.NewWindow = savedWindow
		app.NewReporter = savedReporter
	}()

	injected := errors.New("injected: graphics device unavailable")
	app.NewWindow = func(window.Options) (window.Window, error) {
		return nil, injected
	}
	capture := &captureReporter{}
	app.NewReporter = func() observe.Reporter { return capture }

	done := make(chan error, 1)
	go func() {
		done <- app.Run(app.Options{Title: "experiment-a", Width: 640, Height: 480})
	}()

	var err error
	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return within 10s: unbounded startup failure (G-REL-01)")
	}

	if err == nil {
		t.Fatal("Run must fail when a startup step fails")
	}
	if !errors.Is(err, injected) {
		t.Errorf("error must wrap the injected cause, got: %v", err)
	}
	if !strings.Contains(err.Error(), "window") {
		t.Errorf("error must name the failing component, got: %v", err)
	}

	diags := capture.snapshot()
	if len(diags) == 0 {
		t.Fatal("no diagnostic reported (P5: failure must be observable)")
	}
	found := false
	for _, d := range diags {
		if d.Component == "window" && errors.Is(d.Err, injected) {
			found = true
		}
	}
	if !found {
		t.Errorf("diagnostics = %+v, want a window diagnostic wrapping the cause", diags)
	}
}
