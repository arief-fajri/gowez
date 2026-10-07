package script

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
)

// TestSpikeExceptionCarriesMessageAndStack verifies assumption (a): a thrown
// exception surfaces as *goja.Exception with a message and a stack usable for
// script.Error.
func TestSpikeExceptionCarriesMessageAndStack(t *testing.T) {
	vm := goja.New()
	_, err := vm.RunString(`(function named(){ throw new Error("boom"); })()`)
	if err == nil {
		t.Fatal("expected error")
	}
	var ex *goja.Exception
	if !errors.As(err, &ex) {
		t.Fatalf("expected *goja.Exception, got %T: %v", err, err)
	}
	if !strings.Contains(ex.Error(), "boom") {
		t.Fatalf("message missing boom: %v", ex)
	}
	t.Logf("exception: %v", ex)
	v := ex.Value()
	if o, ok := v.(*goja.Object); ok {
		t.Logf("stack: %v", o.Get("stack"))
	}
}

// TestSpikeInterruptBoundsInfiniteLoop verifies assumption (b): Interrupt stops
// a runaway loop and ClearInterrupt makes the runtime reusable.
func TestSpikeInterruptBoundsInfiniteLoop(t *testing.T) {
	vm := goja.New()
	timer := time.AfterFunc(50*time.Millisecond, func() { vm.Interrupt("halt") })
	defer timer.Stop()
	start := time.Now()
	_, err := vm.RunString(`for(;;){}`)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected interrupt error")
	}
	var ie *goja.InterruptedError
	if !errors.As(err, &ie) {
		t.Fatalf("expected *goja.InterruptError, got %T: %v", err, err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("interrupt not bounded: %v", elapsed)
	}
	t.Logf("interrupted after %v: %v", elapsed, err)

	// Reuse: without ClearInterrupt the next run fails immediately.
	vm.ClearInterrupt()
	v, err := vm.RunString(`1+1`)
	if err != nil {
		t.Fatalf("reuse after ClearInterrupt failed: %v", err)
	}
	if v.Export().(int64) != 2 {
		t.Fatalf("wrong result %v", v)
	}
}

// TestSpikePromiseJobDrain verifies assumption (c): whether promise reaction
// jobs queued by RunString execute before RunString returns (microtask drain),
// which decides how the async wrapper is implemented.
func TestSpikePromiseJobDrain(t *testing.T) {
	vm := goja.New()
	_, err := vm.RunString(`
		var order = [];
		order.push('sync-start');
		Promise.resolve().then(() => order.push('microtask'));
		order.push('sync-end');
	`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	v, err := vm.RunString(`order.join(',')`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := v.Export().(string)
	t.Logf("order after RunString returned: %s", got)
	// Documented expectation for the wrapper design; failure here is a design
	// input (spike), not a product bug — see evidence/learnings.md.
	if got != "sync-start,sync-end,microtask" {
		t.Logf("NOTE: microtask ran inline or not at all: %q", got)
	}
}

// TestSpikeHostPromise verifies host-created promises settle and expose
// resolve/reject for the invoke bridge.
func TestSpikeHostPromise(t *testing.T) {
	vm := goja.New()
	p, resolve, reject := vm.NewPromise()
	if err := vm.Set("p", p); err != nil {
		t.Fatal(err)
	}
	_, err := vm.RunString(`
		var seen = [];
		p.then(v => seen.push('ok:'+v)).catch(e => seen.push('err:'+e));
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolve("done"); err != nil {
		t.Fatal(err)
	}
	// Whether the reaction ran already depends on the drain semantics proven
	// above; read whatever state exists now, then after another tick.
	v, _ := vm.RunString(`seen.join(',')`)
	t.Logf("seen immediately: %q", v.Export().(string))
	_ = reject
}

// TestSpikeCallDrainsMicrotasks verifies that promise jobs queued inside a
// vm.Call (the FireHandler path) run before Call returns, so JS handlers that
// use gowez.call(...) settle without an explicit drain.
func TestSpikeCallDrainsMicrotasks(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`var order = []; function handler(){ Promise.resolve().then(()=>order.push('after-call')); }`); err != nil {
		t.Fatal(err)
	}
	fn, ok := goja.AssertFunction(vm.Get("handler"))
	if !ok {
		t.Fatal("handler is not callable")
	}
	if _, err := fn(goja.Undefined()); err != nil {
		t.Fatal(err)
	}
	v, err := vm.RunString(`order.join(',')`)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("order after Call: %q", v.Export().(string))
	if v.Export().(string) != "after-call" {
		t.Fatalf("Call did not drain microtasks: %q", v.Export().(string))
	}
}
