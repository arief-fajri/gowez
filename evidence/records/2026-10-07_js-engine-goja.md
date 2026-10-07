# DRR — JS engine for Milestone 4: goja

- **Date:** 2026-10-07
- **Author:** agent (M4 plan execution)
- **Status:** confirmed
- **Decision class:** B (requires explicit human confirmation — new dependency)

## Context

M4 delivers the embedded JS engine (`internal/script`), event binding, and the JS → Go IPC call path. `DEVELOPMENT_GUIDE.md` states: "Goja (JS engine) is a Milestone 4 decision — M1–M3 contain no JavaScript." Until now `go.mod` carries pure-Go deps only, `CGO_ENABLED=0 go build ./...` passes (checklist evidence), and hard rule 1 forbids any browser dependency.

Requirements for the engine:

- no cgo (keeps the pure-Go build story and CI), no Chromium/WebView (hard rule 1),
- bounded execution: a stoppable lifecycle (I10, G-REL-01/G-DEP-05) — an infinite script must be interruptible,
- isolated failure: a JS exception surfaces as an error, never a process crash (G-REL-02, failure experiment B),
- no arbitrary Go exposure (I7, G-SEC-01/G-DEP-02) — only deliberately registered host functions,
- ES level high enough for Svelte-compiled output in M5 (Promises at minimum).

## Options considered

1. **`github.com/dop251/goja`** — pure-Go ES2023+ interpreter (MIT), no cgo. `Runtime.Interrupt(v)` / `ClearInterrupt()` give an interrupt-based eval timeout (standard pattern: timer goroutine interrupts a runaway loop). `Exception` values carry message + stack for `script.Error`. Promise support incl. `NewPromise()` for host-created promises. Actively maintained; single-goroutine runtime (documented: one `Runtime` per goroutine).
2. **quickjs bindings** (cgo) — faster, built-in memory limit, but requires cgo + a C toolchain on every platform: breaks `CGO_ENABLED=0 go build ./...` and the no-native-toolchain build story.
3. **`github.com/robertkrimen/otto`** — pure Go but effectively unmaintained, ES5.1 only: Svelte-compiled output (M5) targets modern syntax; a dead engine is a maintenance risk (G-DEP-01).

## Recommendation

Option 1 — **`github.com/dop251/goja`**, pinned in `go.mod` (G-DEP-01).

Known limitation to record (fail explicitly, P4): goja has **no built-in memory limit API**. `internal/script.Limits.MemoryBytes` therefore cannot be enforced — `script.New` rejects a non-zero `MemoryBytes` with an explicit error instead of silently ignoring it. Timeout bound via `Interrupt` is enforced; memory bound is documented as unsupported (G-UPG-04 divergence in `docs/SCRIPT.md`).

Dependency review (G-SEC-05): MIT license; pure Go; transitive deps limited to `github.com/dop251/goja` + `github.com/dop251/goja/parser` tree (`regexp2`, `golang.org/x/xerrors` historically — verified in `go.sum` after `go get` and recorded in learnings).

## Impact

- **Contract:** `protocol/` untouched at this step (the `-32600` code description edit is a separate, additive non-breaking change recorded in the GUARDRAILS changelog). Root `gowez` API untouched (M4 stays internal-only — Level A).
- **Guard rails:** G-DEP-01 (pinned version), G-DEP-02 (script sandbox tests prove no implicit native access), G-REL-01/02 (interrupt timeout + failure experiment B), G-SEC-01 (only registered bindings reachable), G-SEC-05 (review above).
- **Security:** engine runs single-goroutine on the UI goroutine; host surface limited to the `gowez` global (`invoke`, `call`, `on`); no `require`/`process`/reflection access — asserted by tests.
- **Scope:** MVP IN unchanged — "JS can call explicit Go APIs" is an MVP DoD criterion; text input/IME stays deferred to M5 (documented scope move, not an MVP IN/OUT change).

## Confirmation

Human approver: **user (human operator) — 2026-10-07**. Confirmed via explicit engine choice in the M4 planning session ("goja (Recommended)") and execution approval ("mulai eksekusi") after this DRR was named as the blocking gate. Approved: option 1 (goja) with the `MemoryBytes` fail-explicit carve-out.
