# DRR-005 — JS toolchain additions for the M5 adapter

- **Date:** 2026-10-07
- **Author:** agent (verified by inspection)
- **Status:** **confirmed**
- **Decision class:** B (new dependency)

## Context

Milestone 5 needs a TypeScript test runner and a bundler for the adapter
pipeline (`packages/adapter`) and the slice app
(`examples/gowez-dashboard`). The npm workspace already carries `svelte`,
`vite`, `@sveltejs/vite-plugin-svelte`, and `typescript` from the M4-era
sample work (recorded in
[`learnings.md`](../learnings.md), 2026-10-07).

The M5 plan assumed `esbuild` was already present as a transitive Vite
dependency and needed only a confirmation. **Inspection of
`package-lock.json` (lockfileVersion 3) shows that assumption is false:**

- No `esbuild` key exists anywhere in the lockfile.
- The installed Vite is **8.3.3**, which bundles through **rolldown**
  (`node_modules/rolldown`, `@rolldown/binding-*`); Vite 8 no longer ships
  esbuild as its transform engine.
- `vitest` is absent.

So this DRR covers **two brand-new direct dependencies**, not a confirmation of
existing ones. G-DEP-02 ("the JS engine is never an implicit source of
arbitrary native access") and the checklist box "dependencies are reviewed"
both apply: provenance and limits must be recorded, which is what §Impact does.

## Options considered

1. **`esbuild` + `vitest`** — esbuild for single-file IIFE bundling (goja has
   no module system, so the bundle must be one file with no imports) plus a
   `.svelte` onLoad plugin; `vitest` for the adapter's TypeScript test suite.
2. **Vite library mode + a different runner** — reuse the installed Vite 8 to
   produce the IIFE. Rejected: Vite 8's rolldown pipeline makes the `.svelte`
   virtual-module boundary we need an awkward fit, and it does not solve the
   test-runner gap.
3. **No bundler** — emit one already-flattened JS file from the adapter
   directly. Rejected: re-implementing module resolution and scope hoisting is
   exactly the work a bundler exists to do, and the module graph (6 page
   components + shared types) needs it.

## Decision

**Option 1.** Add `esbuild` and `vitest` as direct devDependencies of
`packages/adapter`.

Pinned with provenance:

| Package | Role | Why acceptable |
|---|---|---|
| `esbuild` | Bundle compiled modules to one IIFE file for goja; provide the `.svelte` onLoad plugin | Build-time only. Never ships to the Go binary, never executes inside goja. Output must be `format: 'iife'`, `target: 'es2015'`, `splitting: false` — the ES-level constraint comes from measured goja gaps (`structuredClone`, `Object.groupBy`, `performance`, `WeakRef` are absent). |
| `vitest` | Run the adapter's TypeScript tests in Node | Dev-time only. No runtime effect on the framework or on the sandbox surface. |

Neither package can reach the Go runtime, the UI tree, or any OS capability.
The sandbox surface (`docs/SCRIPT.md`) is unaffected: the bundle is data
produced at build time and `Eval`'d under the existing bounded engine.

## Impact

- **Contract:** none. No `protocol/` schema change; no root public API change.
- **Guard rails:** G-DEP-01/G-DEP-02 — additions reviewed and pinned here.
  G-UPG-04 unaffected (adapter compile errors are unchanged). G-SEC-01/02
  unaffected: the toolchain runs outside the sandbox.
- **Security:** both are devDependencies of a `private: true` workspace
  package; neither enters the Go module graph or the shipped artifact.
- **Scope:** MVP IN/OUT unchanged.
- **Correction recorded:** the M5 plan's claim that esbuild was already
  transitive was wrong (Vite 8.3.3 / rolldown). Recorded as a learning, not a
  silent fix.

## Confirmation

**Confirmed by the repository owner, 2026-10-07**, explicitly answering the
Level B question "add `esbuild` (a new direct dependency) and `vitest`?" —
affirmative. Implementation unblocked.
