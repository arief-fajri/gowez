package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arief-fajri/gowez/internal/api"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/permission"
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

func okHandler(result string) Handler {
	return func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(result), nil
	}
}

func mustRegister(t *testing.T, d *Dispatcher, method string, perm permission.Permission, h Handler) {
	t.Helper()
	if err := d.Register(method, perm, h); err != nil {
		t.Fatalf("register %s: %v", method, err)
	}
}

func req(method string) Request {
	return Request{Version: Version, ID: 7, Method: method, Params: json.RawMessage(`{}`)}
}

func TestDispatchSuccess(t *testing.T) {
	d := NewDispatcher()
	mustRegister(t, d, "app.echo", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		return params, nil
	})

	got := d.Dispatch(context.Background(), req("app.echo"))
	if got.Error != nil {
		t.Fatalf("unexpected error: %+v", got.Error)
	}
	if got.Version != Version || got.ID != 7 {
		t.Fatalf("envelope not echoed: %+v", got)
	}
	if string(got.Result) != `{}` {
		t.Fatalf("result = %s", got.Result)
	}
}

func TestDispatchNilResultBecomesNull(t *testing.T) {
	d := NewDispatcher()
	mustRegister(t, d, "app.nil", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		return nil, nil
	})
	got := d.Dispatch(context.Background(), req("app.nil"))
	if got.Error != nil {
		t.Fatalf("unexpected error: %+v", got.Error)
	}
	if string(got.Result) != "null" {
		t.Fatalf("schema requires exactly one of result/error, got %q", got.Result)
	}
}

func TestDispatchUnknownMethodIsDeterministic(t *testing.T) {
	d := NewDispatcher()
	rep := &collectReporter{}
	rec := observe.NewRecorder()
	d.SetObservation(rec, rep)

	for i := 0; i < 3; i++ {
		got := d.Dispatch(context.Background(), req("nope.unknown"))
		if got.Error == nil || got.Error.Code != CodeMethodNotFound {
			t.Fatalf("attempt %d: want -32601, got %+v", i, got.Error)
		}
	}
	m := rec.Snapshot()
	if m.IPCCount != 3 || m.IPCErrorCount != 3 {
		t.Fatalf("metrics = %+v", m)
	}
	if _, ok := rep.last(); !ok {
		t.Fatal("failure produced no diagnostic (classification D)")
	}
}

func TestDispatchRejectsWrongVersion(t *testing.T) {
	d := NewDispatcher()
	bad := Request{Version: 2, ID: 1, Method: "app.any"}
	got := d.Dispatch(context.Background(), bad)
	if got.Error == nil || got.Error.Code != CodeInvalidRequest {
		t.Fatalf("want -32600, got %+v", got.Error)
	}
	if !strings.Contains(got.Error.Message, "version") {
		t.Fatalf("message should name the version: %q", got.Error.Message)
	}
	// The response still carries the current schema version so the caller
	// can detect the mismatch on the reply side too.
	if got.Version != Version {
		t.Fatalf("response version = %d", got.Version)
	}
}

func TestDispatchRejectsEmptyMethod(t *testing.T) {
	d := NewDispatcher()
	got := d.Dispatch(context.Background(), Request{Version: Version, ID: 1})
	if got.Error == nil || got.Error.Code != CodeInvalidRequest {
		t.Fatalf("want -32600, got %+v", got.Error)
	}
}

func TestDispatchRejectsMalformedParams(t *testing.T) {
	d := NewDispatcher()
	mustRegister(t, d, "app.x", "", okHandler(`{}`))

	for _, params := range []string{`5`, `"str"`, `true`, `{bad`, `[`} {
		got := d.Dispatch(context.Background(), Request{Version: Version, ID: 1, Method: "app.x", Params: json.RawMessage(params)})
		if got.Error == nil || got.Error.Code != CodeInvalidParams {
			t.Fatalf("params %q: want -32602, got %+v", params, got.Error)
		}
	}
}

func TestDispatchPermissionGate(t *testing.T) {
	d := NewDispatcher()
	mustRegister(t, d, "fs.readTextFile", permission.FSRead, okHandler(`"data"`))
	mustRegister(t, d, "app.getInfo", "", okHandler(`{"name":"gowez"}`))

	// Deny by default: no grants set at all.
	got := d.Dispatch(context.Background(), req("fs.readTextFile"))
	if got.Error == nil || got.Error.Code != CodePermissionDenied {
		t.Fatalf("want -32000 without grants, got %+v", got.Error)
	}
	// Non-native method needs no grant.
	if got := d.Dispatch(context.Background(), req("app.getInfo")); got.Error != nil {
		t.Fatalf("app.getInfo denied: %+v", got.Error)
	}
	// Explicit grant opens the gate.
	d.SetGrants(permission.NewSet(permission.FSRead))
	if got := d.Dispatch(context.Background(), req("fs.readTextFile")); got.Error != nil {
		t.Fatalf("grant should allow: %+v", got.Error)
	}
	// An empty (but present) set still denies.
	d.SetGrants(permission.NewSet())
	if got := d.Dispatch(context.Background(), req("fs.readTextFile")); got.Error == nil {
		t.Fatal("empty grant set must deny")
	}
}

