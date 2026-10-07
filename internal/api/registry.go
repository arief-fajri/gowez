package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/arief-fajri/gowez/internal/permission"
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

// regEntry is one registered method: its handler plus the permission the
// IPC gate requires before it may run (G-SEC-02). An empty permission marks
// a non-native method that needs no grant (e.g. app.getInfo).
type regEntry struct {
	handler Handler
	perm    permission.Permission
}

// Registry is the single door between IPC and native capabilities.
// Concurrent access is safe: the dispatcher may call Invoke from multiple
// goroutines.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]regEntry
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]regEntry)}
}

// NewDefaultRegistry builds a registry preloaded with the built-in APIs
// (app info, fs, dialog, clipboard, window).
func NewDefaultRegistry() (*Registry, error) {
	r := NewRegistry()
	for _, register := range []func(*Registry) error{
		registerApp,
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

// Register binds a method name to its handler and the permission gate that
// guards it (empty = non-native, no grant required). Duplicate registration
// is an error so a contract conflict fails at startup, not at call time
// (G-IFACE-01).
func (r *Registry) Register(name string, perm permission.Permission, h Handler) error {
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
	r.handlers[name] = regEntry{handler: h, perm: perm}
	return nil
}

// Names lists the registered methods in lexical order — the complete native
// surface the dispatcher may expose (G-SEC-01).
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.handlers))
	for n := range r.handlers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Permission reports the permission the gate requires for a method
// (empty when none). An unknown method reports empty — callers gate on
// registration first, so an unregistered name can never be reached.
func (r *Registry) Permission(name string) permission.Permission {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handlers[name].perm
}

// Invoke calls a registered handler. An unknown name always yields
// ErrNotFound wrapped with the method name — deterministic and observable.
func (r *Registry) Invoke(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error) {
	r.mu.RLock()
	e, ok := r.handlers[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return e.handler(ctx, params)
}
