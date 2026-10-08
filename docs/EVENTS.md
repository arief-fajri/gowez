# Events & Input (Milestone 3)

This document is the source of truth for event semantics (invariant I6):
**event semantics never change silently** — any change here is a deliberate,
documented decision. Owner packages: `internal/window` (native input),
`internal/ui` (dispatch, interaction state), `internal/app` (wiring).

## Pipeline

```text
OS event queue → Window.Pump() (bounded batch) → app.loop
    → scene.handleInput (window event → tree input method)
        → hit test → interaction state → dispatch → handlers
    → scene dirty → style → layout → paint → present
```

- **Coordinate space:** pointer events are in **logical window pixels** —
  the same space layout produces and hit testing reads. HiDPI scale is a
  paint-time concern only; input never scales.
- **Ordering:** every pumped batch is dispatched **before** the frame is
  drawn, so a state change paints in the same frame (`pump → dispatch →
  draw`).
- `CloseEvent` ends the loop; `ResizeEvent` needs no handling because the
  loop re-reads `Window.Size()` every frame.

## Event kinds (`internal/ui`)

| Kind | Produced by | Target |
|---|---|---|
| `PointerMove` | pointer motion | node under the pointer (nil hit → dropped) |
| `PointerDown` | primary/secondary button press | node under the pointer |
| `PointerUp` | button release | node under the pointer |
| `Click` | synthesized press+release (below) | press target |
| `KeyDown` | key press (incl. OS key repeat) | focused node |
| `KeyUp` | key release | focused node |

`Event` carries `Target` (fixed), `CurrentTarget` (advances while
bubbling), `Key`, `X/Y`, `Button`, `Modifier`, plus `StopPropagation()`.

## Dispatch

1. Listeners run on the **target first, then each ancestor** (bubbling)
   up to the root; on one node they run in **registration order**.
2. `StopPropagation()` stops the bubble; listeners already queued on the
   same node still run.
3. An event with no matching listener is **dropped, not guessed at** —
   zero dispatches is normal, never an error (I6).
4. The registry is snapshotted per node, so a handler may add/remove
   listeners mid-dispatch.

Registry API: `Tree.AddEventListener(node, kind, handler) → ListenerID`,
`Tree.RemoveEventListener(node, id)`. The ID is the runtime-side handle;
the M5 adapter maps the protocol's `handlerId` onto it. M4 wires the
equivalent by hand — named JS handlers (`gowez.on`) bound to listeners
([SCRIPT.md](SCRIPT.md)).

## Click synthesis

A `Click` fires only when **all** hold:

- press and release land on the **same node** (a drag-away release fires
  `PointerUp` only),
- both used the **primary button** (`ButtonLeft = 1`),
- a press edge comes only from a button-down event — motion never
  counts, even while a button is held.

## Interaction state and `:hover` / `:active` / `:focus`

The tree tracks three state bits on nodes (`ui.Node.State`), mirrored
into pseudo-class matching by `internal/style`:

| Bit | Set on | Cleared |
|---|---|---|
| `StateHovered` | node under the pointer **and its ancestors** | pointer moves to another node/outside |
| `StatePressed` | press target **and its ancestors** | button release (anywhere) |
| `StateFocused` | the single focused node only | focus moves / click outside / node detached |

- Hover/press propagate to **ancestors** — matching browser behavior, so
  `panel:hover` works while a child is hovered.
- Detaching a subtree clears any interaction state inside it (I1: no
  state may reference nodes outside the tree).
- Any state transition returns `Changed=true`; the caller marks the
  scene dirty so the next frame re-runs style + layout. Transitions are
  exact: motion over the same node does not restyle.

**Guidance:** keep pseudo-class rules **color-only**. Changing geometry
from `:hover` moves the boxes under the pointer and can cause hover
flapping (browsers share this hazard).

## Focus model (minimal)

- **Focusable** = element with a non-negative integer `tabindex`
  attribute. A non-integer value is *not focusable* — an explicit
  divergence from browsers (below).
- **Click (primary button):** focus moves to the nearest focusable
  ancestor of the hit node (self included); a hit outside any focusable
  (including no hit) **clears** focus.
- **Tab / Shift+Tab:** traversal through focusables in document order
  with wrap in both directions. Tab is **consumed** — it never
  dispatches as a `KeyDown`.
- **Enter / Space** on a focused `<button>` fires `Click` after the
  `KeyDown` (keyboard activation).
- **Keys without focus are dropped** (zero dispatches, counted as
  unhandled in metrics). `KeyUp` for Tab *is* delivered (only `KeyDown`
  is consumed).

## Key naming and modifiers (`internal/window`)

Portable names (lowercase kebab): `enter`, `escape`, `backspace`, `tab`,
`space`, `delete`, `insert`, `home`, `end`, `page-up`, `page-down`,
`arrow-left/right/up/down`, `caps-lock`, `num-lock`, `print-screen`,
`scroll-lock`, `pause`, `shift`, `control`, `alt`, `meta`, `menu`,
`f1`–`f12`, and printable ASCII lowercased (`a`, `1`, …). Keypad Enter
maps to `enter`. **Anything else is reported as `#<keycode>`** — a key
is never dropped silently.

