package script

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/arief-fajri/gowez/internal/api"
	"github.com/arief-fajri/gowez/internal/permission"
)

// newIPCTestEngine wires an engine over a dispatcher exposing the default
// api registry — the real M4 path (JS → dispatcher → registry).
func newIPCTestEngine(t *testing.T, limits Limits) *testEngine {
	t.Helper()
	te := newTestEngine(t, limits)
	reg, err := api.NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if err := te.disp.RegisterRegistry(reg); err != nil {
		t.Fatal(err)
	}
	return te
}

func TestInvokeSuccessReturnsResult(t *testing.T) {
	te := newIPCTestEngine(t, DefaultLimits)
	src := `var info = gowez.invoke("app.getInfo");`
	if err := te.Eval("invoke.js", src); err != nil {
		t.Fatalf("invoke threw: %v", err)
	}
	check := `if (info.name !== "gowez") throw new Error("name=" + info.name);
	           if (info.ipcVersion !== 1) throw new Error("ipcVersion=" + info.ipcVersion);
	           if (info.engine !== "goja") throw new Error("engine=" + info.engine);`
	if err := te.Eval("check.js", check); err != nil {
		t.Fatalf("result wrong: %v", err)
	}
	m := te.rec.Snapshot()
	if m.IPCCount != 1 || m.IPCErrorCount != 0 {
		t.Errorf("metrics = %+v", m)
	}
}

func TestInvokeUnknownMethodThrowsWithCode(t *testing.T) {
	te := newIPCTestEngine(t, DefaultLimits)
	src := `var caught = null;
	        try { gowez.invoke("eval.exec"); } catch (e) { caught = { code: e.code, msg: e.message, isErr: e instanceof Error }; }`
	if err := te.Eval("unknown.js", src); err != nil {
		t.Fatalf("invoke must throw, not fail the eval: %v", err)
	}
	check := `if (!caught) throw new Error("did not throw");
	          if (caught.code !== -32601) throw new Error("code=" + caught.code);
	          if (!caught.isErr) throw new Error("not an Error instance");
	          if (caught.msg.indexOf("eval.exec") < 0) throw new Error("msg=" + caught.msg);`
	if err := te.Eval("check.js", check); err != nil {
		t.Fatalf("throw shape wrong: %v", err)
	}
}

func TestInvokePermissionDeniedThrowsWithCode(t *testing.T) {
	te := newIPCTestEngine(t, DefaultLimits) // no grants: deny by default
	src := `var caught = null;
	        try { gowez.invoke("fs.readTextFile", { path: "/etc/hosts" }); } catch (e) { caught = e; }`
	if err := te.Eval("denied.js", src); err != nil {
		t.Fatal(err)
	}
	if err := te.Eval("check.js", `if (caught.code !== -32000) throw new Error("code=" + caught.code);`); err != nil {
		t.Fatalf("gate not enforced from JS: %v", err)
	}
	// With the grant the same call reaches the (M6 stub) handler and fails
	// explicitly with an internal code instead of succeeding silently.
	te.disp.SetGrants(permission.NewSet(permission.FSRead))
	src = `var caught2 = null;
	       try { gowez.invoke("fs.readTextFile", { path: "/etc/hosts" }); } catch (e) { caught2 = e; }`
	if err := te.Eval("granted.js", src); err != nil {
		t.Fatal(err)
	}
	if err := te.Eval("check2.js", `if (caught2.code !== -32603) throw new Error("code=" + caught2.code);`); err != nil {
		t.Fatalf("granted path wrong: %v", err)
	}
}

func TestInvokeRoundTripsParamsAndResult(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	var got json.RawMessage
	if err := te.disp.Register("echo.upper", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		got = append(json.RawMessage(nil), params...)
		return json.RawMessage(`{"echo": true, "n": 3}`), nil
	}); err != nil {
		t.Fatal(err)
	}
	src := `var r = gowez.invoke("echo.upper", { text: "hi", n: 3 });`
	if err := te.Eval("roundtrip.js", src); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	var gotParams map[string]any
	if err := json.Unmarshal(got, &gotParams); err != nil {
		t.Fatalf("params reached Go as %q: %v", got, err)
	}
	if gotParams["text"] != "hi" || gotParams["n"] != float64(3) {
		t.Errorf("params = %+v", gotParams)
	}
	if err := te.Eval("check.js", `if (!r.echo || r.n !== 3) throw new Error(JSON.stringify(r));`); err != nil {
		t.Fatalf("result: %v", err)
	}
}