func TestDispatchTimeoutIsBounded(t *testing.T) {
	d := NewDispatcher()
	d.SetCallTimeout(50 * time.Millisecond)
	block := make(chan struct{})
	defer close(block)
	mustRegister(t, d, "app.hang", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		// Ignores ctx on purpose — the worst-case handler (G-REL-01).
		<-block
		return nil, nil
	})

	start := time.Now()
	got := d.Dispatch(context.Background(), req("app.hang"))
	elapsed := time.Since(start)
	if got.Error == nil || got.Error.Code != CodeTimeout {
		t.Fatalf("want -32001, got %+v", got.Error)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("dispatch not bounded: %v", elapsed)
	}
}

func TestDispatchTimeoutRespectsShorterCallerDeadline(t *testing.T) {
	d := NewDispatcher()
	mustRegister(t, d, "app.slow", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return nil, nil
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	got := d.Dispatch(ctx, req("app.slow"))
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("caller deadline ignored: %v", elapsed)
	}
	if got.Error == nil || got.Error.Code != CodeTimeout {
		t.Fatalf("want -32001, got %+v", got.Error)
	}
}

func TestDispatchRecoversHandlerPanic(t *testing.T) {
	d := NewDispatcher()
	rep := &collectReporter{}
	rec := observe.NewRecorder()
	d.SetObservation(rec, rep)
	mustRegister(t, d, "app.boom", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		panic("handler exploded")
	})

	got := d.Dispatch(context.Background(), req("app.boom"))
	if got.Error == nil || got.Error.Code != CodeInternal {
		t.Fatalf("want -32603, got %+v", got.Error)
	}
	if !strings.Contains(got.Error.Message, "panicked") {
		t.Fatalf("message should say panicked: %q", got.Error.Message)
	}
	if _, ok := rep.last(); !ok {
		t.Fatal("panic produced no diagnostic")
	}
	if rec.Snapshot().IPCErrorCount != 1 {
		t.Fatal("panic not counted as failed IPC")
	}
}

func TestDispatchUsesHandlerCodeError(t *testing.T) {
	d := NewDispatcher()
	mustRegister(t, d, "ui.setText", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		return nil, NewCodeError(CodeInvalidParams, "unknown node 42")
	})
	got := d.Dispatch(context.Background(), req("ui.setText"))
	if got.Error == nil || got.Error.Code != CodeInvalidParams {
		t.Fatalf("want -32602 from CodeError, got %+v", got.Error)
	}
	if got.Error.Message != "unknown node 42" {
		t.Fatalf("message = %q", got.Error.Message)
	}
}

func TestCodeForMapping(t *testing.T) {
	cases := map[error]int{
		context.DeadlineExceeded:                     CodeTimeout,
		context.Canceled:                             CodeTimeout,
		api.ErrNotFound:                              CodeMethodNotFound,
		api.ErrNotImplemented:                        CodeInternal,
		NewCodeError(CodePermissionDenied, "denied"): CodePermissionDenied,
		errors.New("anything"):                       CodeInternal,
	}
	for err, want := range cases {
		if got := CodeFor(err); got != want {
			t.Errorf("CodeFor(%v) = %d, want %d", err, got, want)
		}
	}
	// Wrapped forms must resolve the same way.
	if got := CodeFor(errors.Join(api.ErrNotFound)); got != CodeMethodNotFound {
		t.Errorf("wrapped ErrNotFound → %d", got)
	}
}

func TestRegisterRejectsBadInput(t *testing.T) {
	d := NewDispatcher()
	if err := d.Register("", "", okHandler(`{}`)); err == nil {
		t.Error("empty method accepted")
	}
	if err := d.Register("app.x", "", nil); err == nil {
		t.Error("nil handler accepted")
	}
	mustRegister(t, d, "app.x", "", okHandler(`{}`))
	if err := d.Register("app.x", "", okHandler(`{}`)); err == nil {
		t.Error("duplicate accepted")
	}
}

func TestMethodsIsDeterministic(t *testing.T) {
	d := NewDispatcher()
	mustRegister(t, d, "window.setTitle", "", okHandler(`null`))
	mustRegister(t, d, "app.getInfo", "", okHandler(`null`))
	mustRegister(t, d, "fs.readTextFile", "", okHandler(`null`))
	got := d.Methods()
	want := []string{"app.getInfo", "fs.readTextFile", "window.setTitle"}
	if len(got) != len(want) {
		t.Fatalf("Methods() = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Methods() = %v, want %v", got, want)
		}
	}
}

