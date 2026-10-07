package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/arief-fajri/gowez/internal/api"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/permission"
)

// Version is the IPC schema version carried by every request and response
// (protocol/ipc.schema.json). A mismatch is rejected with CodeInvalidRequest;
// the wire contract changes only with a version bump (G-IFACE-02, G-IFACE-03).
const Version = 1

// DefaultCallTimeout bounds one dispatch when the caller's context carries no
// shorter deadline (G-REL-01): no IPC call ever waits forever, even when a
// handler ignores its context.
const DefaultCallTimeout = 2 * time.Second

// Handler executes one method and returns its JSON result payload. Handlers
// are registered explicitly — there is no reflection-based exposure of
// arbitrary Go functions (G-SEC-01). Handlers must respect ctx (G-REL-01);
// a handler that ignores it can outlive its deadline but never blocks the
// caller past the bound.
type Handler func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)

type entry struct {
	handler Handler
	perm    permission.Permission
	// inline runs the handler on the caller's goroutine instead of a
	// bounded worker (see RegisterInline).
	inline bool
}

// Dispatcher routes requests to registered handlers behind the permission
// gate. It is safe for concurrent Dispatch calls; registration is expected
// during wiring, before dispatching starts.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string]entry
	grants   *permission.Set
	timeout  time.Duration
	metrics  *observe.Recorder
	reporter observe.Reporter
}

// NewDispatcher creates an empty dispatcher with DefaultCallTimeout.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string]entry),
		timeout:  DefaultCallTimeout,
	}
}

// SetGrants installs the permission set evaluated by the gate (G-SEC-02).
// A nil set denies every permission-bound method — deny by default.
// Call during wiring, before the first Dispatch.
func (d *Dispatcher) SetGrants(g *permission.Set) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.grants = g
}

// SetObservation wires the metrics recorder and the diagnostics reporter
// (P5: every IPC failure is observable). Nil values leave the sink unset.
// Call during wiring, before the first Dispatch.
func (d *Dispatcher) SetObservation(rec *observe.Recorder, rep observe.Reporter) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.metrics, d.reporter = rec, rep
}

// SetCallTimeout overrides DefaultCallTimeout. Call during wiring or in
// tests, before the first Dispatch.
func (d *Dispatcher) SetCallTimeout(t time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t > 0 {
		d.timeout = t
	}
}

// Register binds a method name to a handler and the permission the gate
// requires to reach it (empty = no permission required — reserved for
// non-native methods such as app.getInfo). Duplicate or empty registrations
// are rejected — a contract mistake must fail loudly (G-IFACE-01).
func (d *Dispatcher) Register(method string, perm permission.Permission, h Handler) error {
	if method == "" {
		return errors.New("ipc: empty method name")
	}
	if h == nil {
		return fmt.Errorf("ipc: nil handler for method %q", method)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.handlers[method]; exists {
		return fmt.Errorf("ipc: method %q already registered", method)
	}
	d.handlers[method] = entry{handler: h, perm: perm}
	return nil
}

// RegisterInline binds a method that executes on the caller's goroutine.
// It exists for runtime-owned methods that mutate the UI tree: the tree is
// single-goroutine (internal/ui/tree.go), so its handlers must run where
// the call originates — the UI goroutine.
//
// Contract: inline handlers MUST NOT block and MUST be O(1) memory
// operations. The dispatcher cannot preempt them, so the bounded worker
// path (G-REL-01) does not apply here; anything that may block belongs in
// Register. The gate, version, and lookup checks are identical.
func (d *Dispatcher) RegisterInline(method string, perm permission.Permission, h Handler) error {
	if err := d.Register(method, perm, h); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	e := d.handlers[method]
	e.inline = true
	d.handlers[method] = e
	return nil
}

// Methods lists the registered method names in lexical order. The set is the
// complete IPC surface: anything not listed fails with CodeMethodNotFound —
// JavaScript can never reach an unregistered Go function (G-SEC-01).
func (d *Dispatcher) Methods() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	names := make([]string, 0, len(d.handlers))
	for n := range d.handlers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// RegisterRegistry exposes every method of an api.Registry through the
// dispatcher, carrying each method's declared permission into the gate.
// The registry stays the single door to native operations (G-SEC-01); this
// adapter is the only place ipc and api meet.
func (d *Dispatcher) RegisterRegistry(reg *api.Registry) error {
	if reg == nil {
		return errors.New("ipc: nil registry")
	}
	for _, name := range reg.Names() {
		perm := reg.Permission(name)
		name := name
		if err := d.Register(name, perm, func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
			return reg.Invoke(ctx, name, params)
		}); err != nil {
			return err
		}
	}
	return nil
}

