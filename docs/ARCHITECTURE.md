# Architecture


## Principle

> Svelte is how developers describe UI; Go is the application runtime; the renderer is an implementation detail; the browser is not a dependency.

## Pipeline

```text
                  Svelte source
                       │ build time
                       ▼
              @gowez/adapter (Strategy B)
                       │ UI instructions
                       ▼
                 script / UI runtime
                  │            │
                  ▼            ▼
               UI tree        IPC ──► permission ──► native Go APIs ──► OS
                  │                         ▲
                  ▼                         │
               layout                       │
                  │                         │
                  ▼                         │
           render commands ──► GPU/software backend ──► native window
```

## Rendering contract

The renderer never receives the DOM/UI tree. It receives commands:

```text
UI tree → style resolution → layout tree → render tree
        → render commands (DrawRect, DrawText, ClipRect, Transform…)
        → backend → native window
```

The `render.Renderer` interface (`internal/render/renderer.go`) is the stable boundary; backends are interchangeable (G-UPG-03). The software backend proves the contract in CI first — proven in M1 (2026-10-03, golden tests + integration present); the OpenGL backend follows.

## Svelte strategy

**Strategy B — compiler-oriented UI runtime** is the target: the adapter translates Svelte output into GoWEZ UI instructions ([`protocol/ui-instruction.schema.json`](../protocol/ui-instruction.schema.json)) so the Go runtime carries no browser semantics. Strategy A (DOM-compat layer) is the fallback if feasibility experiments fail (R1).

## Component map

| Component | Package | Contract |
|---|---|---|
| Lifecycle | `internal/app` | startup sequence, state machine (M1 ✓) |
| Window/input | `internal/window` | `Window` interface, per-OS build tags (M1 ✓ window; M3 ✓ input dispatch) |
| UI tree | `internal/ui` | nodes, mutation, geometry, hit test, event dispatch, focus (M2–M3 ✓) |
| Style subset | `internal/style` | parser → selector → computed style (M2 ✓, [spec](CSS-SUBSET.md)) |
| Layout | `internal/layout` | deterministic box/flex geometry (M2 ✓) |
| Paint | `internal/paint` | tree + styles + geometry → render commands (M2 ✓) |
| Text | `internal/text` | load, shape, fallback (M1 ✓) |
| Render contract | `internal/render` | `Renderer`, `Command`, `Frame` (M1 ✓ contract + software backend) |
| JS engine | `internal/script` | goja, bounded eval/handler budget, `gowez.invoke/call/on` host surface (M4 ✓, [contract](SCRIPT.md)) |
| IPC | `internal/ipc` | versioned request/response, bounded dispatch, inline mode for UI-tree mutation (M4 ✓, [schema](../protocol/ipc.schema.json)) |
| Permission | `internal/permission` | explicit grants, deny by default (gate wired M4 ✓; grant model M6) |
| Native API | `internal/api` | `Registry` — the single door to the OS (registry + `app.getInfo` M4 ✓; full set M6) |
| Assets | `internal/assets` | bundle loader (go:embed at packaging; M5) |
| Observability | `internal/observe` | metrics + diagnostics (M1 ✓ startup/frame; M3 ✓ input metrics; M4 ✓ JS/IPC metrics) |
