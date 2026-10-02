package app

import "errors"

// ErrNotImplemented is returned by entry points whose milestone has not
// landed yet (Milestone 1 for Run).
var ErrNotImplemented = errors.New("app: not implemented yet (Milestone 1)")

// Run executes the full startup sequence (Module 2 §2.3) and blocks until
// shutdown:
//
//	config → window → renderer → JS runtime → UI bundle → UI tree →
//	layout → render → ready
//
// Every step must either succeed or abort with an explicit diagnostic;
// a partially initialized application never reaches ready state.
//
// Milestone 1 implements window + renderer bring-up; the remaining steps
// arrive with their own milestones.
func Run(opts Options) error {
	a, err := New(opts)
	if err != nil {
		return err
	}
	if err := a.transition(StateStarting); err != nil {
		return err
	}
	return ErrNotImplemented
}
