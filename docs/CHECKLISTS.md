# Acceptance Checklists


A checklist item passes only with a test, an experiment record, or a documented observation behind it. A checked box without evidence is classification **E**.

**M1 gate:** the boxes checked below were closed on **2026-10-03** — automated evidence (tests, golden images, experiment A) plus the operator's visual verification of `cmd/gowez-hello` (window opens, text renders, resize works, clean exit). Anything unchecked belongs to a later milestone; Release (M7) boxes are re-verified in full at release time.

**M2 gate:** the boxes checked below were closed on **2026-10-05** — automated evidence (style/layout/ui/app tests, golden `ui.png`, resize→relayout test) plus a smoke run of `cmd/gowez-hello` (window opens, M2 scene paints, SIGTERM → rc 0). Operator visual verification of the live scene: confirmed 2026-10-05 (scene paints; "Apply" label centered after the `align-items` subtree fix — [learnings](../evidence/learnings.md)). Subset spec: [docs/CSS-SUBSET.md](CSS-SUBSET.md).

**M3 gate:** the boxes checked below were closed on **2026-10-06** — automated evidence ([input_test.go](../internal/app/input_test.go), [interaction_test.go](../internal/ui/interaction_test.go), [sdl_test.go](../internal/window/sdl_test.go), [golden ui-state.png](../tests/golden/testdata/ui-state.png)) plus a smoke run of `cmd/gowez-hello`. Event semantics: [docs/EVENTS.md](EVENTS.md).

**M4 gate:** the boxes checked below were closed on **2026-10-07** — automated evidence ([script tests](../internal/script/engine_test.go), [ipc tests](../internal/ipc/dispatcher_test.go), [app tests](../internal/app/input_test.go) incl. `TestSceneJSDrivesStateThroughIPC`, experiments [B](../evidence/experiments/2026-10-07_b_js-handler-exception.md)/[C](../evidence/experiments/2026-10-07_c_unknown-method.md)) plus a smoke run of `cmd/gowez-hello` (alive, SIGTERM → rc 0). Engine decision: [DRR-004](../evidence/records/2026-10-07_js-engine-goja.md). Host contract: [docs/SCRIPT.md](SCRIPT.md).

## Core correctness (M1–M4)

- [x] Application starts from a clean environment — [startup sequence test](../tests/integration/window_test.go), [experiment A](../evidence/experiments/2026-10-03_a_renderer-init-failure.md)
- [x] Native window opens — [tests/integration/window_test.go](../tests/integration/window_test.go)
- [ ] Svelte UI loads
- [x] Text renders — [golden text.png](../tests/golden/testdata/text.png)
- [x] Basic shapes render — [golden rects.png](../tests/golden/testdata/rects.png)
- [x] Button renders — [golden ui.png](../tests/golden/testdata/ui.png) ("Apply" button in the M2 scene), [uiscene.go](../internal/app/uiscene.go)
- [x] Mouse click reaches UI — [input_test.go](../internal/app/input_test.go) (`TestLoopDispatchesInput`, `TestSceneClickUpdatesState`), [interaction_test.go](../internal/ui/interaction_test.go)
- [x] Keyboard input works — [input_test.go](../internal/app/input_test.go) (`TestSceneKeyEchoUpdatesStatus`), [interaction_test.go](../internal/ui/interaction_test.go) (`TestTabTraversalWraps`, `TestEnterSpaceActivatesFocusedButton`)
- [x] UI state can update — [input_test.go](../internal/app/input_test.go) (`TestSceneClickUpdatesState`: click → text change → dirty → relayout), [golden ui-state.png](../tests/golden/testdata/ui-state.png)
- [x] JS exception is observable — [experiment B](../evidence/experiments/2026-10-07_b_js-handler-exception.md), [TestSceneJSHandlerFailureIsObservable](../internal/app/input_test.go), [TestFireHandlerExceptionIsolatedAndCounted](../internal/script/engine_test.go) (`JSExceptions` metric + `Component "script"` diagnostic)
- [x] Application exits cleanly — window close path + SIGINT/SIGTERM → rc 0, [lifecycle.go](../internal/app/lifecycle.go), [integration test](../tests/integration/window_test.go)

## Svelte integration (M5)

- [ ] Svelte component can be compiled
- [ ] Component state can update
- [ ] Event handler works
- [ ] Conditional rendering works
- [ ] List rendering works
- [ ] Basic component composition works
- [ ] Required browser APIs are documented
- [ ] Unsupported Svelte/browser behavior is explicit

## IPC (M4/M6)

