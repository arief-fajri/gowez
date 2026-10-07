package script

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arief-fajri/gowez/internal/ipc"
	"github.com/arief-fajri/gowez/internal/observe"
)

// collectReporter captures diagnostics for assertions (P5).
type collectReporter struct {
	mu sync.Mutex
	d  []observe.Diagnostic
}

func (c *collectReporter) Report(d observe.Diagnostic) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.d = append(c.d, d)
}

func (c *collectReporter) last() (observe.Diagnostic, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.d) == 0 {
		return observe.Diagnostic{}, false
	}
	return c.d[len(c.d)-1], true
}

type testEngine struct {
	Engine
	disp *ipc.Dispatcher
	rec  *observe.Recorder
	rep  *collectReporter
}

func newTestEngine(t *testing.T, limits Limits) *testEngine {
	t.Helper()
	d := ipc.NewDispatcher()
	rec := observe.NewRecorder()
	rep := &collectReporter{}
	d.SetObservation(rec, rep)
	eng, err := New(limits, d)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	eng.SetObservation(rec, rep)
	t.Cleanup(func() { _ = eng.Close() })
	return &testEngine{Engine: eng, disp: d, rec: rec, rep: rep}
}

func TestNewRejectsUnenforceableMemoryLimit(t *testing.T) {
	_, err := New(Limits{MemoryBytes: 1024}, nil)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}

func TestNewZeroLimitsAreBounded(t *testing.T) {
	eng, err := New(Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	e := eng.(*engine)
	if e.limits.EvalTimeout <= 0 || e.limits.HandlerTimeout <= 0 {
		t.Fatalf("unbounded limits: %+v", e.limits)
	}
}

func TestEvalAppliesGlobalsAcrossCalls(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	if err := te.Eval("setup.js", `var counter = 41;`); err != nil {
		t.Fatal(err)
	}
	if err := te.Eval("bump.js", `counter = counter + 1;`); err != nil {
		t.Fatal(err)
	}
	if err := te.Eval("check.js", `if (counter !== 42) { throw new Error("got " + counter); }`); err != nil {
		t.Fatalf("state did not persist: %v", err)
	}
}

func TestEvalExceptionIsolatedWithSourceAndStack(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	err := te.Eval("boom.js", `function named(){ throw new Error("boom"); } named();`)
	if err == nil {
		t.Fatal("expected error")
	}
	var jsErr *Error
	if !errors.As(err, &jsErr) {
		t.Fatalf("want *script.Error, got %T: %v", err, err)
	}
	if jsErr.Source != "boom.js" {
		t.Errorf("Source = %q", jsErr.Source)
	}
	if jsErr.Message != "boom" {
		t.Errorf("Message = %q", jsErr.Message)
	}
	if !strings.Contains(jsErr.Stack, "boom.js") {
		t.Errorf("Stack should name the source: %q", jsErr.Stack)
	}
	if !strings.Contains(err.Error(), "boom.js") {
		t.Errorf("error string should name the source: %v", err)
	}

	// Isolation: the runtime survives and stays usable (G-REL-02).
	if err := te.Eval("after.js", `1 + 1`); err != nil {
		t.Fatalf("engine unusable after exception: %v", err)
	}
	if got := te.rec.Snapshot().JSExceptions; got != 1 {
		t.Errorf("JSExceptions = %d, want 1", got)
	}
	if _, ok := te.rep.last(); !ok {
		t.Error("exception produced no diagnostic (classification D)")
	}
}

func TestEvalThrowPrimitiveStillIsolated(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	err := te.Eval("prim.js", `throw "plain string";`)
	if err == nil {
		t.Fatal("expected error")
	}
	var jsErr *Error
	if !errors.As(err, &jsErr) {
		t.Fatalf("want *script.Error, got %T: %v", err, err)
	}
	if jsErr.Message == "" {
		t.Error("primitive throw lost its message")
	}
}

func TestEvalTimeoutIsBoundedAndReusable(t *testing.T) {
	te := newTestEngine(t, Limits{EvalTimeout: 60 * time.Millisecond})
	start := time.Now()
	err := te.Eval("spin.js", `for (;;) {}`)
	elapsed := time.Since(start)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("eval not bounded: %v", elapsed)
	}
	// Reuse proves ClearInterrupt ran (I10: stoppable lifecycle).
	if err := te.Eval("ok.js", `42`); err != nil {
		t.Fatalf("engine unusable after timeout: %v", err)
	}
	if got := te.rec.Snapshot().JSExceptions; got != 1 {
		t.Errorf("timeout should count as an isolated JS failure, got %d", got)
	}
}

func TestEvalSyntaxErrorIsExplicit(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	err := te.Eval("syntax.js", `var = ;`)
	if err == nil {
		t.Fatal("expected syntax error")
	}
	if !strings.Contains(err.Error(), "syntax.js") {
		t.Errorf("syntax error must name the source: %v", err)
	}
	if err := te.Eval("ok.js", `1`); err != nil {
		t.Fatalf("engine unusable after syntax error: %v", err)
	}
}

