# Acceptance Checklists


A checklist item passes only with a test, an experiment record, or a documented observation behind it. A checked box without evidence is classification **E**.

**M1 gate:** the boxes checked below were closed on **2026-10-03** — automated evidence (tests, golden images, experiment A) plus the operator's visual verification of `cmd/gowez-hello` (window opens, text renders, resize works, clean exit). Anything unchecked belongs to a later milestone; Release (M7) boxes are re-verified in full at release time.

**M2 gate:** the boxes checked below were closed on **2026-10-05** — automated evidence (style/layout/ui/app tests, golden `ui.png`, resize→relayout test) plus a smoke run of `cmd/gowez-hello` (window opens, M2 scene paints, SIGTERM → rc 0). Operator visual verification of the live scene: confirmed 2026-10-05 (scene paints; "Apply" label centered after the `align-items` subtree fix — [learnings](../evidence/learnings.md)). Subset spec: [docs/CSS-SUBSET.md](CSS-SUBSET.md).

**M3 gate:** the boxes checked below were closed on **2026-10-06** — automated evidence ([input_test.go](../internal/app/input_test.go), [interaction_test.go](../internal/ui/interaction_test.go), [sdl_test.go](../internal/window/sdl_test.go), [golden ui-state.png](../tests/golden/testdata/ui-state.png)) plus a smoke run of `cmd/gowez-hello`. Event semantics: [docs/EVENTS.md](EVENTS.md).

**M4 gate:** the boxes checked below were closed on **2026-10-07** — automated evidence ([script tests](../internal/script/engine_test.go), [ipc tests](../internal/ipc/dispatcher_test.go), [app tests](../internal/app/input_test.go) incl. `TestSceneJSDrivesStateThroughIPC`, experiments [B](../evidence/experiments/2026-10-07_b_js-handler-exception.md)/[C](../evidence/experiments/2026-10-07_c_unknown-method.md)) plus a smoke run of `cmd/gowez-hello` (alive, SIGTERM → rc 0). Engine decision: [DRR-004](../evidence/records/2026-10-07_js-engine-goja.md). Host contract: [docs/SCRIPT.md](SCRIPT.md).

## Core correctness (M1–M4)

