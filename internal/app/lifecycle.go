package app

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/render/backend/software"
	"github.com/arief-fajri/gowez/internal/text"
	"github.com/arief-fajri/gowez/internal/window"
)

// ErrNotImplemented is retained for public API stability (G-IFACE).
// Run has implemented the Milestone 1 startup sequence since Milestone 1;
// entry points that are still stubs return their own package-level
// errors (e.g. the OpenGL backend).
var ErrNotImplemented = errors.New("app: not implemented yet (Milestone 1)")

// frameBudget is the M1 target cadence. Software pacing is
// authoritative: on the SDL path Present does not block even with vsync
// reported on (DRR-001, finding F3), so the loop owns pacing and never
// depends on the driver.
const frameBudget = time.Second / 60

// maxFrameOverrun is the slack before a frame counts as dropped
// (Module 5 §5.1 frame-time metric).
const maxFrameOverrun = frameBudget + frameBudget/2

// NewWindow and NewReporter are indirection points for failure
// experiment A (tests/failure): startup faults are injected by
// replacing them; production always uses the defaults. Nothing else
// should write these variables.
var (
	NewWindow   = window.New
	NewReporter = func() observe.Reporter { return observe.StderrReporter{} }
)

// Run executes the full startup sequence (Module 2 §2.3) and blocks
// until shutdown:
//
//	config → window → renderer → scene → ready → render loop
//
// Every step must either succeed or abort with an explicit diagnostic;
// a partially initialized application never reaches ready state.
// Remaining steps of the full MVP sequence (JS runtime, UI bundle, UI
// tree, layout) arrive with their own milestones.
func Run(opts Options) error {
	start := time.Now()
	a, err := New(opts)
	if err != nil {
		return err
	}
	if err := a.transition(StateStarting); err != nil {
		return err
	}
	if err := a.execute(start); err != nil {
		_ = a.transition(StateFailed)
		return err
	}
	return a.transition(StateStopped)
}

// fail reports a diagnostic for the failing component and returns the
// error for the caller (P5: every failure is observable).
func (a *App) fail(step, message string, err error) error {
	a.reporter.Report(observe.Diagnostic{Component: step, Message: message, Err: err})
	return fmt.Errorf("app: %s: %w", step, err)
}

// execute drives startup through the ready state and runs the render
// loop until the window closes.
func (a *App) execute(start time.Time) error {
	// Text stack bring-up: the embedded face must parse before any
	// frame can be trusted.
	if _, err := text.Default(); err != nil {
		return a.fail("font", "embedded font failed to load", err)
	}

	// Window bring-up (OS main thread — SDL requirement).
	win, err := NewWindow(window.Options{
		Title:  a.opts.Title,
		Width:  a.opts.Width,
		Height: a.opts.Height,
	})
	if err != nil {
		return a.fail("window", "window creation failed", err)
	}
	defer win.Close()

	rend := software.New()
	scene, err := newHelloScene()
	if err != nil {
		return a.fail("scene", "scene initialization failed", err)
	}

	if err := a.transition(StateReady); err != nil {
		return err
	}
	ready := time.Now()
	a.metrics.RecordStartup(ready.Sub(start))
	win.Show()

	if err := a.loop(win, rend, scene, ready); err != nil {
		return err
	}
	return a.transition(StateStopping)
}

// loop paces frames at frameBudget, pumps events, rasterizes, and
// presents. It exits cleanly on CloseEvent or on SIGINT/SIGTERM
// (Observed 2026-10-03: the SDL/Cocoa stack leaves those signals
// ignored, so the runtime installs its own handler — otherwise the
// process can only be killed with SIGKILL). It aborts with a reported
// diagnostic on any present failure.
func (a *App) loop(win window.Window, rend *software.Renderer, scene *helloScene, from time.Time) error {
	// Buffered so a signal arriving mid-frame is never lost; signal.Stop
	// on return restores default handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	next := from.Add(frameBudget)
	for {
		now := time.Now()
		if d := next.Sub(now); d > 0 {
			time.Sleep(d) // bounded by frameBudget (G-REL-01)
		}
		now = time.Now()
		next = next.Add(frameBudget)
		if now.Sub(next) > 4*frameBudget {
			next = now // a long stall must not cause a burst of catch-up frames
		}
		frameStart := now

		select {
		case <-sigCh:
			return nil // graceful shutdown, same path as CloseEvent
		default:
		}

		running := true
		for _, ev := range win.Pump() {
			if _, ok := ev.(window.CloseEvent); ok {
				running = false
			}
		}
		if !running {
			return nil
		}

		logicalW, _ := win.Size()
		pw, ph := win.PixelSize()
		if pw <= 0 || ph <= 0 || logicalW <= 0 {
			continue // minimized or not yet mapped: keep looping, present nothing
		}

		rend.BeginFrame(pw, ph)
		if err := scene.Draw(rend, pw, ph, logicalW); err != nil {
			return a.fail("scene", "scene draw failed", err)
		}
		rend.EndFrame()
		px, err := rend.Pixels()
		if err != nil {
			return a.fail("renderer", "rasterization failed", err)
		}
		if err := win.Present(px, pw, ph); err != nil {
			return a.fail("window", "present failed", err)
		}
		scene.Tick()

		a.metrics.RecordFrame(time.Since(frameStart) > maxFrameOverrun)
	}
}
