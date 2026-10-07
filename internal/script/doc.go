// Package script integrates the embedded JavaScript engine used to run
// application interaction logic (Milestone 4).
//
// The engine is sandboxed: it never receives arbitrary access to the Go
// runtime (invariant I7, guard rail G-SEC-01). Every execution is bounded
// (invariant I10) and every failure is isolated and observable
// (Module 2 §2.4).
//
// Engine: goja, pinned in go.mod (decision: evidence/records
// 2026-10-07_js-engine-goja.md, DRR-004). The host surface is the
// `gowez` global only — invoke/call/on — and is documented as a contract
// in docs/SCRIPT.md. Execution budgets and the single-goroutine rule
// (UI goroutine) live there too.
package script
