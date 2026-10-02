package ipc

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotImplemented is returned until the dispatcher lands (Milestone 4).
var ErrNotImplemented = errors.New("ipc: dispatcher not implemented yet (Milestone 4)")

// Handler processes one request. Handlers are registered explicitly —
// there is no reflection-based exposure of arbitrary Go functions
// (G-SEC-01). Handlers must respect ctx (G-REL-01: no unbounded blocking).
type Handler func(ctx context.Context, req Request) (Response, error)

// Dispatcher routes requests to registered handlers behind the permission
// gate.
type Dispatcher struct {
	handlers map[string]Handler
}

// NewDispatcher creates an empty dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: make(map[string]Handler)}
}

// Register binds a method name to a handler. Duplicate or empty
// registrations are rejected — a contract mistake must fail loudly
// (guard rail G-IFACE-01).
func (d *Dispatcher) Register(method string, h Handler) error {
	if method == "" {
		return errors.New("ipc: empty method name")
	}
	if h == nil {
		return fmt.Errorf("ipc: nil handler for method %q", method)
	}
	if _, exists := d.handlers[method]; exists {
		return fmt.Errorf("ipc: method %q already registered", method)
	}
	d.handlers[method] = h
	return nil
}

// Dispatch resolves one request.
//
// Milestone 4 implements: permission gate (G-SEC-02) → deadline
// (G-REL-01) → handler → deterministic error for unknown methods
// (Module 2 §2.4). Until then it fails explicitly rather than pretending
// to succeed.
func (d *Dispatcher) Dispatch(ctx context.Context, req Request) (Response, error) {
	_ = ctx
	_ = req
	return Response{}, ErrNotImplemented
}
