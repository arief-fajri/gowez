# GoWEZ Development Guide

**Single source of truth for commands and conventions.** If a command lives elsewhere, it belongs here.

Prerequisites: **Go 1.25+**, **Node 22+**, **npm 10+**.

---

## 1. Commands

### Build

```bash
go build ./...          # framework — zero external Go dependencies
```

There is no runnable entrypoint yet (`cmd/` arrives in Milestone 7); the framework is a library whose public surface is the root `gowez` package.

### Test

```bash
go test ./... -count=1  # unit tests, colocated with packages
```

Cross-cutting suites (arrive with their milestones):

```bash
go test ./tests/golden/...       # deterministic layout/render snapshots
go test ./tests/integration/...  # window + render end-to-end
go test ./tests/failure/...      # failure experiments A–E (Module 5.3)
go test ./tests/bench/...        # benchmarks (Module 5.2)
go test ./... -race              # required before release (Module 6)
```

Experiments that depend on unimplemented milestones must `t.Skip` with the milestone name — never silently pass.

### Format & lint

```bash
gofmt -l .              # must print nothing
go vet ./...            # must be clean
```

`golangci-lint` is intentionally not configured yet; it lands when the first real milestone code arrives. Formatting rules: `gofmt` defaults, tab indentation, grouped imports (stdlib first, then module packages).

### JavaScript workspace

```bash
npm install             # workspaces: packages/*, examples/*
npm run typecheck -w @gowez/adapter
```

The npm workspace exists for the Svelte toolchain (Strategy B). The Go side never imports it; the two meet only through `protocol/*.schema.json`.

---

## 2. Milestone → package map

Create and touch only what the current milestone needs. The full tree exists as structure; packages activate milestone by milestone.

| Milestone | Packages |
|---|---|
| **M1** Window + renderer | `internal/app`, `internal/window`, `internal/text`, `internal/render` (+ `backend/software`, then `backend/opengl`), `internal/observe` |
| **M2** UI tree | `internal/ui`, `internal/layout`, `internal/style` |
| **M3** Interaction | `internal/ui` (events, hit testing), `internal/window` (input) |
| **M4** JavaScript | `internal/script`, `internal/ipc`, `protocol/ipc.schema.json` |
| **M5** Svelte | `packages/adapter`, `internal/assets`, `examples/counter`, `protocol/ui-instruction.schema.json` |
| **M6** Native API | `internal/api`, `internal/permission` |
| **M7** Packaging + benchmark | `cmd/`, `tests/bench/`, docs split |

Goja (JS engine) is a Milestone 4 decision — M1–M3 contain no JavaScript.

---

## 3. Code conventions

### Package rules

- **The root `gowez` package is the entire public API.** Applications and examples import only `github.com/volantisfrontend/gowez`.
- **Everything else lives under `internal/`.** Importing `internal/*` from `examples/`, `packages/`, or application code is a hard error (AGENTS.md hard rule 5).
- **Every package has a `doc.go`** stating its responsibility and scope.
- **Platform variance uses build tags**, not runtime switches (`internal/window/darwin.go`, `win32.go`, `linux.go`, `fallback.go`).

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
| Golden | `tests/golden/` | layout/render output must be byte-deterministic |
| Integration | `tests/integration/` | one milestone end-to-end path |
| Failure | `tests/failure/` | experiments A–E; record every run in `evidence/experiments/` |
| Bench | `tests/bench/` | fixed workload; compare against Electron/Tauri baselines (M7) |

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
