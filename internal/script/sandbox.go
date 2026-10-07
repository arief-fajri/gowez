package script

import "time"

// Limits bound JS execution (invariant I10, guard rails G-REL-01/G-DEP-05):
// no script may run, block, or allocate forever.
type Limits struct {
	// EvalTimeout caps a single Eval call.
	EvalTimeout time.Duration
	// HandlerTimeout caps one gowez.on handler invocation (FireHandler).
	// Kept tighter than EvalTimeout: handlers run on the UI goroutine, so
	// their budget is a frame-stall budget (Module 5, tune by measurement).
	HandlerTimeout time.Duration
	// MemoryBytes would cap engine memory. The goja engine provides no
	// memory limit, so New rejects a non-zero value with ErrUnsupported —
	// an unenforceable limit is never silently ignored (P4, docs/SCRIPT.md).
	MemoryBytes uint64
}

// DefaultLimits is the MVP starting point; tune only with measurements
// (Module 5).
var DefaultLimits = Limits{
	EvalTimeout:    2 * time.Second,
	HandlerTimeout: 100 * time.Millisecond,
}

// withDefaults fills zero values from DefaultLimits so a caller passing
// Limits{} still gets bounded execution (never unbounded).
func (l Limits) withDefaults() Limits {
	if l.EvalTimeout <= 0 {
		l.EvalTimeout = DefaultLimits.EvalTimeout
	}
	if l.HandlerTimeout <= 0 {
		l.HandlerTimeout = DefaultLimits.HandlerTimeout
	}
	return l
}