- [x] UI can invoke Go API — [TestSceneJSDrivesStateThroughIPC](../internal/app/input_test.go) (scene click → JS → `gowez.invoke` → dispatcher)
- [x] Go can return success — same test: `app.getInfo` payload rendered into the status line; [TestDispatchSuccess](../internal/ipc/dispatcher_test.go)
- [x] Go can return error — [experiment C](../evidence/experiments/2026-10-07_c_unknown-method.md), [TestDispatchUsesHandlerCodeError](../internal/ipc/dispatcher_test.go), [TestInvokePermissionDeniedThrowsWithCode](../internal/script/bindings_test.go)
- [x] Unknown method fails deterministically — [experiment C](../evidence/experiments/2026-10-07_c_unknown-method.md) (`-32601`, both Go and JS paths, watchdog-bounded), [TestDispatchUnknownMethodIsDeterministic](../internal/ipc/dispatcher_test.go)
- [x] IPC cannot invoke arbitrary native function — JS surface is `gowez.invoke/call/on` only ([TestSandboxSurface](../internal/script/engine_test.go)); methods must be explicitly registered behind permissions ([TestDispatchPermissionGate](../internal/ipc/dispatcher_test.go), [registry tests](../internal/api/registry_test.go))
- [x] IPC lifecycle is bounded — [TestDispatchTimeoutIsBounded](../internal/ipc/dispatcher_test.go), [TestInvokeIsBoundedByHandlerBudget](../internal/script/bindings_test.go), watchdogs in experiments B/C

## Rendering (M1/M2)

- [x] Layout produces deterministic geometry — [layout_test.go](../internal/layout/layout_test.go) (`TestLayoutDeterministic`)
- [x] Renderer receives explicit commands — [renderer_test.go](../internal/render/backend/software/renderer_test.go)
- [x] Text renders correctly — [golden text.png](../tests/golden/testdata/text.png)
- [x] Basic clipping works — [renderer_test.go](../internal/render/backend/software/renderer_test.go)
- [x] Resize triggers relayout — [uiscene_test.go](../internal/app/uiscene_test.go) (`TestUISceneRelayout`)
- [x] Renderer failure is observable — [experiment A](../evidence/experiments/2026-10-03_a_renderer-init-failure.md), `Pixels()` error paths in [renderer_test.go](../internal/render/backend/software/renderer_test.go)

## Security (M4/M6)

- [x] JavaScript cannot access arbitrary Go functions — [TestSandboxSurface](../internal/script/engine_test.go) (globals limited to the documented host surface), [TestOnRejectsInvalidRegistrations](../internal/script/bindings_test.go) (handlers register by explicit name only), [G-SEC-01](GUARDRAILS.md)
- [ ] Native APIs are explicit
- [ ] Sensitive APIs can be permission controlled
- [ ] Production debug interfaces are disabled
- [x] Dependencies are reviewed — goja pinned in [go.mod](../go.mod) with provenance and limits recorded in [DRR-004](../evidence/records/2026-10-07_js-engine-goja.md) (periodic audit stays with G-SEC-05)

## Release (M7)

- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Svelte integration tests pass
- [ ] Renderer tests pass
- [ ] IPC tests pass
- [ ] Failure experiments pass
- [ ] Basic benchmark exists
- [ ] Documentation updated

## MVP definition of done

The MVP counts as complete for **technical validation** when:

- [ ] A Svelte application builds successfully
- [x] No Chromium dependency — [go.mod](../go.mod) carries pure-Go deps only; `CGO_ENABLED=0 go build ./...` passes
- [x] No OS WebView dependency — windowing via purego SDL3, [DRR-001](../evidence/records/2026-10-03_windowing-purego-sdl3.md)
- [x] The Go binary can create a native window — `cmd/gowez-hello`, [integration test](../tests/integration/window_test.go)
- [ ] Basic UI renders through the GPU
- [x] The user can interact with the UI — click/keyboard/state via [input_test.go](../internal/app/input_test.go) + [EVENTS.md](EVENTS.md) semantics; interactive scene in [uiscene.go](../internal/app/uiscene.go)
- [ ] Svelte state produces UI updates
- [x] JS can call explicit Go APIs — [experiment C](../evidence/experiments/2026-10-07_c_unknown-method.md) (JS path asserts `e.code`), [TestInvokeSuccessReturnsResult](../internal/script/bindings_test.go), [TestSceneJSDrivesStateThroughIPC](../internal/app/input_test.go)
- [ ] Go APIs can access at least one native capability
- [ ] Main failure modes can be tested
- [ ] Startup/memory/rendering benchmarks exist
- [ ] Browser/Svelte compatibility limits are documented

Reaching these does **not** mean the framework is production-ready — it only proves the architectural hypothesis is worth continuing.