`Modifier` is a portable bitfield `ModShift|ModCtrl|ModAlt|ModMeta`
(never a raw platform mask). `KeyEvent.Repeat` marks OS key repeat;
repeat arrives as repeated `KeyDown` with no matching extra `KeyUp`.

## Handler contract

- Handlers run **inline on the UI goroutine**. They **must not block**
  (G-REL-01) — a blocked handler stalls frames, which the frame-time
  metric exposes.
- A panicking handler is **recovered**: dispatch continues, the fault is
  counted (`observe.Metrics.HandlerPanics`), and a
  `observe.Diagnostic{Component: "ui"}` is reported. The loop and the
  process survive.
- Handlers mutate application/scene state and mark the scene dirty;
  they never call layout or paint directly.
- **M4:** a listener may forward the event to a JavaScript handler
  (`script.Engine.FireHandler`). The JS side follows the same rule —
  inline, never blocking, bounded by the handler budget
  ([SCRIPT.md](SCRIPT.md)) — and a JS throw is isolated and observable
  (G-REL-02), so dispatch continues.
- **The mutation door depends on which UI path is running, and this changed in
  M5.** The M1–M4 demo scene uses `ui.setText` as the script layer's only
  tree-mutation door. A **compiled Svelte bundle** does not: it registers
  exactly one method, `ui.apply`, carrying a whole instruction batch
  ([SVELTE.md §The single mutation door](SVELTE.md#the-single-mutation-door-d-1)).
  `ui.setText` is *not* registered on the bundle path, so one door stays
  auditable (pinned by `TestBundleSceneRegistersOnlyUIApply`; recorded in
  [GUARDRAILS.md §Changelog](GUARDRAILS.md#changelog)).

## Observability (P5)

| Metric | Meaning |
|---|---|
| `InputEvents` | input events fed to the tree |
| `DispatchedEvents` | handler invocations across those events |
| `UnhandledEvents` | events that reached no listener (includes traversal/dropped keys) |
| `HandlerPanics` | recovered handler faults |

## Divergences from browsers (G-UPG-04)

- No `mouseenter`/`mouseleave`/`pointerenter` events — hover is exposed
  through `:hover` state only.
- No pointer capture, no drag-and-drop, no right-click context model
  (non-primary buttons dispatch down/up but never `Click`).
- No focus ring rendering (`outline` does not exist in the subset) —
  authors style `:focus` themselves.
- Non-integer `tabindex` is not focusable (browsers parse it leniently).
- Click requires identical press/release nodes; browsers fire on the
  nearest common ancestor instead.
- No scroll events and no text input (`EVENT_TEXTINPUT`/IME): scroll
  needs a scroll container (absent from the subset); text input/IME is
  deferred to Milestone 5 (decision 2026-10-07 — M4 covered the JS
  engine and IPC, not text entry).

## Text input and IME (Milestone 5)

Two additive event kinds carry text. They are the only events whose payload is
text — every other kind is geometry or a key name.

| Kind | Produced by | Target | Payload |
|---|---|---|---|
| `textinput` | the platform text-input source, active only while `StartTextInput` is in effect | the focused node | `Text` — committed text |
| `textediting` | the platform IME | the focused node | `Text`, `TextStart`, `TextLength` — preedit plus the composition extent |

Rules:

- **The target is focus, not hit testing.** A pointer over an unrelated element
  must not redirect typing into it, so `Tree.TextInput`/`Tree.TextEditing`
  dispatch to the focused node and are separate entry points from
  `PointerDown`. With nothing focused the event is unhandled, not an error (I6).
- **Characters never come from layout.** There is no keycode→character table;
  the platform hands over text, which is the only way composition and
  non-Latin input work at all.
- **Preedit is never committed.** `textediting` renders and is discarded; only
  `textinput` changes a value.
- **The runtime does not write the value.** Committed text reaches the
  application as an ordinary event, and the application decides what the value
  becomes — the same single mutation door (D-1) every other UI change uses.

### Window contract

`window.Window` gains `StartTextInput()` / `StopTextInput()`. The frame loop
starts text input when the focused node is editable and stops it when focus
leaves, so the platform is never asked for text it has no recipient for. A
backend that cannot deliver text reports an explicit error rather than
accepting a start that produces nothing (P4).

### Divergences

- **No caret and no selection rendering.** The composition extent travels with
  the event so a future caret renderer has what it needs; nothing draws it.
- **No IME candidate window.** That is platform UI, outside the render pipeline.
- **Deletion is the application's job.** Backspace arrives as `key-down`; the
  handler rewrites the value and submits ops.
- **Text input follows focus**, so ordinary keys are unaffected: without an
  editable node the runtime never starts text input.

Evidence: [`input_text_test.go`](../internal/app/input_text_test.go),
[`reactivity_test.go`](../internal/app/reactivity_test.go).
