# GoWEZ

**A Go-native desktop application runtime: Svelte as the UI authoring layer, Go as the application runtime, an own render pipeline — no Chromium, no OS WebView.**

> **Status:** research / MVP technical validation. Not production-ready.
> **Milestone 1 complete (2026-10-03):** native window → software renderer → text → resize, with the acceptance sample `cmd/gowez-hello`.
> **Milestone 3 complete (2026-10-06):** mouse/keyboard input, event dispatch with bubbling, `:hover`/`:active`/`:focus` styling, Tab focus model, and state updates — event semantics contract in [docs/EVENTS.md](docs/EVENTS.md), golden `ui-state.png`.
>
> **Milestone 2 complete (2026-10-05):** UI tree → CSS subset → block/flex layout → hit testing → paint, with a live UI scene in `cmd/gowez-hello` and the [CSS subset spec](docs/CSS-SUBSET.md). See the [Roadmap](#roadmap) and the gated boxes in [`docs/CHECKLISTS.md`](docs/CHECKLISTS.md).

## The thesis

Desktop applications can use Svelte as the UI authoring layer and Go as the native application runtime without Chromium and without the OS WebView — as long as the framework restricts browser semantics and provides its own UI, layout, and rendering runtime.

```text
GoWEZ
Svelte → compile → Go UI runtime → layout → render commands → GPU → OS
                     │
                     └── Native Go APIs (explicit, permission-gated)
```

How this differs from the incumbents:

| | UI layer | Runtime | Browser dependency |
|---|---|---|---|
| **Electron** | Svelte/any web UI | Chromium + Node.js | ships Chromium |
| **Tauri** | Svelte/any web UI | OS WebView + Rust | uses OS WebView |
| **GoWEZ** | Svelte (compiled) | Go runtime + own renderer | **none** |

The full argument, evidence status, and risks live in [`docs/IDEA-VALIDATION.md`](docs/IDEA-VALIDATION.md).

## Architecture

```text
             Svelte source
                  │ build time
                  ▼
         @gowez/adapter (Strategy B)
                  │ UI instructions  ← protocol/ui-instruction.schema.json
                  ▼
   ┌──────────────────────────────────────────┐
   │                 GoWEZ                    │
   │                                          │
   │  script (JS) ──► ui tree ──► layout      │
   │       │              │          │        │
   │       └──── ipc ─────┘          ▼        │
   │             │            render commands │
   │             ▼                  │         │
   │      permission gate           ▼         │
   │             │            GPU / software  │
   │             ▼                  │         │
   │      native Go APIs            ▼         │
   │             │            native window   │
   └─────────────┴────────────────────────────┘
```

Key properties:

- **Svelte is compile-time.** It never runs as a browser application.
- **The renderer is an implementation detail.** UI/layout talk to a stable command contract; backends (software, OpenGL, …) are interchangeable.
- **Native access is explicit.** JS reaches Go only through a versioned, permission-gated IPC contract.
- **Failures are bounded and observable.** No operation blocks forever; every startup failure is explicit.

## Repository layout

```text
gowez/
├── gowez.go              # public API surface (the only import apps need)
├── cmd/gowez-hello/      # Milestone 1 acceptance sample (packaging CLIs: M7)
├── internal/             # runtime (not importable by applications)
│   ├── app/              # lifecycle, startup sequence, config (M1 ✓)
│   ├── window/           # native window + input (M1 ✓; input M3 ✓)
│   ├── ui/               # UI tree, geometry, hit testing, events (M2–M3 ✓)
│   ├── style/            # CSS subset (M2 ✓ — docs/CSS-SUBSET.md)
│   ├── layout/           # box + flex layout (M2 ✓)
│   ├── text/             # fonts, shaping, fallback (M1 ✓)
│   ├── paint/            # tree + styles + layout → render commands (M2 ✓)
│   ├── render/           # backend-agnostic command contract (M1 ✓)
│   │   └── backend/{software,opengl}/   # software M1 ✓; opengl after M1
│   ├── script/           # embedded JS engine (sandboxed; M4)
│   ├── ipc/              # UI ↔ Go dispatch (versioned, bounded; M4)
│   ├── permission/       # explicit grant model (M6)
│   ├── api/              # built-in native APIs — the single door to the OS (M6)
│   ├── assets/           # UI bundle loading (M5)
│   └── observe/          # metrics + diagnostics (M1 ✓ startup/frame metrics)
├── protocol/             # JSON Schemas: UI instructions + IPC contracts
├── packages/adapter/     # Svelte → UI instructions (TypeScript; M5)
├── examples/counter/     # acceptance sample app (Milestone 5)
├── tests/{golden,integration,failure,bench}/
├── evidence/{experiments,records,learnings.md}
└── docs/                 # documentation (platform, guard rails, checklists)
```

## Roadmap

| Milestone | Delivers | Status |
|---|---|---|
| **M1** | Native window → software renderer → shapes/text → resize | ✅ done — 2026-10-03 ([checklist](docs/CHECKLISTS.md), [evidence](evidence/learnings.md)) |
| **M2** | UI tree, style subset, layout, hit testing | ✅ done — 2026-10-05 ([checklist](docs/CHECKLISTS.md), [CSS subset](docs/CSS-SUBSET.md), [evidence](evidence/learnings.md)) |
| **M3** | Button, mouse/keyboard events, state updates | ✅ done — 2026-10-06 ([EVENTS](docs/EVENTS.md), [checklist](docs/CHECKLISTS.md)) |
| **M4** | Embedded JS engine, event binding, JS → Go API | planned |
| **M5** | Svelte compile pipeline, counter example, state updates | planned |
| **M6** | Native APIs: fs, dialog, clipboard, window control | planned |
| **M7** | Packaging, benchmarks vs Electron/Tauri, sample application | planned |

MVP success = the 12 criteria in [`docs/CHECKLISTS.md`](docs/CHECKLISTS.md). Reaching them proves the architectural hypothesis — **not** production readiness.

## Documentation

| Document | Purpose |
|---|---|
| [`DEVELOPMENT_GUIDE.md`](DEVELOPMENT_GUIDE.md) | Commands, conventions, milestone → package map |
| [`AGENTS.md`](AGENTS.md) | Working agreement for AI coding agents — read first |
| [`docs/`](docs/) | Platform, guard rails, failure modes, observability, checklists |
| [`protocol/`](protocol/) | Versioned UI-instruction and IPC contracts |
| [`evidence/`](evidence/) | Experiment records, decision records (DRRs), learnings |

## Contributing

Read [`AGENTS.md`](AGENTS.md) first — it defines the system-thinking loop, the failure classification, decision authority, and hard rules that every change must follow.

Quick start:

```bash
go build ./...              # framework compiles (pure-Go deps, no cgo)
go run ./cmd/gowez-hello    # opens a real window (Milestone 1 sample)
gofmt -l .                  # must print nothing
go vet ./...
```

See [`DEVELOPMENT_GUIDE.md`](DEVELOPMENT_GUIDE.md) for the full command set.
