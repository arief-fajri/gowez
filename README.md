# GoWEZ

**A Go-native desktop application runtime: Svelte as the UI authoring layer, Go as the application runtime, an own render pipeline — no Chromium, no OS WebView.**

> **Status:** research / MVP technical validation. Not production-ready.
> Nothing is runnable yet; see [Roadmap](#roadmap) for the milestone plan.

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
├── cmd/                  # CLI — Milestone 7
├── internal/             # runtime (not importable by applications)
│   ├── app/              # lifecycle, startup sequence, config
│   ├── window/           # native window + input (per-OS build tags)
│   ├── ui/               # UI tree, event dispatch, hit testing
│   ├── style/            # CSS subset
│   ├── layout/           # box + flex layout
│   ├── text/             # fonts, shaping, fallback
│   ├── render/           # backend-agnostic command contract
│   │   └── backend/{software,opengl}/
│   ├── script/           # embedded JS engine (sandboxed; Milestone 4)
│   ├── ipc/              # UI ↔ Go dispatch (versioned, bounded)
│   ├── permission/       # explicit grant model
│   ├── api/              # built-in native APIs — the single door to the OS
│   ├── assets/           # UI bundle loading
│   └── observe/          # metrics + diagnostics
├── protocol/             # JSON Schemas: UI instructions + IPC contracts
├── packages/adapter/     # Svelte → UI instructions (TypeScript)
├── examples/counter/     # acceptance sample app (Milestone 5)
├── tests/{golden,integration,failure,bench}/
├── evidence/{experiments,records,learnings.md}
└── docs/                 # documentation (platform, guard rails, checklists)
```

## Roadmap

| Milestone | Delivers |
|---|---|
| **M1** | Native window → software renderer → shapes/text → resize |
| **M2** | UI tree, style subset, layout, hit testing |
| **M3** | Button, mouse/keyboard events, state updates |
| **M4** | Embedded JS engine, event binding, JS → Go API |
| **M5** | Svelte compile pipeline, counter example, state updates |
| **M6** | Native APIs: fs, dialog, clipboard, window control |
| **M7** | Packaging, benchmarks vs Electron/Tauri, sample application |

MVP success = the 12 criteria in [`docs/CHECKLISTS.md`](docs/CHECKLISTS.md). Reaching them proves the architectural hypothesis — **not** production readiness.

## Documentation

| Document | Purpose |
|---|---|
| [`DEVELOPMENT_GUIDE.md`](DEVELOPMENT_GUIDE.md) | Commands, conventions, milestone → package map |
| [`AGENTS.md`](AGENTS.md) | Working agreement for AI coding agents — read first |
| [`docs/`](docs/) | Platform, guard rails, failure modes, observability, checklists |
| [`protocol/`](protocol/) | Versioned UI-instruction and IPC contracts |

## Contributing

Read [`AGENTS.md`](AGENTS.md) first — it defines the system-thinking loop, the failure classification, decision authority, and hard rules that every change must follow.

Quick start:

```bash
go build ./...      # framework compiles with zero external dependencies
gofmt -l .          # must print nothing
go vet ./...
```

See [`DEVELOPMENT_GUIDE.md`](DEVELOPMENT_GUIDE.md) for the full command set.
