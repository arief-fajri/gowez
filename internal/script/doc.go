// Package script integrates the embedded JavaScript engine used to run
// application interaction logic (Milestone 4).
//
// The engine is sandboxed: it never receives arbitrary access to the Go
// runtime (invariant I7, guard rail G-SEC-01). Every execution is bounded
// (invariant I10) and every failure is isolated and observable
// (Module 2 §2.4).
//
// Planned engine: goja. It is deliberately not wired yet — M1–M3 carry no
// JavaScript at all (see DEVELOPMENT_GUIDE.md).
package script
