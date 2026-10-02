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

The `render.Renderer` interface (`internal/render/renderer.go`) is the stable boundary; backends are interchangeable (G-UPG-03). The software backend proves the contract in CI first; the OpenGL backend follows.

## Svelte strategy

**Strategy B — compiler-oriented UI runtime** is the target: the adapter translates Svelte output into GoWEZ UI instructions ([`protocol/ui-instruction.schema.json`](../protocol/ui-instruction.schema.json)) so the Go runtime carries no browser semantics. Strategy A (DOM-compat layer) is the fallback if feasibility experiments fail (R1).

## Component map

| Component | Package | Contract |
|---|---|---|
| Lifecycle | `internal/app` | startup sequence, state machine |
| Window/input | `internal/window` | `Window` interface, per-OS build tags |
| UI tree | `internal/ui` | nodes, mutation, dispatch, hit test |
| Style subset | `internal/style` | parser → selector → computed style |
| Layout | `internal/layout` | deterministic box/flex geometry |
| Text | `internal/text` | load, shape, fallback |
| Render contract | `internal/render` | `Renderer`, `Command`, `Frame` |
| JS engine | `internal/script` | sandboxed engine + bounded limits (M4) |
| IPC | `internal/ipc` | versioned request/response, bounded dispatch |
| Permission | `internal/permission` | explicit grants, deny by default |
| Native API | `internal/api` | `Registry` — the single door to the OS |
| Assets | `internal/assets` | bundle loader (go:embed at packaging) |
| Observability | `internal/observe` | metrics + diagnostics |
