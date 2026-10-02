package script

import "errors"

// ErrNotImplemented is returned until the JS engine lands (Milestone 4).
var ErrNotImplemented = errors.New("script: JS engine not wired yet (Milestone 4)")

// Engine executes JavaScript inside a bounded, observable sandbox.
type Engine interface {
	// Eval runs source under a stable source name for diagnostics.
	// A thrown exception returns an *Error; it never crashes the runtime
	// (guard rail G-REL-02).
	Eval(sourceName, source string) error
	// Close releases engine resources. It must not block indefinitely
	// (invariant I12).
	Close() error
}

// New creates the JS engine (Milestone 4: goja + sandbox wiring).
func New(limits Limits) (Engine, error) {
	_ = limits
	return nil, ErrNotImplemented
}
