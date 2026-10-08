# DRR-008 — `examples/dashboard` becomes the target application; M6–M10 program

- **Date:** 2026-10-08
- **Author:** agent (raised from a visual triage of the M5 slice)
- **Status:** open — awaiting human confirmation
- **Decision class:** B (MVP scope change; requires explicit human confirmation)

## Context

M5 shipped a working pipeline: Svelte → adapter → bundle → sandbox → `ui.apply` →
style → layout → paint → pixels. The gate was closed on 2026-10-07 and the slice in
`examples/gowez-dashboard` mounts, styles, lays out, reacts to state and passes a real
OS-window test.

A visual review of that slice against `examples/dashboard` — the full six-page admin
dashboard — then established three things:

1. **The slice renders badly, and it should be expected to.** The M5 subset has no
   inline flow, no percentage heights, no margin collapsing and no CSS Grid. The
   published golden PNG locked in that bad-looking output. The pipeline evidence was
   real; a *fidelity* claim would not have been.
2. **`examples/dashboard` has never been a target.** It was deliberately written as a
   browser test bed — "deliberately outside the M5 subset" per [docs/SVELTE.md](../../docs/SVELTE.md)
   — so its 139 adapter findings were an acceptable standing measurement rather than a
   backlog. Making it a target changes what that number means.
3. **The triage found a defect the gap register could not see.** Component styles were
   never scoped, so 8 selectors were declared by more than one module and the last rule
   silently won for all of them. A register that counts *rejections* is blind to output
   that is wrong rather than missing. This is fixed ([learnings](../learnings.md),
   2026-10-08) and is not what this DRR decides — it is the reason the DRR's acceptance
   criteria are written against *resolved* behavior rather than finding counts alone.

The open question is therefore a scope one: the subset stops being "the small thing the
slice needs" and becomes "the thing a real application needs", which means the MVP's
capability boundary moves and the roadmap must be re-sequenced to keep each milestone
falsifiable.

## Baseline measurement (2026-10-08)

`npm run report:dashboard` — **139 findings across 7 modules**:

| Code | Count | What it rejects |
|---|---|---|
| `CSS-PROPERTY` | 65 | grid, `overflow-*`, `border-radius`, `box-shadow`, shorthands, `font-weight`, `list-style`, custom properties, `var()`, `color-mix()`, `line-height`, `min`/`max`, `margin: auto`, percentage heights |
| `DOM-GLOBAL` | 29 | `location`, `window`, `document`, `setTimeout`, `history`, `navigator`, `fetch`, `console` |
| `ELEMENT-UNSUPPORTED` | 12 | attributes outside the subset |
| `ELEMENT-REJECTED` | 11 | `<table>`, `<tr>`, `<td>`, `<select>`, `<option>`, `<strong>` |
| `SVELTE-HEAD` | 6 | `<svelte:head>`, `<title>` |
| `SVELTE-IMPORT` | 6 | the shared `styles/ui.css` kit and other bare/non-`.svelte` imports |
| `CSS-SELECTOR` | 4 | `:last-child`, `::placeholder` |
| `SVELTE-UNSUPPORTED-NODE` | 2 | nodes outside the subset |
| `SVELTE-TRANSITION` | 2 | `in:fade` |
| `CSS-AT-RULE` | 2 | `@media` |

Grouped by the capability that would close them (approximate — buckets overlap):

| Bucket | ≈ findings | Contents |
|---|---|---|
| Paint & value | ~41 | shorthands, `border-radius`, `font-weight`, `list-style`, `outline`, custom properties, `var()`, `color-mix()` |
| Layout engine | ~20 | `line-height`, `min`/`max`, `margin: auto`, `height: 100%`, `box-sizing`, `overflow`/clipping |
| CSS Grid | ~5 | `display: grid`, `grid-*`, `@media` |
| Host capability | ~29 | timers, routing, document head, element lookup |
| Widget & animation | ~23 | table layout, `<select>` popup, transitions |
| Inline flow | ~1 | `<strong>` inside prose |

## Options considered

### Option 1 — Do nothing; keep the subset sized to the slice

The slice stays the only target. The dashboard remains a test bed.

- **For:** no scope change; every existing claim stays true; M6 stays "native APIs".
- **Against:** the thesis is that GoWEZ can render *real* applications. A runtime that
  renders a counter and cannot render a table has not demonstrated that. It also leaves
  the honest question open forever: *when* is the subset good enough?

### Option 2 — Declare the dashboard a target but keep a single M6

One milestone absorbs every bucket.

- **For:** fastest route to "the dashboard runs"; one gate.
- **Against: the gate is unfalsifiable.** If M6 fails, nothing identifies which capability
  regressed, and the milestone has no intermediate shippable state. This is the same
  failure as the M5 gate that was deliberately narrowed after its first plan. **Rejected
  on the same reasoning.**

### Option 3 — Make the dashboard the target; close the gap register over five sequenced milestones (RECOMMENDED)

Each milestone closes one capability bucket and lowers the finding budget by a measured
amount. Each is separately falsifiable and separately shippable.