func TestInvokeRejectsNonSerializableParams(t *testing.T) {
	te := newIPCTestEngine(t, DefaultLimits)
	src := `var caught = null;
	        try { gowez.invoke("app.getInfo", function () {}); } catch (e) { caught = e; }`
	if err := te.Eval("badparams.js", src); err != nil {
		t.Fatal(err)
	}
	if err := te.Eval("check.js", `if (caught.code !== -32602) throw new Error("code=" + caught.code);`); err != nil {
		t.Fatalf("non-serializable params must fail explicitly: %v", err)
	}
}

func TestInvokeWithoutDispatcherFailsExplicitly(t *testing.T) {
	eng, err := New(DefaultLimits, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	err = eng.Eval("nodisp.js", `gowez.invoke("app.getInfo")`)
	if err == nil {
		t.Fatal("invoke without dispatcher must throw")
	}
	if err := eng.Eval("check.js", `var c=null; try { gowez.invoke("app.getInfo"); } catch(e) { c=e; } if (c.code !== -32603) throw new Error("code=" + c.code);`); err != nil {
		t.Fatalf("missing dispatcher not surfaced: %v", err)
	}
}

// TestCallReturnsSettledPromise proves the async wrapper: gowez.call returns
// a Promise whose reaction has already run by the time Eval returns (goja
// drains the microtask queue — Phase 1 spike assumption).
func TestCallReturnsSettledPromise(t *testing.T) {
	te := newIPCTestEngine(t, DefaultLimits)
	src := `var settled = null;
	        gowez.call("app.getInfo").then(function (info) { settled = info.name; });
	        if (settled !== null) { throw new Error("then ran synchronously: " + settled); }`
	if err := te.Eval("async.js", src); err != nil {
		t.Fatalf("call: %v", err)
	}
	if err := te.Eval("check.js", `if (settled !== "gowez") throw new Error("settled=" + settled);`); err != nil {
		t.Fatalf("promise did not settle: %v", err)
	}
}

func TestCallRejectsWithCode(t *testing.T) {
	te := newIPCTestEngine(t, DefaultLimits)
	src := `var caught = null;
	        gowez.call("no.such.method").catch(function (e) { caught = e.code; });`
	if err := te.Eval("async.js", src); err != nil {
		t.Fatal(err)
	}
	if err := te.Eval("check.js", `if (caught !== -32601) throw new Error("code=" + caught);`); err != nil {
		t.Fatalf("rejection: %v", err)
	}
}

func TestOnRejectsInvalidRegistrations(t *testing.T) {
	te := newTestEngine(t, DefaultLimits)
	src := `var errs = {};
	        try { gowez.on("", function () {}); } catch (e) { errs.empty = e.code; }
	        try { gowez.on("notFn", 42); } catch (e) { errs.notFn = e.code; }
	        gowez.on("once", function () {});
	        try { gowez.on("once", function () {}); } catch (e) { errs.dup = e.code; }`
	if err := te.Eval("on.js", src); err != nil {
		t.Fatal(err)
	}
	check := `if (errs.empty !== -32602) throw new Error("empty=" + errs.empty);
	          if (errs.notFn !== -32602) throw new Error("notFn=" + errs.notFn);
	          if (errs.dup !== -32600) throw new Error("dup=" + errs.dup);`
	if err := te.Eval("check.js", check); err != nil {
		t.Fatalf("registration validation: %v", err)
	}
	if !te.HasHandler("once") {
		t.Fatal("valid handler not stored")
	}
}

// TestInvokeIsBoundedByHandlerBudget: an IPC call made from inside a
// handler cannot outlive the handler's deadline — the invoke inherits
// FireHandler's context (G-REL-01 end to end).
func TestInvokeIsBoundedByHandlerBudget(t *testing.T) {
	te := newTestEngine(t, Limits{HandlerTimeout: 80 * time.Millisecond, EvalTimeout: 2 * time.Second})
	block := make(chan struct{})
	defer close(block)
	if err := te.disp.Register("app.hang", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		<-block // ignores ctx: worst case
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := te.Eval("handlers.js", `gowez.on("hang", function () { gowez.invoke("app.hang"); });`); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err := te.FireHandler("hang", nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected failure")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("handler+invoke not bounded: %v", elapsed)
	}
}
