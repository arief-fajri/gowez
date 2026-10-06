# GoWEZ Development Guide

**Single source of truth for commands and conventions.** If a command lives elsewhere, it belongs here.

Prerequisites: **Go 1.25+**, **Node 22+**, **npm 10+**.

---

## 1. Commands

### Build

```bash
go build ./...          # framework (pure-Go deps only, no cgo)
go run ./cmd/gowez-hello  # Milestone 1 acceptance sample (opens a window)
```

`cmd/gowez-hello` is the runnable entrypoint; packaging-grade CLIs arrive in Milestone 7. The framework is a library whose public surface is the root `gowez` package.

### Test

```bash
go test ./... -count=1  # unit tests, colocated with packages
```

Cross-cutting suites (arrive with their milestones):

```bash
go test ./tests/golden/...       # deterministic layout/render snapshots (-update to regenerate)
go test -tags integration ./tests/integration/...  # real window + present (OS main thread, TestMain)
go test ./tests/failure/...      # failure experiments A–E (Module 5.3)
go test ./... -race              # required before release (Module 6)
go test ./... -run '^$' -bench . # package-level micro-benchmarks (text, raster)
```

Integration tests need `-tags integration` because they open a real window; they run the window lifecycle in `TestMain` on the OS main thread (SDL/Cocoa requirement — test functions run on worker goroutines).

Experiments that depend on unimplemented milestones must `t.Skip` with the milestone name — never silently pass.

### Format & lint

```bash
gofmt -l .              # must print nothing
go vet ./...            # must be clean
```

`golangci-lint` is intentionally not configured yet (deliberate deferral; `gofmt` + `go vet` gate every change). Formatting rules: `gofmt` defaults, tab indentation, grouped imports (stdlib first, then module packages).

### JavaScript workspace

```bash
npm install             # workspaces: packages/*, examples/*
npm run typecheck -w @gowez/adapter
```

The npm workspace exists for the Svelte toolchain (Strategy B). The Go side never imports it; the two meet only through `protocol/*.schema.json`.

---

## 2. Milestone → package map

Create and touch only what the current milestone needs. The full tree exists as structure; packages activate milestone by milestone.

| Milestone | Packages | Status |
|---|---|---|
| **M1** Window + renderer | `internal/app`, `internal/window`, `internal/text`, `internal/render` (+ `backend/software`), `internal/observe` | ✅ done 2026-10-03 (OpenGL backend follows as a separate step, after the software backend proves the contract) |
| **M2** UI tree | `internal/ui`, `internal/layout`, `internal/style`, `internal/paint` | ✅ done 2026-10-05 (subset spec: `docs/CSS-SUBSET.md`) |
| **M3** Interaction | `internal/ui` (events, hit testing), `internal/window` (input) | ✅ done 2026-10-06 ([EVENTS](docs/EVENTS.md)) |
| **M4** JavaScript | `internal/script`, `internal/ipc`, `protocol/ipc.schema.json` | planned |
| **M5** Svelte | `packages/adapter`, `internal/assets`, `examples/counter`, `protocol/ui-instruction.schema.json` | planned |
| **M6** Native API | `internal/api`, `internal/permission` | planned |
| **M7** Packaging + benchmark | `cmd/`, `tests/bench/`, docs split | planned |

Goja (JS engine) is a Milestone 4 decision — M1–M3 contain no JavaScript.

---

## 3. Code conventions

### Package rules

- **The root `gowez` package is the entire public API.** Applications and examples import only `github.com/arief-fajri/gowez`.
- **Everything else lives under `internal/`.** Importing `internal/*` from `examples/`, `packages/`, or application code is a hard error (AGENTS.md hard rule 5).
- **Every package has a `doc.go`** stating its responsibility and scope.
- **Platform variance uses build tags**, not runtime switches (`internal/window/sdl.go` for {darwin,linux,windows} × {amd64,arm64}, `fallback.go` for everything else).

### Correctness rules

- **Fail boundedly (P3).** No IPC call, JS eval, native op, or shutdown may block forever. Every wait has a deadline (G-REL-01, G-DEP-05).
- **Fail explicitly (P4).** Reject over silently approximating. Unsupported CSS/UI features error — never misrender (Module 2 §2.4).
- **No partial state (I1, I2).** A failed update leaves the previous consistent state, or nothing.
- **One door to the OS.** All native calls go through `internal/api.Registry` behind `internal/permission` (G-SEC-01/02).
- **Backends are interchangeable (G-UPG-03).** Changing `render/backend/*` must not change `render.Renderer` or anything above it.

### Error handling

- Package-level sentinel errors: `ErrNotImplemented`, `ErrNotFound`, `ErrUnsupported` — return them wrapped with `fmt.Errorf("...: %w", err)`.
- Never panic across a package boundary; handlers return errors.
- Milestone stubs return `ErrNotImplemented` rather than pretending success.

---

## 4. Testing conventions

| Kind | Location | Rule |
|---|---|---|
| Unit | `_test.go` beside the package | test the contract, not the implementation |
| Golden | `tests/golden/` | layout/render output must be byte-deterministic (M1 ✓ — `-update` regenerates) |
| Integration | `tests/integration/` | one milestone end-to-end path; GUI checks live in `TestMain` (OS main thread) |
| Failure | `tests/failure/` | experiments A–E; record every run in `evidence/experiments/` (A executed M1; B–E pending) |
| Bench | `tests/bench/` | fixed workload; compare against Electron/Tauri baselines (M7). M1 micro-benchmarks live in-package — see §1 |

Determinism is an acceptance criterion (Module 6): identical input must produce identical layout geometry, shaped text, and command streams.

---

## 5. Contracts (`protocol/`)

- `ui-instruction.schema.json` — output of the Svelte adapter, input of the Go UI runtime (Strategy B).
- `ipc.schema.json` — the only UI ↔ Go message shapes.

Both carry a `version` field. **Any breaking change bumps the version** (G-IFACE-02/03) and is recorded in `docs/GUARDRAILS.md` changelog section.

---

## 6. Documentation rules

- Docs are written in **English**.
- Internal links keep the `.md` suffix: `[platform](./docs/PLATFORM.md)`.
- `docs/*` is the canonical documentation.
- Update `README.md` when the public surface, layout, or roadmap changes.
- After every non-trivial change: append to `evidence/learnings.md` (AGENTS.md → Evidence and learning).
