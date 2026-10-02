package script

import "time"

// Limits bound JS execution (invariant I10, guard rails G-REL-01/G-DEP-05):
// no script may run, block, or allocate forever.
type Limits struct {
	// EvalTimeout caps a single Eval call.
	EvalTimeout time.Duration
	// MemoryBytes caps engine memory where the engine supports it.
	MemoryBytes uint64
}

// DefaultLimits is the MVP starting point; tune only with measurements
// (Module 5).
var DefaultLimits = Limits{
	EvalTimeout: 2 * time.Second,
}
