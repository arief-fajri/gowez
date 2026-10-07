package observe

import (
	"sync"
	"time"
)

// Metrics are the runtime measurements defined in Module 5 §5.1.
type Metrics struct {
	// StartupDuration is time from Run to ready state.
	StartupDuration time.Duration
	// LayoutCount is the number of completed layout passes.
	LayoutCount uint64
	// LastLayoutDuration is the duration of the most recent layout pass.
	LastLayoutDuration time.Duration
	// FrameCount is the number of completed frames.
	FrameCount uint64
	// DroppedFrames is frames that exceeded the budget.
	DroppedFrames uint64
	// IPCCount is the number of dispatched IPC calls.
	IPCCount uint64
	// IPCErrorCount is the number of failed IPC calls.
	IPCErrorCount uint64
	// LastIPCDuration is the duration of the most recent IPC call.
	LastIPCDuration time.Duration
	// JSExceptions is the number of isolated JS errors.
	JSExceptions uint64
	// LastJSEvalDuration is the duration of the most recent JS evaluation.
	LastJSEvalDuration time.Duration
	// InputEvents is the number of input events fed to the UI tree.
	InputEvents uint64
	// DispatchedEvents is handler invocations across all input events.
	DispatchedEvents uint64
	// UnhandledEvents is input events that reached no listener
	// (includes focus traversal and dropped key events).
	UnhandledEvents uint64
	// HandlerPanics is handler faults recovered by dispatch
	// (docs/EVENTS.md §handler contract).
	HandlerPanics uint64
}

// Recorder is a minimal in-process metrics sink. It is safe for concurrent
// use; wiring to real runtime events happens milestone by milestone.
type Recorder struct {
	mu sync.Mutex
	m  Metrics
}

// NewRecorder creates an empty recorder.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// RecordStartup sets the measured startup duration.
func (r *Recorder) RecordStartup(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m.StartupDuration = d
}

// RecordLayout counts one layout pass and stores its duration.
func (r *Recorder) RecordLayout(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m.LayoutCount++
	r.m.LastLayoutDuration = d
}

// RecordFrame counts one completed frame, flagged as dropped when it
// exceeded budget.
func (r *Recorder) RecordFrame(dropped bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m.FrameCount++
	if dropped {
		r.m.DroppedFrames++
	}
}

// RecordIPC counts one IPC call, stores its duration, and flags failures
// (Module 5 §5.1: IPC invocation count, error count, duration).
func (r *Recorder) RecordIPC(d time.Duration, failed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m.IPCCount++
	r.m.LastIPCDuration = d
	if failed {
		r.m.IPCErrorCount++
	}
}

// RecordJSException counts one isolated JS error (Module 5 §5.1: JS
// execution errors, uncaught exceptions).
func (r *Recorder) RecordJSException() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m.JSExceptions++
}

// RecordJSEval stores the duration of the most recent JS evaluation.
func (r *Recorder) RecordJSEval(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m.LastJSEvalDuration = d
}

// RecordInput counts one input event fed to the UI tree: dispatched is
// the number of handler runs it produced, panics the number of
// recovered handler faults. An event with zero handler runs counts as
// unhandled.
func (r *Recorder) RecordInput(dispatched, panics int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m.InputEvents++
	r.m.DispatchedEvents += uint64(dispatched)
	if dispatched == 0 {
		r.m.UnhandledEvents++
	}
	if panics > 0 {
		r.m.HandlerPanics += uint64(panics)
	}
}

// Snapshot returns a consistent copy of the current metrics.
func (r *Recorder) Snapshot() Metrics {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.m
}
