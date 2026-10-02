package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// ErrNotFound is returned deterministically for unknown methods
// (Module 2 §2.4: unknown handler → deterministic error).
var ErrNotFound = errors.New("api: method not found")

// ErrNotImplemented is returned by built-in handlers until Milestone 6.
var ErrNotImplemented = errors.New("api: not implemented yet (Milestone 6)")

// Handler executes one native operation with the request already
// permission-checked by the caller. Handlers return errors, never panic,
// and must respect ctx (G-REL-01, G-REL-02).
type Handler func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)

// Registry is the single door between IPC and native capabilities.
// Concurrent access is safe: the dispatcher may call Invoke from multiple
// goroutines.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// NewDefaultRegistry builds a registry preloaded with the built-in APIs
// (fs, dialog, clipboard, window).
func NewDefaultRegistry() (*Registry, error) {
	r := NewRegistry()
	for _, register := range []func(*Registry) error{
		registerFS,
		registerDialog,
		registerClipboard,
		registerWindow,
	} {
		if err := register(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Register binds a method name. Duplicate registration is an error so a
// contract conflict fails at startup, not at call time (G-IFACE-01).
func (r *Registry) Register(name string, h Handler) error {
	if name == "" {
		return errors.New("api: empty method name")
	}
	if h == nil {
		return fmt.Errorf("api: nil handler for method %q", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[name]; exists {
		return fmt.Errorf("api: method %q already registered", name)
	}
	r.handlers[name] = h
	return nil
}

// Invoke calls a registered handler. An unknown name always yields
// ErrNotFound wrapped with the method name — deterministic and observable.
func (r *Registry) Invoke(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error) {
	r.mu.RLock()
	h, ok := r.handlers[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return h(ctx, params)
}
