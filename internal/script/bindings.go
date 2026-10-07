package script

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dop251/goja"

	"github.com/arief-fajri/gowez/internal/ipc"
)

// Host surface names. The native functions are installed under these names
// and deleted again by the bootstrap, so the only thing JavaScript can see
// is the gowez object (G-SEC-01, G-DEP-02: no implicit native access).
const (
	nativeInvoke = "__gowez_invoke"
	nativeOn     = "__gowez_on"
)

// hostBootstrap builds the gowez object on top of the two native functions
// and removes them from the global scope. The native layer never throws: it
// returns {result} or {error:{code,message}}; raising a real JS Error with a
// .code property happens here, so catch(e) { e.code } works as documented.
const hostBootstrap = `(function (global) {
	var nativeInvoke = global.__gowez_invoke;
	var nativeOn = global.__gowez_on;
	delete global.__gowez_invoke;
	delete global.__gowez_on;

	function raise(code, message) {
		var e = new Error(message);
		e.code = code;
		throw e;
	}
	function unwrap(r) {
		if (r && r.error) {
			raise(r.error.code, r.error.message);
		}
		return r ? r.result : undefined;
	}

	global.gowez = {
		invoke: function (method, params) {
			return unwrap(nativeInvoke(method, params));
		},
		call: function (method, params) {
			return Promise.resolve().then(function () {
				return global.gowez.invoke(method, params);
			});
		},
		on: function (name, fn) {
			unwrap(nativeOn(name, fn));
			return true;
		}
	};
})(globalThis);`

// installHost registers the native functions and evaluates the bootstrap.
func (e *engine) installHost() error {
	if err := e.vm.Set(nativeInvoke, e.hostInvoke); err != nil {
		return fmt.Errorf("set %s: %w", nativeInvoke, err)
	}
	if err := e.vm.Set(nativeOn, e.hostOn); err != nil {
		return fmt.Errorf("set %s: %w", nativeOn, err)
	}
	if _, err := e.vm.RunScript("gowez-bootstrap.js", hostBootstrap); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	return nil
}

// hostInvoke is gowez.invoke's native half: encode params → dispatch →
// project the response into {result} or {error}. The call runs under the
// current JS call's context, so it inherits the Eval/handler deadline
// (G-REL-01) and is bounded again by the dispatcher's own timeout.
func (e *engine) hostInvoke(method string, params goja.Value) *goja.Object {
	obj := e.vm.NewObject()
	if e.dispatcher == nil {
		return e.ipcFailure(obj, ipc.CodeInternal, "script: no ipc dispatcher wired")
	}
	raw, err := encodeParams(params)
	if err != nil {
		return e.ipcFailure(obj, ipc.CodeInvalidParams, err.Error())
	}
	ctx := e.curCtx
	if ctx == nil {
		ctx = context.Background()
	}
	e.callID++
	resp := e.dispatcher.Dispatch(ctx, ipc.Request{
		Version: ipc.Version,
		ID:      e.callID,
		Method:  method,
		Params:  raw,
	})
	if resp.Error != nil {
		return e.ipcFailure(obj, resp.Error.Code, resp.Error.Message)
	}
	var result any
	if len(resp.Result) > 0 {
		// Handler payloads are produced by Go and are valid JSON by
		// construction; a failure here is an internal error.
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return e.ipcFailure(obj, ipc.CodeInternal, fmt.Sprintf("script: unparsable result for %q: %v", method, err))
		}
	}
	_ = obj.Set("result", result)
	return obj
}

// hostOn is gowez.on's native half: it validates and stores the JS callback
// under an explicit name. Exactly one deliberately registered function per
// name — never arbitrary Go exposure (G-SEC-01).
func (e *engine) hostOn(name string, fn goja.Value) *goja.Object {
	obj := e.vm.NewObject()
	if name == "" {
		return e.ipcFailure(obj, ipc.CodeInvalidParams, "script: empty handler name")
	}
	callable, ok := goja.AssertFunction(fn)
	if !ok {
		return e.ipcFailure(obj, ipc.CodeInvalidParams, fmt.Sprintf("script: handler %q is not a function", name))
	}
	if _, exists := e.handlers[name]; exists {
		return e.ipcFailure(obj, ipc.CodeInvalidRequest, fmt.Sprintf("script: handler %q already registered", name))
	}
	e.handlers[name] = callable
	_ = obj.Set("ok", true)
	return obj
}

// ipcFailure projects a failure into the native result shape {error:{code,message}}.
func (e *engine) ipcFailure(obj *goja.Object, code int, message string) *goja.Object {
	fail := e.vm.NewObject()
	_ = fail.Set("code", code)
	_ = fail.Set("message", message)
	_ = obj.Set("error", fail)
	return obj
}

// encodeParams converts a JS value to the request params payload. Undefined
// and null become no params at all (the schema makes params optional);
// anything not JSON-serializable fails explicitly with CodeInvalidParams.
func encodeParams(v goja.Value) (json.RawMessage, error) {
	// goja has no IsUndefined/IsNull on Value; both export as nil, and the
	// schema treats absent params and null params identically.
	if v == nil || v.Export() == nil {
		return nil, nil
	}
	b, err := json.Marshal(v.Export())
	if err != nil {
		return nil, fmt.Errorf("params are not JSON-serializable: %w", err)
	}
	return b, nil
}