func TestOperationsOnClosedEngineFailExplicitly(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	if err := te.Close(); err != nil {
		t.Fatal(err)
	}
	if err := te.Close(); err != nil {
		t.Fatalf("Close must be idempotent (I12): %v", err)
	}
	if err := te.Eval("x.js", `1`); !errors.Is(err, ErrClosed) {
		t.Errorf("Eval after close = %v", err)
	}
	if err := te.FireHandler("h", nil); !errors.Is(err, ErrClosed) {
		t.Errorf("FireHandler after close = %v", err)
	}
}

func TestFireHandlerRunsJSWithPayload(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	register := `gowez.on("counterClick", function (ev) { last = ev.type + ":" + ev.value; });`
	if err := te.Eval("handlers.js", register); err != nil {
		t.Fatal(err)
	}
	if !te.HasHandler("counterClick") {
		t.Fatal("handler not registered")
	}
	if err := te.FireHandler("counterClick", []byte(`{"type":"click","value":7}`)); err != nil {
		t.Fatalf("FireHandler: %v", err)
	}
	if err := te.Eval("read.js", `if (last !== "click:7") { throw new Error("got " + last); }`); err != nil {
		t.Fatalf("payload did not reach the handler: %v", err)
	}
}

func TestFireHandlerExceptionIsolatedAndCounted(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	if err := te.Eval("handlers.js", `gowez.on("bad", function () { throw new Error("handler boom"); });`); err != nil {
		t.Fatal(err)
	}
	err := te.FireHandler("bad", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var jsErr *Error
	if !errors.As(err, &jsErr) {
		t.Fatalf("want *script.Error, got %T: %v", err, err)
	}
	if jsErr.Source != "handler:bad" {
		t.Errorf("Source = %q", jsErr.Source)
	}
	if got := te.rec.Snapshot().JSExceptions; got != 1 {
		t.Errorf("JSExceptions = %d, want 1", got)
	}
	if _, ok := te.rep.last(); !ok {
		t.Error("no diagnostic")
	}
	// The runtime keeps working after a throwing handler (experiment B unit
	// shape): the next fire succeeds.
	if err := te.Eval("handlers.js", `gowez.on("good", function (ev) { lastGood = ev.v; });`); err != nil {
		t.Fatalf("engine unusable after handler exception: %v", err)
	}
	if err := te.FireHandler("good", []byte(`{"v":1}`)); err != nil {
		t.Fatalf("second handler failed: %v", err)
	}
}

func TestFireHandlerUnknownNameIsWiringMissNotJSException(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	err := te.FireHandler("nope", nil)
	if !errors.Is(err, ErrHandlerNotFound) {
		t.Fatalf("want ErrHandlerNotFound, got %v", err)
	}
	if got := te.rec.Snapshot().JSExceptions; got != 0 {
		t.Errorf("wiring miss must not count as a JS exception, got %d", got)
	}
	if _, ok := te.rep.last(); !ok {
		t.Error("wiring miss produced no diagnostic")
	}
}

func TestFireHandlerTimeoutIsBounded(t *testing.T) {
	te := newTestEngine(t, Limits{HandlerTimeout: 50 * time.Millisecond})
	if err := te.Eval("handlers.js", `gowez.on("spin", function () { for (;;) {} });`); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err := te.FireHandler("spin", nil)
	elapsed := time.Since(start)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("handler not bounded: %v", elapsed)
	}
	// The engine must remain usable (ClearInterrupt ran despite the
	// concurrent timer — the done-flag race in bounded()).
	if err := te.Eval("ok.js", `7`); err != nil {
		t.Fatalf("engine unusable after handler timeout: %v", err)
	}
}

// TestSandboxSurface pins the sandbox contract (G-SEC-01, G-DEP-02): the
// only host capability visible to JavaScript is the gowez object; the
// natives are gone from the global scope and no ambient Go/Node surface
// exists.
func TestSandboxSurface(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	src := `
		var problems = [];
		if (typeof gowez.invoke !== 'function') problems.push('invoke');
		if (typeof gowez.call !== 'function') problems.push('call');
		if (typeof gowez.on !== 'function') problems.push('on');
		if (typeof __gowez_invoke !== 'undefined') problems.push('native invoke leaked');
		if (typeof __gowez_on !== 'undefined') problems.push('native on leaked');
		if (typeof require !== 'undefined') problems.push('require');
		if (typeof process !== 'undefined') problems.push('process');
		if (typeof Go !== 'undefined') problems.push('Go');
		if (typeof setTimeout !== 'undefined') problems.push('setTimeout');
		if (typeof console !== 'undefined') problems.push('console');
		if (typeof window !== 'undefined') problems.push('window');
		if (typeof document !== 'undefined') problems.push('document');
		if (typeof fetch !== 'undefined') problems.push('fetch');
		if (Object.keys(gowez).sort().join(',') !== 'call,invoke,on') problems.push('surface: ' + Object.keys(gowez));
		if (problems.length) { throw new Error(problems.join('; ')); }
	`
	if err := te.Eval("sandbox.js", src); err != nil {
		t.Fatalf("sandbox surface violated: %v", err)
	}
}
