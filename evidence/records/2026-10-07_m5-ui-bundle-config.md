# DRR-006 — UI bundle loading: `Config.UI fs.FS` + bundle layout

- **Date:** 2026-10-07
- **Author:** agent
- **Status:** **confirmed**
- **Decision class:** B (root public API change)

## Context

Milestone 5 must hand the runtime a UI authored in Svelte. The adapter
(`packages/adapter`) produces a build artifact; the Go side must load it at
startup. Today `internal/assets.Loader` exists (`internal/assets/loader.go`,
wrapping an `fs.FS`) but nothing calls it, and `gowez.Config` has only
`Title`/`Width`/`Height`.

The startup sequence in [`docs/PLATFORM.md`](../../docs/PLATFORM.md) already
reserves the slot: `config → window → renderer → JS runtime → **UI bundle** →
UI tree → layout → render → ready`. This DRR decides how the bundle enters the
process.

## Options considered

1. **`Config.UI fs.FS`** — the application supplies a filesystem; the runtime
   reads `manifest.json`, `app.js`, `styles.css` from it.
2. **`Config.UIPath string`** — a directory path on disk. Rejected: ties the
   runtime to the OS filesystem and duplicates what `fs.FS` already expresses.
3. **`Config.UIData map[string][]byte`** — in-memory bytes. Rejected for
   development (no incremental build loop); viable at packaging time and
   explicitly preserved, because `embed.FS` satisfies option 1 directly.
4. **A global/setter outside `Config`.** Rejected: hidden state, and it would
   make two app instances impossible to reason about.

## Decision

**Option 1**, with the bundle layout as the versioned contract:

```text
dist/
├── manifest.json  # {schemaVersion:1, adapter:{name,version}, svelte:"5.x",
│                   #  script:"app.js", styles:"styles.css"}
├── app.js         # single-file IIFE, no imports — mount + effects + handlers
└── styles.css     # component CSS validated against the subset
```

- `gowez.Config` gains `UI fs.FS`.
- `internal/app/Options` mirrors it; `Run` passes it through.
- `Config.UI == nil` keeps the M1–M4 demo scene byte-for-byte, so no existing
  test changes behavior.
- Development: `os.DirFS("dist")`. Packaging (M7): `//go:embed`.

`manifest.json` carries `schemaVersion` and the supported Svelte range; a
mismatch is an explicit startup failure. This mirrors
`internal/api/app.go:11` ↔ `internal/ipc/dispatcher_test.go:308` — the
`ipcVersion` drift-guard pattern already in the codebase.

## Impact

- **Contract / root public API:** **additive** — one new optional field on an
  existing struct. No existing field changes meaning, no behavior changes when
  the field is nil. Precedent DRR-003/004: pre-release (no tags, no external
  consumers), so no version bump; recorded in the
  [`docs/GUARDRAILS.md`](../../docs/GUARDRAILS.md) changelog instead.
- **Regression test required** (AGENTS hard rule 3):
  `TestBundleLoadMissingFileExplicit`, `TestBundleManifestVersionMismatch`,
  `TestBundleCSSParseError`, `TestBundleEvalBudgetAbortsStartup`,
  `TestBundleMountsFixture`, `TestBundleClickIncrementsCount`, plus
  `TestDemoSceneUnchangedWhenUINil`.
- **`protocol/` schemas:** unchanged. The manifest is validated by Go code
  that is *stricter* than the schema where they differ (e.g. `setText` on an
  element node is an error in Go, permitted by the schema). Any schema edit
  later would be a separate Level B DRR.
- **Guard rails:** G-IFACE-02/03 satisfied by the additive shape + regression
  tests; G-DEP-05/G-REL-01 satisfied because the mount `Eval` stays inside the
  existing `Limits.EvalTimeout`.
- **Security:** the bundle is data `Eval`'d in the existing sandbox
  (`docs/SCRIPT.md` surface unchanged: no `document`, `window`, `setTimeout`,
  `fetch`). The adapter never emits DOM calls, so a compromised bundle gains
  nothing beyond what `docs/SCRIPT.md` already documents.
- **Scope:** MVP IN/OUT unchanged.

## Confirmation

**Confirmed by the repository owner, 2026-10-07** — decision #4 of the M5 plan
("Bundle loading: `Config.UI fs.FS`"), stated up front as requiring a DRR.
Implementation unblocked.