// TestRegisterRegistryExposesDefaultAPIs proves the checklist chain end to
// end: the registry is the only door, unknown methods fail with -32601,
// permission-less methods succeed, permissioned methods are denied without
// a grant, and the built-in app.getInfo success path carries the current
// schema version (drift guard: api hardcodes ipcVersion).
func TestRegisterRegistryExposesDefaultAPIs(t *testing.T) {
	reg, err := api.NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	d := NewDispatcher()
	if err := d.RegisterRegistry(reg); err != nil {
		t.Fatal(err)
	}

	// Success path.
	got := d.Dispatch(context.Background(), req("app.getInfo"))
	if got.Error != nil {
		t.Fatalf("app.getInfo: %+v", got.Error)
	}
	var info struct {
		Name       string `json:"name"`
		IPCVersion int    `json:"ipcVersion"`
		Engine     string `json:"engine"`
	}
	if err := json.Unmarshal(got.Result, &info); err != nil {
		t.Fatalf("payload: %v (%s)", err, got.Result)
	}
	if info.IPCVersion != Version {
		t.Fatalf("api reports ipcVersion %d, dispatcher speaks %d", info.IPCVersion, Version)
	}
	if info.Engine != "goja" {
		t.Fatalf("engine = %q", info.Engine)
	}

	// Permission path: fs needs FSRead; no grants → denied, never executed.
	got = d.Dispatch(context.Background(), req("fs.readTextFile"))
	if got.Error == nil || got.Error.Code != CodePermissionDenied {
		t.Fatalf("fs without grant: want -32000, got %+v", got.Error)
	}

	// Explicit error path: grant exactly one permission — the registered
	// native stub then runs and returns its explicit not-implemented error,
	// while fs (a different permission) stays denied.
	d.SetGrants(permission.NewSet(permission.WindowCtl))
	got = d.Dispatch(context.Background(), req("window.setTitle"))
	if got.Error == nil || got.Error.Code != CodeInternal {
		t.Fatalf("stub should return -32603, got %+v", got.Error)
	}
	if !strings.Contains(got.Error.Message, "Milestone 6") {
		t.Fatalf("stub message should be explicit: %q", got.Error.Message)
	}
	got = d.Dispatch(context.Background(), req("fs.readTextFile"))
	if got.Error == nil || got.Error.Code != CodePermissionDenied {
		t.Fatalf("grant of window:control must not open fs:read, got %+v", got.Error)
	}

	// Unknown path.
	got = d.Dispatch(context.Background(), req("eval.exec"))
	if got.Error == nil || got.Error.Code != CodeMethodNotFound {
		t.Fatalf("want -32601, got %+v", got.Error)
	}

	// Nothing beyond the registered set is reachable (G-SEC-01): every
	// reachable name comes from the registry.
	if len(d.Methods()) != len(reg.Names()) {
		t.Fatalf("dispatcher surface %v != registry %v", d.Methods(), reg.Names())
	}
}

func TestDispatchConcurrentIsRaceFree(t *testing.T) {
	d := NewDispatcher()
	mustRegister(t, d, "app.n", "", okHandler(`1`))
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := d.Dispatch(context.Background(), req("app.n")); got.Error != nil {
				t.Errorf("dispatch: %+v", got.Error)
			}
		}()
	}
	wg.Wait()
}

// TestInlineHandlerBypassesTimeout documents the RegisterInline contract:
// inline handlers run on the caller's goroutine and cannot be preempted, so
// the deadline does not cut them — which is exactly why they must stay O(1)
// and non-blocking (UI-tree mutations).
func TestInlineHandlerBypassesTimeout(t *testing.T) {
	d := NewDispatcher()
	d.SetCallTimeout(30 * time.Millisecond)
	mustRegister := func() {
		t.Helper()
		if err := d.RegisterInline("ui.setText", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
			time.Sleep(120 * time.Millisecond)
			return json.RawMessage(`true`), nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	mustRegister()

	start := time.Now()
	got := d.Dispatch(context.Background(), req("ui.setText"))
	elapsed := time.Since(start)
	if got.Error != nil {
		t.Fatalf("inline handler must succeed: %+v", got.Error)
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("inline handler was cut short: %v", elapsed)
	}

	// The same body via Register is bounded — the contrast is the contract.
	if err := d.Register("app.bounded", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		time.Sleep(120 * time.Millisecond)
		return json.RawMessage(`true`), nil
	}); err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	got = d.Dispatch(context.Background(), req("app.bounded"))
	elapsed = time.Since(start)
	if got.Error == nil || got.Error.Code != CodeTimeout {
		t.Fatalf("bounded mode must time out, got %+v", got.Error)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("bounded mode not bounded: %v", elapsed)
	}
}

// TestInlineHandlerPanicRecovered: a panicking UI mutation is converted to
// an internal error instead of crashing the caller's goroutine.
func TestInlineHandlerPanicRecovered(t *testing.T) {
	d := NewDispatcher()
	if err := d.RegisterInline("ui.boom", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		panic("inline boom")
	}); err != nil {
		t.Fatal(err)
	}
	got := d.Dispatch(context.Background(), req("ui.boom"))
	if got.Error == nil || got.Error.Code != CodeInternal {
		t.Fatalf("want -32603, got %+v", got.Error)
	}
}
