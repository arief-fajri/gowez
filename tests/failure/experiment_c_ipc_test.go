package failure

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arief-fajri/gowez/internal/ipc"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/script"
)

// TestCUnknownMethodIsDeterministic is failure experiment C
// (tests/failure/README.md): a call to an unregistered method. Expected
// outcome — deterministic error code -32601 (CodeMethodNotFound), no
// hang (G-REL-01), and the failure observable in metrics, both from Go
// (dispatcher) and from JS (gowez.invoke throws an Error whose .code is
// -32601 — docs/SCRIPT.md).
//
// Induction: no handler is ever registered for "app.nonexistent"; the
// call is attempted over both paths under one hard watchdog.
//
// Every run of this test must be recorded in evidence/experiments/.
func TestCUnknownMethodIsDeterministic(t *testing.T) {
	rec := observe.NewRecorder()
	capture := &captureReporter{}

	disp := ipc.NewDispatcher()
	disp.SetObservation(rec, capture)
	eng, err := script.New(script.DefaultLimits, disp)
	if err != nil {
		t.Fatalf("script.New: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	// JS path: the handler asserts the thrown error carries the contract
	// code; a wrong or missing code throws, which FireHandler reports.
	if err := eng.Eval("probe-unknown.js", `
		gowez.on("probeUnknown", function () {
			try {
				gowez.invoke("app.nonexistent");
				throw new Error("invoke of an unknown method must throw");
			} catch (e) {
				if (e.code !== -32601) {
					throw new Error("wrong code: " + e.code + " (" + e.message + ")");
				}
			}
			return true;
		});`); err != nil {
		t.Fatalf("eval probe handler: %v", err)
	}

	done := make(chan struct{})
	var goResp ipc.Response
	var jsErr error
	go func() {
		defer close(done)
		goResp = disp.Dispatch(context.Background(), ipc.Request{
			Version: ipc.Version,
			ID:      1,
			Method:  "app.nonexistent",
		})
		jsErr = eng.FireHandler("probeUnknown", nil)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("unknown method call did not settle within 10s: unbounded failure (G-REL-01)")
	}

	// Go path: deterministic code and a message that names the method.
	if goResp.Error == nil {
		t.Fatal("dispatcher must fail an unknown method")
	}
	if goResp.Error.Code != ipc.CodeMethodNotFound {
		t.Errorf("code = %d, want -32601", goResp.Error.Code)
	}
	if !strings.Contains(goResp.Error.Message, "app.nonexistent") {
		t.Errorf("message = %q, want it to name the method", goResp.Error.Message)
	}

	// JS path: .code reached the script layer intact (nil error means the
	// in-script assertion held).
	if jsErr != nil {
		t.Errorf("JS path: %v", jsErr)
	}

	// Observable in metrics (P5).
	m := rec.Snapshot()
	if m.IPCErrorCount == 0 {
		t.Errorf("IPCErrorCount = 0, want >= 1")
	}
	if m.JSExceptions != 0 {
		t.Errorf("JSExceptions = %d, want 0: a rejected invoke is an IPC error, not a JS crash", m.JSExceptions)
	}
}
