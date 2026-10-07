package failure

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/arief-fajri/gowez/internal/ipc"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/script"
)

// TestBJSExceptionIsObservable is failure experiment B
// (tests/failure/README.md): a JS handler throws. Expected outcome — the
// error is isolated and observable (Metrics.JSExceptions plus a
// Component "script" diagnostic), the native runtime stays controlled
// (the engine keeps evaluating, the dispatcher keeps serving), and
// nothing hangs (G-REL-01, G-REL-02).
//
// Induction: gowez.on registers a handler whose body throws. The whole
// run executes under a hard watchdog — a stall fails the test instead of
// blocking CI.
//
// Every run of this test must be recorded in evidence/experiments/.
func TestBJSExceptionIsObservable(t *testing.T) {
	rec := observe.NewRecorder()
	capture := &captureReporter{}

	disp := ipc.NewDispatcher()
	disp.SetObservation(rec, capture)
	eng, err := script.New(script.DefaultLimits, disp)
	if err != nil {
		t.Fatalf("script.New: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	// Control: a healthy handler works before the induced failure.
	if err := eng.Eval("ok.js", `gowez.on("ok", function () { return 42; });`); err != nil {
		t.Fatalf("eval ok handler: %v", err)
	}
	if err := eng.FireHandler("ok", json.RawMessage(`{"n":1}`)); err != nil {
		t.Fatalf("healthy handler failed: %v", err)
	}

	// Induce: the handler throws.
	if err := eng.Eval("boom.js", `gowez.on("boom", function () { throw new Error("handler exploded"); });`); err != nil {
		t.Fatalf("eval boom handler: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- eng.FireHandler("boom", nil) }()

	var errFire error
	select {
	case errFire = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("FireHandler hung after a JS exception (G-REL-01)")
	}
	if errFire == nil {
		t.Fatal("throwing handler must surface an error")
	}
	if !strings.Contains(errFire.Error(), "handler exploded") {
		t.Fatalf("error must carry the JS message, got: %v", errFire)
	}

	// Observable: metric counted and a script diagnostic reported.
	m := rec.Snapshot()
	if m.JSExceptions == 0 {
		t.Fatalf("JSExceptions = 0, want >= 1 (P5: failures must be measurable)")
	}
	found := false
	for _, d := range capture.snapshot() {
		if d.Component == "script" && d.Err != nil && strings.Contains(d.Err.Error(), "handler exploded") {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v, want a script diagnostic carrying the message", capture.snapshot())
	}

	// Native runtime stays controlled: the engine is reusable and the
	// dispatcher still serves calls after the exception.
	stuck := make(chan struct{})
	go func() {
		defer close(stuck)
		if err := eng.Eval("after.js", `gowez.on("after", function () { return "alive"; });`); err != nil {
			t.Errorf("engine unusable after exception: %v", err)
			return
		}
		if err := eng.FireHandler("after", nil); err != nil {
			t.Errorf("post-exception handler failed: %v", err)
		}
		if err := disp.Register("app.probe", "", func(ctx context.Context, p json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`"alive"`), nil
		}); err != nil {
			t.Errorf("dispatcher unusable after exception: %v", err)
			return
		}
		resp := disp.Dispatch(context.Background(), ipc.Request{Version: ipc.Version, ID: 1, Method: "app.probe"})
		if resp.Error != nil {
			t.Errorf("post-exception IPC failed: %+v", resp.Error)
		}
	}()
	select {
	case <-stuck:
	case <-time.After(10 * time.Second):
		t.Fatal("runtime stuck after a JS exception (G-REL-01)")
	}
}
