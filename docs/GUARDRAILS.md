# Guard Rails


Guard rails are absolute: a violation is a defect, not a trade-off. Enforcement mechanisms (code, test, checklist) are tracked as milestones land. Rows marked ✅ have a live enforcement artifact as of their milestone (latest: M4, 2026-10-07).

## G-DATA — data / state integrity

| ID | Guard rail | Enforcement (planned) |
|---|---|---|
| G-DATA-01 | Atomic state updates have an explicit boundary | ✅ ui tree mutation tests ([tree_test.go](../internal/ui/tree_test.go), [ui_test.go](../internal/ui/ui_test.go)), interaction state transitions ([interaction_test.go](../internal/ui/interaction_test.go)) |
| G-DATA-02 | Failed operations never partially apply state | ✅ applier two-phase validation ([apply.go](../internal/ui/apply.go), `TestApplyOpsAtomicRejectsInvalidStream`, `TestSessionRejectedBatchKeepsState`); failure experiment D (native state) still pending — M6 |
| G-DATA-03 | Persisted state has a format/version | M6 persistence tests |
| G-DATA-04 | Destructive native operations require an explicit API call | api/registry review + tests |

## G-DEP — dependency guard rails

| ID | Guard rail | Enforcement (planned) |
|---|---|---|
| G-DEP-01 | Dependency versions are locked or have a compatibility policy | go.mod / package-lock.json |
| G-DEP-02 | The JS engine is never an implicit source of arbitrary native access | ✅ script sandbox tests ([TestSandboxSurface](../internal/script/engine_test.go), [bindings tests](../internal/script/bindings_test.go)), [DRR-004](../evidence/records/2026-10-07_js-engine-goja.md) |
| G-DEP-03 | Renderer/window dependency failure is observable | ✅ failure experiment A ([record](../evidence/experiments/2026-10-03_a_renderer-init-failure.md)) |
| G-DEP-04 | Portability-critical dependencies are documented | README + this file |
| G-DEP-05 | No dependency may cause an operation to block forever | bounded-call tests |

## G-IFACE — interface / compatibility

| ID | Guard rail | Enforcement (planned) |
|---|---|---|
| G-IFACE-01 | The public Go API has a documented contract | gowez.go docs + API tests |
| G-IFACE-02 | The UI ↔ Go IPC contract has a schema/version | ✅ `protocol/ipc.schema.json` + `ipcVersion` drift guard ([dispatcher_test.go](../internal/ipc/dispatcher_test.go)) |
| G-IFACE-03 | Breaking changes are explicit (version bump + regression test) | schema `version` field |
| G-IFACE-04 | Unsupported browser APIs are never treated as supported | ✅ adapter compile errors ([findings.ts](../packages/adapter/src/findings.ts), one test per rejected construct) + generated [gap register](SVELTE.md#gap-register) |
| G-IFACE-05 | The supported Svelte compatibility range is documented | ✅ [SVELTE.md §Svelte compatibility](SVELTE.md#svelte-compatibility); manifest carries the major version, the runtime refuses a mismatch at startup |

## G-SEC — security

| ID | Guard rail | Enforcement (planned) |
|---|---|---|
| G-SEC-01 | No arbitrary Go function exposure to JavaScript | ✅ JS surface limited to `gowez.invoke/call/on` ([TestSandboxSurface](../internal/script/engine_test.go)); handlers by explicit name ([registry](../internal/api/registry.go), [registry tests](../internal/api/registry_test.go)) |
| G-SEC-02 | Native capability only via explicit APIs | ✅ `permission.Set` gate in ipc, deny by default ([TestDispatchPermissionGate](../internal/ipc/dispatcher_test.go)); grant model completes in M6 |
| G-SEC-03 | File/system access has a permission boundary | permission tests |
| G-SEC-04 | Debug/internal interfaces are off in production builds | build tags |
| G-SEC-05 | Dependencies are reviewed for known vulnerabilities | periodic audit |

## G-REL — reliability

| ID | Guard rail | Enforcement (planned) |
|---|---|---|
| G-REL-01 | No IPC call blocks without a bound | ✅ dispatcher deadline tests ([dispatcher_test.go](../internal/ipc/dispatcher_test.go)), handler-budget tests ([bindings_test.go](../internal/script/bindings_test.go)), watchdogs in experiments B/C |
| G-REL-02 | JS errors never damage the native runtime | ✅ failure experiment B ([record](../evidence/experiments/2026-10-07_b_js-handler-exception.md)) |
| G-REL-03 | Renderer failure is a bounded failure | ✅ failure experiment A ([record](../evidence/experiments/2026-10-03_a_renderer-init-failure.md)) |
| G-REL-04 | Shutdown releases native resources | ✅ lifecycle + integration tests (idempotent close, present-after-close fails, SIGINT/SIGTERM → rc 0) |
| G-REL-05 | Crashes never silently corrupt data | failure experiment D |

## G-UPG — upgrade / evolution

| ID | Guard rail | Enforcement (planned) |
|---|---|---|
| G-UPG-01 | Runtime API changes are documented | this file + README |
| G-UPG-02 | The Svelte compatibility range is tested | ✅ `TestBundleManifestSvelteRangeMismatch` (Go) + `bundle.test.ts` target/format assertions (adapter) |
| G-UPG-03 | Renderer backends are swappable without changing the UI API | dual-backend tests (software + opengl) |
| G-UPG-04 | Divergence from browser semantics is documented | ✅ CSS subset divergence ([CSS-SUBSET.md](CSS-SUBSET.md)) + style errors carry `style: line X:Y` ([style_test.go](../internal/style/style_test.go)); Svelte/browser divergence table ([SVELTE.md §Divergences](SVELTE.md#divergences-from-browser-and-svelte-semantics)), one test per rejected construct |

## Changelog

Record every breaking change to `protocol/` schemas or the root public API here (G-IFACE-03). Format: `YYYY-MM-DD — description — version bump old → new`.

- 2026-10-08 — **M5 bundle mutation door**: the bundle path registers exactly one UI mutation method, `ui.apply`; node addressability is adapter-allocated ids inside validated op batches, never ambient tree access. `ui.setText` is scoped to the demo scene and is *not* registered when a bundle is loaded, so one door stays auditable (pinned by `TestBundleSceneRegistersOnlyUIApply`).
- 2026-10-03 — module path renamed `github.com/volantisfrontend/gowez` → `github.com/arief-fajri/gowez` — pre-release (no tags, no consumers), no version bump ([DRR-003](../evidence/records/2026-10-03_module-path-rename.md))
- 2026-10-08 — **M5**: root public API gains one additive optional field, `gowez.Config.UI fs.FS` (the compiled Svelte bundle), plus the bundle layout `{manifest.json, app.js, styles.css}` — pre-release (no tags, no external consumers), no version bump ([DRR-006](../evidence/records/2026-10-07_m5-ui-bundle-config.md)). `protocol/` unchanged: the manifest is validated by Go code that is stricter than the schema where they differ. New dev dependencies `esbuild` and `vitest`, build-time only ([DRR-005](../evidence/records/2026-10-07_m5-js-toolchain.md)). Reactivity model recorded in [SVELTE.md §Reactivity model](SVELTE.md#reactivity-model).
- 2026-10-07 — `ipc.schema.json` error-code description extended with `-32600` (version mismatch) and clarified the `-32601` wording — additive description only, no shape/version change, no version bump (recorded under [DRR-004](../evidence/records/2026-10-07_js-engine-goja.md))
