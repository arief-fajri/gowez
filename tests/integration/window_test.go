//go:build integration

package integration

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/arief-fajri/gowez/internal/window"
)

// lifecycleErr is set by TestMain before m.Run.
var lifecycleErr error

// TestMain runs the SDL lifecycle check on the OS main thread.
//
// SDL/Cocoa refuses to initialize from any other goroutine: go test
// executes each Test function on a worker goroutine, so the check must
// live here, before m.Run (verified by experiment — see
// evidence/learnings.md). runtime.LockOSThread pins this goroutine to
// the main thread even across the park inside m.Run.
func TestMain(m *testing.M) {
	runtime.LockOSThread()
	lifecycleErr = runWindowLifecycle()
	if lifecycleErr == nil {
		fmt.Fprintln(os.Stderr, "window lifecycle: OK (open, present×2, pump, close)")
	}
	code := m.Run()
	if lifecycleErr != nil {
		fmt.Fprintf(os.Stderr, "window lifecycle: %v\n", lifecycleErr)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// TestWindowLifecycle is a declarative alias for the check TestMain
// already executed: if the lifecycle failed, TestMain exits non-zero
// regardless, and this assertion surfaces the same error under the
// normal test reporter instead of bare stderr.
func TestWindowLifecycle(t *testing.T) {
	if lifecycleErr != nil {
		t.Fatalf("window lifecycle failed in TestMain: %v", lifecycleErr)
	}
}

// runWindowLifecycle opens a real OS window, presents frames, pumps
// events, and shuts down — the Milestone 1 proof that the purego SDL3
// backend works on this platform (DRR-001). Every step is bounded: a
// hang is an error, not a stall (G-REL-01).
func runWindowLifecycle() error {
	deadline := time.Now().Add(20 * time.Second)
	step := func(name string, err error) error {
		if time.Now().After(deadline) {
			return fmt.Errorf("%s: past deadline — hung (G-REL-01)", name)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}

	w, err := window.New(window.Options{Title: "gowez-integration", Width: 320, Height: 240})
	if err != nil {
		return step("window.New", err)
	}
	defer w.Close()

	pw, ph := w.PixelSize()
	if pw <= 0 || ph <= 0 {
		return fmt.Errorf("PixelSize = %dx%d, want positive", pw, ph)
	}
	if lw, lh := w.Size(); lw <= 0 || lh <= 0 {
		return fmt.Errorf("Size = %dx%d, want positive", lw, lh)
	}

	// Fully opaque buffer (M1 present contract: A=255 everywhere).
	buf := make([]byte, pw*ph*4)
	for i := 0; i < len(buf); i += 4 {
		buf[i+0] = 40
		buf[i+1] = 44
		buf[i+2] = 56
		buf[i+3] = 255
	}
	if err := step("Present #1", w.Present(buf, pw, ph)); err != nil {
		return err
	}
	_ = w.Pump() // must never block

	// Present again to prove the streaming-texture path is reusable.
	if err := step("Present #2", w.Present(buf, pw, ph)); err != nil {
		return err
	}

	// Undersized buffer must fail explicitly, not corrupt the texture.
	if err := w.Present(buf[:8], pw, ph); err == nil {
		return fmt.Errorf("undersized buffer must be rejected")
	}

	w.Close()
	w.Close() // idempotent (invariant I12)

	if err := w.Present(buf, pw, ph); err == nil {
		return fmt.Errorf("Present after Close must fail explicitly")
	}
	if evs := w.Pump(); evs != nil {
		return fmt.Errorf("Pump after Close = %v, want nil", evs)
	}
	if w.Title() == "" {
		return fmt.Errorf("Title must survive Close")
	}
	return nil
}
