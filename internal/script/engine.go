package script

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dop251/goja"

	"github.com/arief-fajri/gowez/internal/ipc"
	"github.com/arief-fajri/gowez/internal/observe"
)

// Sentinel errors. Every failure a caller may need to branch on has a
// package-level error; nothing is identified by message matching.
var (
	// ErrTimeout is returned when an Eval or handler call exceeds its
	// limit (invariant I10: JS execution has a stoppable lifecycle,
	// G-REL-01/G-DEP-05: never runs forever).
	ErrTimeout = errors.New("script: execution exceeded its timeout")

	// ErrUnsupported is returned for a requested limit the engine cannot
	// enforce. Fail explicitly rather than pretend — MemoryBytes has no
	// engine support (P4, docs/SCRIPT.md).
	ErrUnsupported = errors.New("script: limit not supported by the engine")

	// ErrHandlerNotFound is returned by FireHandler when no JS handler is
	// registered under the name — a wiring miss, not a JS exception, so it
	// never counts toward Metrics.JSExceptions.
	ErrHandlerNotFound = errors.New("script: handler not registered")

	// ErrClosed is returned by operations on a closed engine.
	ErrClosed = errors.New("script: engine is closed")
)

// Engine executes JavaScript inside a bounded, observable sandbox.
//
// Threading contract: an engine instance is not safe for concurrent use —
// like the goja Runtime it owns, all calls happen on one goroutine (the UI
// goroutine). Host handlers invoked from JS must not re-enter the engine.
type Engine interface {
	// Eval runs source under a stable source name for diagnostics. A thrown
	// exception returns an *Error; a timeout returns an error satisfying
	// errors.Is(err, ErrTimeout). Either way the runtime survives and stays
	// reusable (guard rail G-REL-02).
	Eval(sourceName, source string) error

	// FireHandler invokes a handler registered from JS via gowez.on,
	// passing payload as the single JSON-shaped argument. The call is
	// bounded by Limits.HandlerTimeout.
	FireHandler(name string, payload json.RawMessage) error

	// HasHandler reports whether JS registered a handler under name.
	HasHandler(name string) bool

	// SetObservation wires the metrics recorder and diagnostics reporter
	// (P5): every evaluation records its duration; failures count as JS
	// exceptions and produce a Diagnostic{Component:"script"}.
	SetObservation(rec *observe.Recorder, rep observe.Reporter)

	// Close releases engine resources without blocking (invariant I12).
	// It is idempotent; later operations return ErrClosed.
	Close() error
}

// engine is the goja-backed implementation.
type engine struct {
	vm         *goja.Runtime
	limits     Limits
	dispatcher *ipc.Dispatcher
	handlers   map[string]goja.Callable
	metrics    *observe.Recorder
	reporter   observe.Reporter

	// curCtx is the deadline-carrying context of the JS call currently
	// running (Eval or FireHandler). gowez.invoke dispatches under it, so
	// an IPC call from inside a handler inherits the handler's budget.
	// Only ever touched on the engine goroutine.
	curCtx context.Context

	// callID correlates in-process invoke requests in diagnostics.
	callID uint64
	closed bool
}

// New creates the JS engine and installs the host surface (gowez.invoke,
// gowez.call, gowez.on). A nil dispatcher is allowed: invoke then fails
// with an explicit error, which keeps the engine usable for pure-script
// tests.
func New(limits Limits, d *ipc.Dispatcher) (Engine, error) {
	limits = limits.withDefaults()
	if limits.MemoryBytes != 0 {
		return nil, fmt.Errorf("script: MemoryBytes=%d: %w: the goja engine has no memory limit; remove the limit or see docs/SCRIPT.md", limits.MemoryBytes, ErrUnsupported)
	}
	e := &engine{
		vm:         goja.New(),
		limits:     limits,
		dispatcher: d,
		handlers:   make(map[string]goja.Callable),
	}
	if err := e.installHost(); err != nil {
		return nil, fmt.Errorf("script: host bindings: %w", err)
	}
	return e, nil
}

// Eval implements Engine.
func (e *engine) Eval(sourceName, source string) error {
	if e.closed {
		return ErrClosed
	}
	if sourceName == "" {
		sourceName = "<eval>"
	}
	start := time.Now()
	_, err := e.bounded(sourceName, e.limits.EvalTimeout, func() (goja.Value, error) {
		return e.vm.RunScript(sourceName, source)
	})
	e.observe(sourceName, start, err)
	return err
}

