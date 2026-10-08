# JavaScript Script Host (M4)

**The contract between embedded JavaScript and the Go runtime.** This
document is authoritative for what scripts can call, what they receive,
and what is deliberately absent. The M5 Svelte adapter targets this
surface; anything outside it is unsupported (G-UPG-04) and fails
explicitly, never silently.

Engine decision: [DRR-004](../evidence/records/2026-10-07_js-engine-goja.md)
(goja, pinned). Wire shapes: [`protocol/ipc.schema.json`](../protocol/ipc.schema.json).

## Host surface

Exactly three functions on the `gowez` global — asserted by
[`TestSandboxSurface`](../internal/script/engine_test.go) (keys must be
`call,invoke,on`):

```js
gowez.invoke(method, params?)  // synchronous call → result, or throws Error{code, message}
gowez.call(method, params?)    // Promise-returning form of invoke
gowez.on(name, fn)             // register a named handler (one per name)
```

- **`invoke`** dispatches to the IPC dispatcher immediately and returns
  the Go result. On failure it throws an `Error` whose `.code` is the IPC
  code (below) and `.message` the dispatcher message. `catch (e) { e.code }`
  is the documented pattern.
- **`call`** wraps the same dispatch in a microtask, so it returns a
  real Promise that settles with the result or rejects with the same
  `Error`. It exists because `Promise`-based Svelte output (M5) expects it.
- **`on`** stores `fn` under `name` for `script.Engine.FireHandler`.
  Registration is explicit: non-functions, empty names, and duplicate
  names are rejected. There is no way to expose an arbitrary Go function
  — only registered IPC methods are reachable (G-SEC-01/02).

There is no other entry point. The natives `__gowez_invoke` and
`__gowez_on` exist only during bootstrap and are deleted from the global
scope afterwards.

## Error codes

Owned by `protocol/ipc.schema.json`:

| Code | Meaning |
|---|---|
| `-32600` | invalid request — wrong schema version, empty method |
| `-32601` | unknown method (nothing registered under that name) |
| `-32602` | invalid params |
| `-32603` | internal handler failure (incl. recovered handler panic) |
| `-32000` | permission gate denied the call |
| `-32001` | timeout — the call exceeded its deadline |

JS-side, these arrive as `Error.code`; Go-side failures inside a JS
handler surface as `*script.Error` (source + message + engine stack).

## Bounds and threading (P3, G-REL-01)

- The engine is **single-goroutine on the UI goroutine**: `Eval`,
  `FireHandler`, and everything `invoke` does run there. Never call the
  engine from another goroutine.
- **`Eval` budget: 2 s. Handler budget: 100 ms** (tighter because
  handlers run on the UI goroutine). Overruns are interrupted
  (`ErrTimeout`), the engine is cleared, and reuse continues — proven by
  [`TestEvalTimeoutIsBoundedAndReusable`](../internal/script/engine_test.go)
  and [`TestSpikeInterruptBoundsInfiniteLoop`](../internal/script/spike_test.go).
- Each `invoke` is bounded again by the dispatcher's deadline (default
  2 s, caller deadline wins) and the JS call's remaining budget.
- **Memory is not enforceable:** goja has no memory-limit API.
  `script.New` rejects `Limits.MemoryBytes != 0` with an explicit error
  instead of silently ignoring it (P4, [DRR-004](../evidence/records/2026-10-07_js-engine-goja.md)).

## Binding to UI events

M4 wires listeners by hand in the scene
([`internal/app/uiscene.go`](../internal/app/uiscene.go)):

1. Go listener on a tree node (`ui.Click`, `ui.KeyDown`, …),
2. listener calls `Engine.FireHandler(name, payload)` with a JSON
   payload (`{"type":"click"}`, `{"type":"key-down","key":…}`),
3. the JS handler mutates state and writes UI text through
   `ui.setText` — the script layer's **only** tree-mutation door
   (inline on the UI goroutine, params `{nodeId, text}`, unknown ids
   rejected with `-32602`).

`FireHandler` failures (throw, timeout) are isolated: `JSExceptions` is
counted and a `Diagnostic{Component:"script"}` is reported; dispatch
continues (G-REL-02, [experiment B](../evidence/experiments/2026-10-07_b_js-handler-exception.md)).

Globals persist across `Eval` calls on the same engine, so scripts can
declare state once (`var sceneState = …`) and handlers share it.

## What is deliberately absent (G-UPG-04)

- **No DOM** — no `document`, `window`, `Element`, layout, or CSSOM.
  The UI tree is manipulated only through registered IPC methods.
- **No timers, no `console`, no `fetch`, no modules** — no
  `setTimeout`/`setInterval` (a timer would outlive the UI-goroutine
  budget with nothing to schedule into), no `require`/`import`,
  no `process`/`Go` bindings. Unsupported access throws a normal
  `ReferenceError` — explicit, not silently stubbed.
- **No memory limit API** (above). **No second thread / Worker.**
- **Text input and IME *are* part of the host since M5** — the runtime starts
  platform text input when an editable node gains focus and stops it on blur.
  Committed text arrives as a `textinput` event carrying text (never a
  keycode); IME composition arrives as `textediting`. Deletion is the
  application's job: Backspace arrives as `key-down`
  ([EVENTS.md](EVENTS.md), [SVELTE.md](SVELTE.md)).

## Observability (P5)

| Metric / signal | Source |
|---|---|
| `JSExceptions` | failed `Eval` / `FireHandler` (missing-handler misses excluded) |
| `LastJSEvalDuration` | every `Eval`, always recorded |
| `IPCCount`, `IPCErrorCount`, `LastIPCDuration` | every dispatcher round trip |
| `UIOpBatches`, `UIOpsApplied`, `UIOpsRejected` | every `ui.apply` (M5) |
| `Diagnostic{Component:"script"}` | JS failure with source + stack |

## Evidence

- Engine + sandbox: [`internal/script/engine_test.go`](../internal/script/engine_test.go),
  [`bindings_test.go`](../internal/script/bindings_test.go),
  [`spike_test.go`](../internal/script/spike_test.go) (goja assumptions).
- Dispatch: [`internal/ipc/dispatcher_test.go`](../internal/ipc/dispatcher_test.go)
  (deadlines, permission gate, inline mode, `-32601`).
- End-to-end scene path: [`TestSceneJSDrivesStateThroughIPC`](../internal/app/input_test.go).
- Failure experiments: [B](../evidence/experiments/2026-10-07_b_js-handler-exception.md),
  [C](../evidence/experiments/2026-10-07_c_unknown-method.md).