| MS | Bucket | Gate (finding budget from today's 139) |
|---|---|---|
| **M6** | Paint & value — `background`/`border` shorthands, `border-radius`, `font-weight`, `list-style`, `outline`, custom properties + `var()`, `color-mix()` | CSS findings 71 → **≤25**; **no change to `internal/layout`** |
| **M7** | Layout engine — inline flow, `line-height`, `min-*`/`max-*`, `margin: auto`, `height: 100%`, `box-sizing`, `overflow` + clipping | CSS → **≤12**; prose renders on one line |
| **M8** | CSS Grid + `@media` — a second layout algorithm, so **its own DRR** | CSS → **≤2**; dashboard shell lays out correctly |
| **M9** | Host capability — bounded timers, `location`/`hashchange` + router, document head, element lookup. **Own DRR**: it reverses a deliberate exclusion in [docs/SCRIPT.md](../../docs/SCRIPT.md) | `DOM-GLOBAL` 29 → **0**; routing navigates |
| **M10** | Widget & animation — `<table>` layout, `<select>` popup, `in:fade`/`transition:*` | **139 → 0**; all six pages render and navigate |

Two mechanisms make each gate real rather than asserted:

1. **A finding budget, not a finding snapshot.** `gap-report.test.ts` becomes
   `expect(findings.length).toBeLessThanOrEqual(budget)`, with the committed baseline
   lowered per milestone and the snapshot kept as a regression check.
2. **Capability fixtures, not one screenshot.** Each capability gets its own fixture, its
   own strict-compile test and its own golden PNG. One large dashboard screenshot hides
   which capability broke; a per-capability golden does not. Layout fidelity stops being
   judged by the M5 golden PNG, which pins the pipeline only.

### Option 4 — Narrow the target: pick a subset of the dashboard

Declare a reduced target (e.g. one page, no routing, no widgets).

- **For:** a reachable gate.
- **Against:** it needs a *different* application, and the dashboard already exists and is
  realistic. Writing a second, smaller app to hit a smaller gate is the same as Option 1
  with extra work. Not recommended, but the honest fallback if M6–M10 proves too large to
  schedule.

## Recommendation

**Option 3.**

The dashboard is the target, and M6–M10 close the gap register in five sequenced
milestones, each gated by a measured finding budget and a per-capability fixture.

Reasons:

- The **finding budget is the falsifiable gate** this repo has been missing. It is
  measured, committed, and it moves down only when a capability actually lands.
- **Sequencing is by risk.** M6 touches no layout algorithm at all; M8 introduces a
  second one and gets its own DRR; M9 reverses a documented design decision in
  `docs/SCRIPT.md` and gets its own DRR. Each of those deserves a separate decision, and
  bundling them behind one gate would hide them.
- **The per-capability fixture follows from the same lesson as the scoping fix.** The
  scoping defect survived 139 published findings because the register measures
  *rejections*. Accepting the target therefore requires evidence per capability, not one
  aggregate number and one aggregate image.

### Scope impact

| | Before | After (if confirmed) |
|---|---|---|
| Target application | `examples/gowez-dashboard` slice | `examples/dashboard` (slice stays as the fast regression fixture) |
| CSS subset | sized to the slice | sized to a real application |
| Host surface | `gowez.*` only, no browser globals | adds bounded timers, `location`, document head |
| M6 | native APIs | **paint & value** (native APIs move later) |
| M7 | packaging | **layout engine** |
| M8 | — | **CSS Grid** |
| M9 | — | **host capabilities** |
| M10 | — | **widgets & animation** |
| Definition of "subset sufficient" | none | gap register = 0 **and** all six pages render |

The roadmap renumbering is the part most likely to be contentious: packaging (previously
M7) and native APIs (previously M6) are pushed behind ten milestones of rendering work.
That is the honest ordering for a render pipeline, but it is a scope decision and belongs
to the human, which is why this record is open.

### What this DRR does *not* decide

- The **algorithm** for grid (M8) — separate DRR.
- The **timer model** for M9 (frame-loop flush vs. a real clock; cancellation; bounds) —
  separate DRR, because `docs/SCRIPT.md` excluded timers deliberately.
- Any `protocol/` schema change or root public API change. None is required by this DRR.

## Impact

- **Contract:** no `protocol/` change, no root public API change. `Config.UI fs.FS` from
  DRR-006 is sufficient.
- **Guard rails:** G-UPG-04 (no silent misrender) is the rail most at stake — it is why
  the finding budget must fall to 0 and why per-capability goldens are required.
  G-UPG-03 (a render backend must not change the contract) is unaffected. G-SEC-01/02 are
  unaffected: new host APIs go through `internal/api` + `internal/permission` like every
  other native capability.
- **Security:** M9's `location` and document head are new host surfaces and need the same
  explicit-grant treatment as `app.getInfo`.
- **Scope:** MVP IN/OUT changes — see the table above. This is the Level B item.
- **Reversibility:** the M5 slice remains a committed, passing artifact, so the runtime
  stays demonstrable at every point in the program. Nothing in this DRR requires
  deleting or rewriting existing passing work.

## Confirmation

**Human approver:** ____________________  **Date:** ____________

Required before implementation. Silence is not approval (AGENTS.md → Decision authority);
this record stays `open` until it is signed.

On confirmation, the first implementation step is **M6 — paint & value**, whose design
note is written before code, exactly as M5's was.