# M5 Phase 0 — compile-strategy spike (Strategy A vs B)

**Date:** 2026-10-07
**Type:** validation run (per [`README.md`](README.md): "failure experiment or validation run")
**Question:** which compile strategy reaches the M5 acceptance gate with less
total cost and without breaking a guard rail — A (run Svelte's own client
runtime inside goja) or B (own subset compiler walking the Svelte AST)?

**Outcome:** **B**, on evidence. Decision recorded in
[DRR-007](../records/2026-10-07_m5-compile-strategy.md).

---

## Method

Both strategies were exercised against the **real** sample — all 7 modules of
`examples/dashboard/src` (1,623 lines, every file `lang="ts"`), not a toy
fixture.

Probe used for Strategy B: `spike-m5-walker-probe.mjs` — a throwaway
prototype, **deleted after this record and therefore not linkable**. The shipped
walker lives in `packages/adapter`; the measurement below is what this record
claims, and the shipped code is what a reader can re-run.

---

## Strategy A — DOM-compat shim (rejected)

`svelte.compile(generate:'client')` output run inside goja behind a shim of
`svelte/internal/client` plus a virtual `document`.

Measured facts:

| Fact | Value |
|---|---|
| Distinct `$.` internals referenced across the 7 modules | **35** |
| Total symbols exported by `svelte/internal/client` | **196** |
| Compiled output referencing runtime `document`/`window` | **0** |
| `queueMicrotask` in goja | **absent** (`ReferenceError`) — needed by `dom/task.js:19`, `dom/elements/events.js:42`, `:314`, `reactivity/async.js:212` |
| Other goja gaps | `structuredClone`, `Object.groupBy`, `performance`, `WeakRef`, `Array.fromAsync` |

Cost drivers, in order:

1. **35 internals to shim** with exact Svelte 5 semantics — each one is a
   re-implementation of framework behavior, not a stub.
2. **HTML template parsing.** `$.from_html` parses HTML at runtime; goja has
   no DOM parser, so the shim would need one.
3. **Scheduler.** `queueMicrotask` is missing and must be synthesized on top
   of goja's promise jobs (which *do* drain across `RunString` boundaries —
   verified).
4. **Sandbox contract break.** `internal/script/engine_test.go:292` asserts
   `document === undefined`. A virtual `document` is a Level B contract
   change to `docs/SCRIPT.md` ("No DOM"), plus regression work.
5. **Version fragility.** Shims track Svelte internals, not a public API.

**Rejected.** Items 1–3 are a second Svelte runtime maintained by us; item 4
breaks a guard rail the plan was explicitly not allowed to break silently.

## Strategy B — own subset compiler (adopted)

`parse()` → walker → UI ops. No DOM, no Svelte runtime, no shim.

### TS handling (decision D-2)

`parse()` does **not** strip TypeScript, and `remove_typescript_nodes` is not
exported (`ERR_PACKAGE_PATH_NOT_EXPORTED` on deep import). The walker
therefore strips TS shapes itself:

- **unwrapped** to the inner expression: `TSTypeAnnotation`, `TSAsExpression`,
  `TSSatisfiesExpression`, `TSNonNullExpression`, `TSTypeAssertion`
- **dropped entirely**: `TSInterfaceDeclaration`, `TSTypeAliasDeclaration`,
  `TSEnumDeclaration`, `TSDeclareFunction`

All 18 TS shapes actually present in the sample were enumerated and are
covered by one of those two rules: `TSTypeAnnotation` (73), `TSTypeReference`
(40), `TSPropertySignature` (27), `TSStringKeyword` (20), `TSArrayType` (14),
`TSTypeParameterInstantiation` (9), `TSFunctionType` (9), `TSVoidKeyword` (9),
`TSTypeLiteral` (8), `TSAsExpression` (7), `TSLiteralType` (6), `TSUnionType`
(3), `TSNumberKeyword` (3), `TSIndexedAccessType` (2), `TSTypeQuery` (1),
`TSTypeAliasDeclaration` (1), `TSInterfaceDeclaration` (1),
`TSInterfaceBody` (1).

`import type` disappears from the AST on its own — no handling needed.

### Result of the full walk

| Module | Ops emitted | Findings |
|---|---|---|
| `src/App.svelte` | 113 | 45 |
| `src/pages/Dashboard.svelte` | 226 | 53 |
| `src/pages/Users.svelte` | 170 | 53 |
| `src/pages/Analytics.svelte` | 120 | 44 |
| `src/pages/Orders.svelte` | 117 | 36 |
| `src/pages/Reports.svelte` | 70 | 23 |
| `src/pages/Settings.svelte` | 115 | 34 |
| **total** | **931** | **288** |

The op stream builds a real tree with no DOM. Findings are the gap register,
exactly as the plan's Track B requires:

| Finding | Count | Meaning |
|---|---|---|
| `EXPR-UNSUPPORTED` | 262 | prototype lacks an expression printer (real adapter ships one) |
| `ELEMENT-REJECTED` | 11 | `option`(17) `th`(10) `td`(10) `select`(6) `tr`(4) `strong`(3) `table`(2) `thead`(2) `tbody`(2) |
| `COMPONENT-SHIM` | 6 | child components — supported via the adapter shim |
| `SVELTE-HEAD-REJECTED` | 6 | one per page module |
| `CSS-INLINE-DYNAMIC` | 2 | dynamic `style` attribute |
| `TRANSITIONDIRECTIVE-REJECTED` | 1 | `in:fade` |

### Defect found and fixed during the spike

The probe initially walked `element.children`, which the modern AST does not
have — children live under `element.fragment.nodes`. The walk therefore
skipped every nested element and reported `ELEMENT-REJECTED: 0` despite 11
present in the sample. The symptom was self-inconsistent output (931 ops after
the fix, 72 before; 11 element findings after, 0 before). Fixed; lesson below.

---

## Conclusion

| | A | B |
|---|---|---|
| New runtime to maintain | Svelte client internals (35 shims) + HTML parser + scheduler | none |
| Guard rails broken | G-UPG-04 (`document`), sandbox contract | none |
| Output | ops + gaps | ops + gaps |
| Cost driver | volume of framework re-implementation | subset surface to define |

**B adopted.** A is not cheaper by any measure measured here, and it is the
only option that would require a guard-rail change.

---

## Lessons (recorded in [`learnings.md`](../learnings.md))

1. **A finding count of zero is a bug report, not a result.** The probe's
   first run reported `ELEMENT-REJECTED: 0` on a sample containing 11
   rejected elements. A gap register whose gaps are missing is worse than no
   gap register — it converts "unsupported" into "silently supported"
   (G-UPG-04). Cross-check totals against an independent count before
   trusting a scan.
2. **The toolchain assumption in the plan was wrong.** The plan recorded
   "esbuild already transitive via vite" — the installed Vite is **8.3.3** and
   bundles with **rolldown**; esbuild is absent from the lockfile entirely.
   Re-verify "already available" claims against the lockfile.
3. **"parse() handles TypeScript" was half true.** `compile()` strips TS;
   `parse()` does not, and the helper is unexported. Any plan that leans on
   the AST must own its TS stripping explicitly.
