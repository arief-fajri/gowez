# DRR-007 — M5 compile strategy: own subset compiler (Strategy B)

- **Date:** 2026-10-07
- **Author:** agent
- **Status:** **confirmed** (decided on evidence, per plan decision #1)
- **Decision class:** B (guard-rail-relevant; strategy A would have required a sandbox contract change)

## Context

The M5 plan locked decision #1 as "spike first (A vs B), decide with
evidence", deferring the choice until Phase 0 produced numbers. This DRR
records that decision.

Two candidate strategies for turning Svelte source into GoWEZ UI ops:

- **A — DOM-compat shim.** `svelte.compile(generate:'client')`, run inside
  goja behind a shim of `svelte/internal/client` plus a virtual `document`.
  The adapter would then translate DOM operations into ops.
- **B — own subset compiler.** `parse()` → walker → ops. The adapter never
  executes Svelte's runtime; it emits code against its own primitives.

## Evidence

Full measurement, including the defect found and fixed during the probe, is in
[`2026-10-07_m5_compile-strategy-spike.md`](../experiments/2026-10-07_m5_compile-strategy-spike.md).
Headline numbers:

| | Strategy A | Strategy B |
|---|---|---|
| `$.` internals to shim | **35** (of 196 exported by `svelte/internal/client`) | 0 |
| HTML template parser needed | yes (`$.from_html`) | no |
| `queueMicrotask` needed | **yes** — absent from goja | no |
| `document` global in sandbox | **yes** — breaks the contract | no |
| Ops emitted for the 7 sample modules | n/a | **931** |
| Gap findings | n/a | **288** |

Measured goja gaps that make A worse: `queueMicrotask`, `structuredClone`,
`Object.groupBy`, `performance`, `WeakRef`, `Array.fromAsync`.

Both strategies produce the same deliverable shape (ops + gap register), so
the comparison is purely cost and guard rails.

## Options considered

1. **B — own subset compiler.** Adopted.
2. **A — DOM-compat shim.** Rejected: 35 internals plus a parser plus a
   scheduler is a second Svelte runtime maintained by us; the shims track
   Svelte internals rather than a public API, so every Svelte upgrade is a
   re-audit; and `document` would break `internal/script/engine_test.go:292`
   and the "No DOM" section of `docs/SCRIPT.md`.
3. **Defer / ship neither.** Rejected: the M5 gate requires a working compile
   path.

## Decision

**Strategy B.** `packages/adapter` walks the Svelte AST and emits UI ops plus
an explicit finding for every construct outside the subset. Svelte's runtime
never executes in the Go process.

### TypeScript handling (D-2)

`parse()` does **not** strip TypeScript — verified: the AST retains
`TSTypeAnnotation` (73 occurrences) and `TSInterfaceDeclaration`. The helper
`remove_typescript_nodes` is unexported and deep-importing it fails with
`ERR_PACKAGE_PATH_NOT_EXPORTED`. The walker therefore strips TS itself:

- **unwrap to inner expression:** `TSTypeAnnotation`, `TSAsExpression`,
  `TSSatisfiesExpression`, `TSNonNullExpression`, `TSTypeAssertion`
- **drop:** `TSInterfaceDeclaration`, `TSTypeAliasDeclaration`,
  `TSEnumDeclaration`, `TSDeclareFunction`

All 18 TS shapes present in the sample fall into one of those two rules. A TS
shape the walker does not recognize **fails the build explicitly** (G-UPG-04) —
it never reaches codegen as a residual annotation. `import type` needs no
handling: it is already absent from the AST.

## Impact

- **Contract:** none. No `protocol/` change, no root API change.
- **Guard rails:** none broken. The sandbox contract in `docs/SCRIPT.md`
  stands verbatim; `TestSandboxSurface` keeps asserting `document ===
  undefined`.
- **Scope:** MVP IN/OUT unchanged.
- **Consequence for Phase 3:** the adapter owns the Svelte subset surface
  explicitly. Every construct outside it is a named finding code, published
  in the generated gap register ([`docs/SVELTE.md`](../../docs/SVELTE.md)).
- **Maintenance:** upgrades track Svelte's **AST** shape (a documented, far
  more stable surface than client internals), and the supported range is
  pinned + integration-tested per G-UPG-02 / G-IFACE-05.

## Confirmation

**Confirmed by the repository owner, 2026-10-07** — decision #1 of the M5 plan
("spike first, decide with evidence"), with the strategy B outcome accepted
after review of the spike evidence.