// FireHandler implements Engine.
func (e *engine) FireHandler(name string, payload json.RawMessage) error {
	if e.closed {
		return ErrClosed
	}
	sourceName := "handler:" + name
	start := time.Now()
	fn, ok := e.handlers[name]
	if !ok {
		// A wiring miss is not a JS exception: observed and reported, but
		// never counted toward Metrics.JSExceptions.
		err := fmt.Errorf("%w: %q", ErrHandlerNotFound, name)
		e.observe(sourceName, start, err)
		return err
	}
	arg := goja.Undefined()
	if len(payload) > 0 {
		var v any
		if err := json.Unmarshal(payload, &v); err == nil {
			arg = e.vm.ToValue(v)
		}
	}
	_, err := e.bounded(sourceName, e.limits.HandlerTimeout, func() (goja.Value, error) {
		return fn(goja.Undefined(), arg)
	})
	e.observe(sourceName, start, err)
	return err
}

// HasHandler implements Engine.
func (e *engine) HasHandler(name string) bool {
	_, ok := e.handlers[name]
	return ok
}

// SetObservation implements Engine.
func (e *engine) SetObservation(rec *observe.Recorder, rep observe.Reporter) {
	e.metrics, e.reporter = rec, rep
}

// Close implements Engine.
func (e *engine) Close() error {
	if e.closed {
		return nil
	}
	e.closed = true
	e.handlers = nil
	e.curCtx = nil
	e.vm.ClearInterrupt()
	return nil
}

// bounded runs fn under a deadline: a timer interrupts runaway JS
// (goja Interrupt), the interrupt flag is always cleared afterward, and the
// caller's context deadline (when shorter) wins via context.WithTimeout.
// The done-flag under mu closes the documented goja race where a timer
// fires between fn returning and ClearInterrupt, which would otherwise
// poison the next call.
func (e *engine) bounded(sourceName string, limit time.Duration, fn func() (goja.Value, error)) (goja.Value, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()

	prev := e.curCtx
	e.curCtx = ctx
	defer func() { e.curCtx = prev }()

	var mu sync.Mutex
	done := false
	timer := time.AfterFunc(limit, func() {
		mu.Lock()
		defer mu.Unlock()
		if !done {
			e.vm.Interrupt(errTimeout{})
		}
	})

	v, err := fn()

	mu.Lock()
	done = true
	mu.Unlock()
	timer.Stop()
	e.vm.ClearInterrupt()
	return v, e.convert(sourceName, err, limit)
}

// convert maps engine-level failures onto the package contract: a thrown
// JS exception becomes *Error (source, message, stack — isolated, never a
// crash), a timeout becomes ErrTimeout, anything else is passed through
// wrapped with the source name.
func (e *engine) convert(sourceName string, err error, limit time.Duration) error {
	if err == nil {
		return nil
	}
	var ie *goja.InterruptedError
	if errors.As(err, &ie) {
		if _, isTimeout := ie.Value().(errTimeout); isTimeout {
			return fmt.Errorf("%s: %w (limit %s)", sourceName, ErrTimeout, limit)
		}
		return fmt.Errorf("%s: interrupted: %v", sourceName, ie.Value())
	}
	var ex *goja.Exception
	if errors.As(err, &ex) {
		return newExceptionError(sourceName, ex)
	}
	return fmt.Errorf("%s: %w", sourceName, err)
}

// errTimeout is the value passed to Interrupt; a timeout is distinguishable
// from any other interrupt by type.
type errTimeout struct{}

// observe records the outcome of one JS call (P5): duration always,
// JSExceptions + Diagnostic on failure. Missing-handler misses are wiring
// faults, not JS exceptions, and are reported without counting.
func (e *engine) observe(sourceName string, start time.Time, err error) {
	if e.metrics != nil {
		e.metrics.RecordJSEval(time.Since(start))
	}
	if err == nil {
		return
	}
	if e.metrics != nil && !errors.Is(err, ErrHandlerNotFound) {
		e.metrics.RecordJSException()
	}
	if e.reporter != nil {
		e.reporter.Report(observe.Diagnostic{
			Component: "script",
			Message:   fmt.Sprintf("%s failed", sourceName),
			Err:       err,
		})
	}
}
