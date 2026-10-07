# AGENTS.md — instructions for AI coding agents working on this repository

GoWEZ is a Go-native desktop application runtime: Svelte as the UI authoring layer, Go as the application runtime, an own render pipeline — **no Chromium and no OS WebView**. Status: research / MVP technical validation. The public behavior contracts are [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and the schemas in [`protocol/`](protocol/). Caller-visible changes must be deliberate, tested, and written there.

**This file is the single entry point for every AI agent session.** Read it first, before any other file.

---

## System thinking methodology

GoWEZ follows a system-thinking engineering loop. **You must use it for every non-trivial task.**

```text
PLATFORM DESIGN → GUARD RAILS → IMPLEMENT → OBSERVE & MEASURE → EVALUATE → RECORD LEARNING
```

Full framework details:

- [docs/PLATFORM.md](docs/PLATFORM.md) — what we are building: principles P1–P5, invariants I1–I12, desired outcomes
- [docs/GUARDRAILS.md](docs/GUARDRAILS.md) — what is never allowed: G-DATA, G-DEP, G-IFACE, G-SEC, G-REL, G-UPG
- [docs/FAILURE-MODES.md](docs/FAILURE-MODES.md) — failure-mode model and classification (classify before you fix)
- [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) — metrics and failure experiments that prove guard rails hold
- [docs/CHECKLISTS.md](docs/CHECKLISTS.md) — acceptance gates (Module 6)

### System thinking preflight (before starting non-trivial work)

Answer these **before writing any code**:

1. What system property are we changing (P1–P5)?
2. Which desired outcome (Module 1 §1.4) does this affect?
3. Which invariants (I1–I12) must remain true?
4. Which guard rails (G-*) apply?
5. How will the behavior be observed (Module 5 metrics)?
6. How will it be tested?
7. Which failure modes (Module 2 §2.4) are relevant?
8. Which acceptance checklist items (Module 6) gate this?
9. What evidence will prove it works?
10. What did we learn after implementation?

The answers go in the PR description — this is the **10-step Definition of Done**.

### Failure classification — classify before you fix

When a test fails or an experiment misbehaves, **classify it before changing code**:

| Type | Pattern | Response |
|---|---|---|
| **A — Implementation failure** | Design ✓, guard rail ✓, implementation ✗ | Fix implementation |
| **B — Design failure** | Implementation ✓, design was insufficient | Update docs/PLATFORM.md |
| **C — Missing guard rail** | System entered a forbidden state | Add a guard rail (docs/GUARDRAILS.md) |
| **D — Missing observability** | System failed but nobody can tell why | Add measurement (docs/OBSERVABILITY.md) |
| **E — Incorrect acceptance criteria** | Checklist passed but behavior was unsafe | Update docs/CHECKLISTS.md |

**You may never jump from a failing test directly to a code change.** Record the classification first (PR description, or an experiment record in [`evidence/experiments/`](evidence/experiments/README.md)).

### Decision authority

Your autonomy is bounded:

| Level | What | Action |
|---|---|---|
| **A — Decide & execute** | Internal, reversible, guard-rail-safe, contract-safe | Execute, record in PR |
| **B — Explicit human confirm** | Everything else: `protocol/` schema changes, root public API, new dependencies, security/permission model, MVP scope (IN/OUT) changes, destructive operations | Open a DRR in [`evidence/records/`](evidence/records/README.md) first. Blocked until a human confirms. Silence is not approval. |

**When in doubt: raise the class, never lower it.** You must not downgrade B to A.

### Evidence and learning

- Failure experiments (A–E) produce records in `evidence/experiments/`.
- Decision Request Records (Level B) go in `evidence/records/`.
- After implementing, append what you learned to [`evidence/learnings.md`](evidence/learnings.md).
- An experiment without a traceable artifact is not evidence.

---

## Read first (per task)

- **Idea validation:** [docs/IDEA-VALIDATION.md](docs/IDEA-VALIDATION.md) — problem, evidence status, risks, decision
- **Architecture:** [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- **Contracts:** [protocol/ui-instruction.schema.json](protocol/ui-instruction.schema.json), [protocol/ipc.schema.json](protocol/ipc.schema.json), [docs/CSS-SUBSET.md](docs/CSS-SUBSET.md), [docs/EVENTS.md](docs/EVENTS.md), [docs/SCRIPT.md](docs/SCRIPT.md)
- **Commands & conventions:** [DEVELOPMENT_GUIDE.md](DEVELOPMENT_GUIDE.md) (single source of truth)
- **Guard rails:** [docs/GUARDRAILS.md](docs/GUARDRAILS.md)
- **Roadmap:** [README.md](README.md#roadmap)

## Build

```bash
go build ./...          # framework compiles (pure-Go deps only, no cgo)
```

Runnable acceptance sample: `go run ./cmd/gowez-hello` (Milestone 1 complete 2026-10-03; Milestone 2 UI scene 2026-10-05; Milestone 3 input/interaction 2026-10-06; Milestone 4 JS engine/IPC 2026-10-07). Packaging-grade entrypoints arrive with Milestone 7. The public surface is the root `gowez` package only.

## Test

```bash
go test ./... -count=1  # unit tests
go test ./... -race     # required before any release-oriented change
go test -tags integration ./tests/integration/...  # real-window checks (OS main thread)
```

Cross-cutting suites arrive with their milestones — see [DEVELOPMENT_GUIDE.md §1](DEVELOPMENT_GUIDE.md#1-commands). Failure experiments live in `tests/failure/` and must record every run in `evidence/experiments/`. Experiments for unimplemented milestones `t.Skip` with the milestone name — never silently pass.

## Lint & format

```bash
gofmt -l .              # must print nothing
go vet ./...            # must be clean
```

`golangci-lint` is not configured yet (deliberate deferral; `gofmt` + `go vet` gate every change).

## Docs

- English, `.md` suffix on all internal links.
- `docs/*` is the canonical documentation — keep it accurate and consistent with `protocol/`.
- Update `README.md` when the public surface, layout, or roadmap changes.

## Code layout

- `gowez.go` — the entire public API (`Config`, `App`, `Run`, `ErrNotImplemented`)
- `internal/app` — lifecycle state machine, startup sequence, config
- `internal/window` — window contract; per-OS backends via build tags (M1 ✓ window; M3 ✓ input)
- `internal/ui` — UI tree, geometry, hit testing, event dispatch + focus (M2–M3 ✓)
- `internal/style` — CSS subset (parser, selectors, resolve; M2 ✓, spec in `docs/CSS-SUBSET.md`)
- `internal/layout` — box + flex layout, deterministic geometry (M2 ✓)
- `internal/paint` — tree + styles + geometry → render commands (M2 ✓)
- `internal/text` — font loading, shaping, fallback
- `internal/render` — backend-agnostic `Renderer` contract + commands
- `internal/render/backend/software` — CPU reference backend (first, for CI/tests)
- `internal/render/backend/opengl` — GPU backend (after software proves the contract)
- `internal/script` — sandboxed JS engine (goja, M4 ✓ — contract in [`docs/SCRIPT.md`](docs/SCRIPT.md); DRR-004)
- `internal/ipc` — versioned, bounded UI ↔ Go dispatch (M4 ✓ incl. inline UI-mutation mode)
- `internal/permission` — explicit grant model (gate wired M4 ✓ deny-by-default; grants M6)
- `internal/api` — built-in native APIs; `Registry` is the single door to the OS (registry + `app.getInfo` M4 ✓; full set M6)
- `internal/assets` — UI bundle loading (go:embed at packaging)
- `internal/observe` — metrics + diagnostics (Module 5)
- `protocol/` — versioned JSON Schemas shared by Go and TypeScript
- `packages/adapter` — Svelte → UI instructions (Strategy B, Milestone 5)
- `examples/counter` — acceptance sample (Milestone 5)
- `tests/{golden,integration,failure,bench}` — cross-cutting suites
- `evidence/{experiments,records,learnings.md}` — traceable artifacts

## Hard rules

1. **Never add Chromium or OS WebView as a dependency.** The whole thesis collapses if the runtime needs a browser (README thesis).
2. **Never expose arbitrary Go functions to JavaScript.** Native capabilities are reachable only through registered handlers in `internal/api/registry.go`, gated by `internal/permission` (G-SEC-01/02).
3. **Never change a `protocol/` schema or the root public API without a version bump, a documented breaking change, and a regression test** (G-IFACE-02/03). Level B: DRR first.
4. **Never let an operation block forever.** IPC, JS eval, native calls, shutdown — everything is bounded (P3, G-REL-01, G-DEP-05).
5. **Never import `internal/*` from applications, `examples/`, or `packages/`.** The root package is the whole public surface.
6. **Never silently misrender unsupported features.** Unsupported CSS/UI or browser APIs fail explicitly and are documented (Module 2 §2.4, G-UPG-04).
7. **Never change a render backend in a way that changes the `render.Renderer` contract or anything above it** (G-UPG-03).
8. **Never change a system property without defining how its correctness will be observed** (P5, docs/OBSERVABILITY.md).
9. **Never jump from a failing test to a code change — classify (A–E) first.**
10. **Every non-trivial change completes the 10-step DoD** with answers in the PR description, and records Level B decisions as DRRs.