// Dispatch resolves one request: schema version (G-IFACE-02) → parameter
// shape → handler lookup (deterministic unknown-method error) → permission
// gate (G-SEC-02) → bounded execution (G-REL-01). It always returns a
// response; failures carry Error with a well-known code and are counted and
// reported (P5). A panicking handler is recovered — the runtime survives.
func (d *Dispatcher) Dispatch(ctx context.Context, req Request) Response {
	start := time.Now()
	resp, cause := d.resolve(ctx, req)
	d.mu.RLock()
	rec, rep := d.metrics, d.reporter
	d.mu.RUnlock()
	if rec != nil {
		rec.RecordIPC(time.Since(start), resp.Error != nil)
	}
	if resp.Error != nil && rep != nil {
		rep.Report(observe.Diagnostic{
			Component: "ipc",
			Message:   fmt.Sprintf("%s: code %d: %s", methodLabel(req), resp.Error.Code, resp.Error.Message),
			Err:       cause,
		})
	}
	return resp
}

// resolve performs the dispatch and returns the response plus the cause for
// diagnostics (nil on success).
func (d *Dispatcher) resolve(ctx context.Context, req Request) (Response, error) {
	resp := Response{Version: Version, ID: req.ID}
	d.mu.RLock()
	grants, timeout := d.grants, d.timeout
	e, found := d.handlers[req.Method]
	d.mu.RUnlock()

	// Envelope validation (protocol/ipc.schema.json): version must match,
	// method must be non-empty, params must be object/array/null when set.
	if req.Version != Version {
		return resp.fail(CodeInvalidRequest, fmt.Sprintf("unsupported ipc version %d (expected %d)", req.Version, Version), nil)
	}
	if req.Method == "" {
		return resp.fail(CodeInvalidRequest, "empty method name", nil)
	}
	if len(req.Params) > 0 && !validParams(req.Params) {
		return resp.fail(CodeInvalidParams, "params must be a JSON object, array, or null", nil)
	}
	if !found {
		return resp.fail(CodeMethodNotFound, fmt.Sprintf("unknown method %q", req.Method), nil)
	}
	// Permission gate: absent grant means denial (G-SEC-02). Reserved for
	// non-native methods skip the gate only when no permission is declared.
	if e.perm != "" && !grants.Allows(e.perm) {
		return resp.fail(CodePermissionDenied, fmt.Sprintf("method %q requires permission %q", req.Method, e.perm), nil)
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type outcome struct {
		result json.RawMessage
		err    error
	}
	if e.inline {
		res, err := runInline(e.handler, callCtx, req.Params)
		if err != nil {
			return resp.fail(CodeFor(err), err.Error(), err)
		}
		resp.Result = res
		if len(resp.Result) == 0 {
			resp.Result = json.RawMessage("null")
		}
		return resp, nil
	}
	// Buffered: a timed-out handler that finishes later never blocks on send.
	ch := make(chan outcome, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- outcome{err: fmt.Errorf("ipc: handler %q panicked: %v", req.Method, r)}
			}
		}()
		res, err := e.handler(callCtx, req.Params)
		ch <- outcome{result: res, err: err}
	}()

	select {
	case out := <-ch:
		if out.err != nil {
			return resp.fail(CodeFor(out.err), out.err.Error(), out.err)
		}
		resp.Result = out.result
		if len(resp.Result) == 0 {
			resp.Result = json.RawMessage("null") // schema: exactly one of result/error
		}
		return resp, nil
	case <-callCtx.Done():
		cause := fmt.Errorf("ipc: method %q exceeded deadline: %w", req.Method, callCtx.Err())
		return resp.fail(CodeTimeout, fmt.Sprintf("method %q exceeded %s", req.Method, timeout), cause)
	}
}

// fail attaches a deterministic error to the response and returns the
// diagnostic cause.
func (r Response) fail(code int, message string, cause error) (Response, error) {
	r.Error = &Error{Code: code, Message: message}
	if cause == nil {
		cause = NewCodeError(code, message)
	}
	return r, cause
}

// runInline executes a handler on the caller's goroutine, recovering a
// panic into an error so a UI mutation never takes down the runtime
// (G-REL-02).
func runInline(h Handler, ctx context.Context, params json.RawMessage) (res json.RawMessage, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ipc: inline handler panicked: %v", r)
		}
	}()
	return h(ctx, params)
}

// CodeFor maps a handler error to its deterministic wire code. Handlers
// choose a code by returning a *CodeError; anything context-shaped maps to
// CodeTimeout; unknown-method and not-implemented from the api registry map
// to their own codes; the rest are internal failures (G-REL-02: never a
// silent success, never a hang).
func CodeFor(err error) int {
	var ce *CodeError
	if errors.As(err, &ce) {
		return ce.Code
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return CodeTimeout
	case errors.Is(err, api.ErrNotFound):
		return CodeMethodNotFound
	case errors.Is(err, api.ErrNotImplemented):
		return CodeInternal
	default:
		return CodeInternal
	}
}

// validParams mirrors the schema constraint: params is optional, but when
// present it must be a JSON object, array, or null.
func validParams(p json.RawMessage) bool {
	if !json.Valid(p) {
		return false
	}
	trim := bytes.TrimSpace(p)
	if len(trim) == 0 {
		return false
	}
	if trim[0] == '{' || trim[0] == '[' {
		return true
	}
	return string(trim) == "null"
}

// methodLabel names the request for diagnostics without leaking params.
func methodLabel(req Request) string {
	if req.Method == "" {
		return "<empty method>"
	}
	return req.Method
}