- [x] Application starts from a clean environment — [startup sequence test](../tests/integration/window_test.go), [experiment A](../evidence/experiments/2026-10-03_a_renderer-init-failure.md)
- [x] Native window opens — [tests/integration/window_test.go](../tests/integration/window_test.go)
- [x] Svelte UI loads — [`TestGowezDashboardMounts`](../tests/golden/bundle_test.go), [`TestBundleMountsFixture`](../internal/app/bundle_test.go), [`TestDashboardWindowMounts`](../tests/integration/dashboard_test.go) (the same bundle, mounted into a **real OS window**); committed `examples/gowez-dashboard/dist` mounts through the real path (manifest → CSS → sandbox eval → `ui.apply` → layout). Operator visual check: [DEVELOPMENT_GUIDE §Manual window check](../DEVELOPMENT_GUIDE.md#manual-window-check)
- [x] Text renders — [golden text.png](../tests/golden/testdata/text.png)
- [x] Basic shapes render — [golden rects.png](../tests/golden/testdata/rects.png)
- [x] Button renders — [golden ui.png](../tests/golden/testdata/ui.png) ("Apply" button in the M2 scene), [uiscene.go](../internal/app/uiscene.go)
- [x] Mouse click reaches UI — [input_test.go](../internal/app/input_test.go) (`TestLoopDispatchesInput`, `TestSceneClickUpdatesState`), [interaction_test.go](../internal/ui/interaction_test.go)
- [x] Keyboard input works — [input_test.go](../internal/app/input_test.go) (`TestSceneKeyEchoUpdatesStatus`), [interaction_test.go](../internal/ui/interaction_test.go) (`TestTabTraversalWraps`, `TestEnterSpaceActivatesFocusedButton`)
- [x] UI state can update — [input_test.go](../internal/app/input_test.go) (`TestSceneClickUpdatesState`: click → text change → dirty → relayout), [golden ui-state.png](../tests/golden/testdata/ui-state.png)
- [x] JS exception is observable — [experiment B](../evidence/experiments/2026-10-07_b_js-handler-exception.md), [TestSceneJSHandlerFailureIsObservable](../internal/app/input_test.go), [TestFireHandlerExceptionIsolatedAndCounted](../internal/script/engine_test.go) (`JSExceptions` metric + `Component "script"` diagnostic)
- [x] Application exits cleanly — window close path + SIGINT/SIGTERM → rc 0, [lifecycle.go](../internal/app/lifecycle.go), [integration test](../tests/integration/window_test.go)

## Svelte integration (M5)

**M5 gate closed 2026-10-07, window proof added 2026-10-08, style scoping added
2026-10-08.** Pipeline, applier, bundle loader, text input, reactivity and
component style scoping are implemented and tested. Reactivity is a full
re-render per state change, diffed against the previous node tree by node id
([SVELTE.md §Reactivity model](SVELTE.md#reactivity-model)).

> **Scope note — layout fidelity is *not* an M5 criterion.** The M5 subset has no
> inline flow, no percentage heights, no margin collapsing and no CSS Grid, so a
> real-world application does not yet render faithfully. The golden PNG locks the
> *pipeline* (Svelte → adapter → bundle → sandbox → `ui.apply` → style → layout →
> paint → pixels), not the visual result; regenerating it as capabilities land is
> an improvement. What an application may not use is enumerated by the gap
> register, and closing that gap register is the scope of
> [DRR-008](../evidence/records/2026-10-08_dashboard-target.md) (confirmed
> 2026-10-08 — see the roadmap in README).

- [x] Svelte component can be compiled — [`TestCompile`](../packages/adapter/test/compile.test.ts) (six in-subset fixtures, strict, zero findings), [`TestGowezDashboardMounts`](../tests/golden/bundle_test.go) (the committed bundle mounts, styles, lays out)
- [x] Component styles are scoped to their component — [`scoping.test.ts`](../packages/adapter/test/scoping.test.ts) (10 tests: scope on every selector, on every element, per-module isolation, collision detection, combinator printing), [`TestBundleStylesAreScoped`](../tests/golden/bundle_test.go) (end-to-end through `internal/style`: each `<button>` resolves the padding declared by *its own* module — the leak that 8 dashboard selectors had)
- [x] Component state can update — [`TestReactivityStateChangeUpdatesUI`](../internal/app/reactivity_test.go) (click → `$state` write → re-render → diff reaches the tree), [`TestReactivityRepeatClicksStayBounded`](same) (five clicks: no duplicated nodes, one batch each)
- [x] Event handler works — [`TestBundleClickIncrementsCount`](../internal/app/bundle_test.go), [`TestLoopDispatchesInput`](../internal/app/input_test.go)
- [x] Conditional rendering works — [`TestReactivityMountRendersCurrentState`](../internal/app/reactivity_test.go) (exactly one `{#if}` arm at mount), same test after a click (the arm flips)
- [x] List rendering works — [`TestReactivityMountRendersCurrentState`](../internal/app/reactivity_test.go) (one node per item), [`TestReactivityListUpdateRemovesOnlyOne`](same) (keyed: removing one row of three emits a bounded diff), [`TestReactivityFilteredListReactsToInput`](same) (typing filters the list)
- [x] Basic component composition works — [`TestCompile` "compiles a child module before its importer"](../packages/adapter/test/compile.test.ts); `<Child {items} onRemove={…} />` inlines one tree root ([golden](../tests/golden/testdata/gowez-dashboard.png))
- [x] Required browser APIs are documented — [SVELTE.md](SVELTE.md) §Gap register, generated from adapter `report` mode and asserted by [`gap-report.test.ts`](../packages/adapter/test/gap-report.test.ts)
- [x] Unsupported Svelte/browser behavior is explicit — [`reject.test.ts`](../packages/adapter/test/reject.test.ts) (one test per forbidden construct), [`gap-report.test.ts`](../packages/adapter/test/gap-report.test.ts) (dashboard scan), closed code catalog in [`findings.ts`](../packages/adapter/src/findings.ts)

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

- [x] Unit tests pass — `go test ./... -count=1` green (2026-10-08)
- [x] Integration tests pass — `go test -tags integration ./tests/integration/` green: [`TestWindowLifecycle`](../tests/integration/window_test.go) (raw SDL backend) + [`TestDashboardWindowMounts`](../tests/integration/dashboard_test.go) (the compiled Svelte bundle mounted into a real OS window, 3 frames presented, no rejected batches) (2026-10-08)
- [x] Svelte integration tests pass — 125 adapter tests (`packages/adapter/test`) plus the Go bundle, reactivity, text-input and golden suites; full battery green (`npm test`, `npm run typecheck`, `go build`, `gofmt`, `go vet`, `go test ./... -count=1`, `-race`, `go test -tags integration`)
- [x] Renderer tests pass — `internal/render/backend/software` + [goldens](../tests/golden) byte-exact (2026-10-08)
- [x] IPC tests pass — [dispatcher tests](../internal/ipc/dispatcher_test.go) (2026-10-08)
- [ ] Failure experiments pass — A–C executed with records; D (native) and E (resource) are M6 / M5+ and still pending
- [ ] Basic benchmark exists
- [x] Documentation updated — [SVELTE.md](SVELTE.md), [SCRIPT.md](SCRIPT.md), [EVENTS.md](EVENTS.md), [CSS-SUBSET.md](CSS-SUBSET.md), [CHECKLISTS.md](CHECKLISTS.md), [GUARDRAILS.md](GUARDRAILS.md) changelog, README/AGENTS/DEVELOPMENT_GUIDE (2026-10-08)

## MVP definition of done

The MVP counts as complete for **technical validation** when:

- [x] A Svelte application builds successfully — [`buildFixture` strict tests](../packages/adapter/test/compile.test.ts) (six in-subset fixtures, zero findings) and `npm run build -w @gowez/example-gowez-dashboard` → [dist/](../examples/gowez-dashboard/dist) (committed, 102 ops)
- [x] No Chromium dependency — [go.mod](../go.mod) carries pure-Go deps only; `CGO_ENABLED=0 go build ./...` passes
- [x] No OS WebView dependency — windowing via purego SDL3, [DRR-001](../evidence/records/2026-10-03_windowing-purego-sdl3.md)
- [x] The Go binary can create a native window — `cmd/gowez-hello`, [integration test](../tests/integration/window_test.go)
- [ ] Basic UI renders through the GPU
- [x] The user can interact with the UI — click/keyboard/state via [input_test.go](../internal/app/input_test.go) + [EVENTS.md](EVENTS.md) semantics; interactive scene in [uiscene.go](../internal/app/uiscene.go)
- [x] Svelte state produces UI updates — [`reactivity_test.go`](../internal/app/reactivity_test.go) against the committed bundle, driving the real path end to end
- [x] JS can call explicit Go APIs — [experiment C](../evidence/experiments/2026-10-07_c_unknown-method.md) (JS path asserts `e.code`), [TestInvokeSuccessReturnsResult](../internal/script/bindings_test.go), [TestSceneJSDrivesStateThroughIPC](../internal/app/input_test.go)
- [ ] Go APIs can access at least one native capability
- [ ] Main failure modes can be tested
- [ ] Startup/memory/rendering benchmarks exist
- [x] Browser/Svelte compatibility limits are documented — [SVELTE.md](SVELTE.md) §Gap register (generated, test-asserted), §Divergences, §Svelte compatibility; [CSS-SUBSET.md](CSS-SUBSET.md) for the CSS half

Reaching these does **not** mean the framework is production-ready — it only proves the architectural hypothesis is worth continuing.

## Paint & value (M6)

Scope authorised by [DRR-008](../evidence/records/2026-10-08_dashboard-target.md);
design and per-step gates in the [M6 design note](../evidence/records/2026-10-08_m6-paint-and-value.md).
The milestone gate is a **finding budget** in the gap register
(`npm run report:dashboard`), not a claim of visual fidelity.

- [x] **M6a** custom properties and `var()` — [vars_test.go](../internal/style/vars_test.go) (inheritance of the computed value, `var(--x, fallback)`, unset **and** empty fallback, cycle reported not hung, order-independence over a three-link chain, substitution inside a shorthand, and that a substituted value is still type-checked)
- [x] **M6a** `color-mix(in srgb, …)` — premultiplied-alpha interpolation pinned per percentage case; every other colour space refused **by name**, because mixing in srgb while the author asked for oklch returns a different colour than any browser
- [x] **M6a** `list-style: none` / `outline: none` — accepted because the outcome is already true (nothing paints a marker or an outline), and every other value is refused rather than swallowed ([parity cases](../tests/parity/parity.go))
- [x] **M6a** per-side border colour — `ComputedStyle.BorderColor` is `[4]uint32`; `border-color` expands to four longhands; `drawBorders` skips a transparent side individually
- [x] **M6a** the gap register is complete — [cssSubsetErrors](../packages/adapter/src/css.ts) collects every rejection instead of throwing on the first per rule. The baseline was **259**, not the 139 an earlier record published; the count is not monotonic while unmasked findings remain behind the first error in a rule
- [ ] **M6b** `background` / `border` / `border-<side>` shorthands — gate `CSS-PROPERTY` 123 → 89
- [ ] **M6c** `font-weight` — gate 89 → 77; golden must change (a real bold face, not faux bold)
- [ ] **M6d** `border-radius` — gate 77 → 65; **extends the `render.Renderer` contract**, so it needs a G-UPG-03 changelog entry
