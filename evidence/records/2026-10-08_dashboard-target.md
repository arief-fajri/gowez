# DRR-008 — `examples/dashboard` becomes the target application; M6–M10 program

- **Date:** 2026-10-08
- **Author:** agent (raised from a visual triage of the M5 slice)
- **Status:** ✅ confirmed 2026-10-08 (see Confirmation)
- **Decision class:** B (MVP scope change; requires explicit human confirmation)

## Correction (2026-10-08, during M6a planning)

**The baseline in this record was wrong, and so were the gates derived from it.**

The register reported **139 findings**. Planning M6a against it exposed that the
adapter's CSS validator **throws on the first bad declaration in a rule**, so
report mode recorded one finding per *rule* rather than per *problem*:

```
.app { display: grid; grid-template-columns: 240px 1fr; height: 100dvh }
```

reported only `display: grid`, hiding two more. `font-family`, `box-shadow`,
`grid-template-rows`, `100dvh` — none of them appeared anywhere in the register,
and all of them are in the sample.

`cssSubsetErrors` now collects every rejection, so the true baseline is:

| | `CSS-PROPERTY` | total |
|---|---|---|
| before the fix (one finding per rule) | 65 | 139 |
| **after the fix (complete register)** | **185** | **259** |
| after M6a (`var()`, custom properties, `color-mix()`, `list-style`, `outline`) | **123** | **197** |

This is worth stating plainly because it invalidates the *point* of a finding
budget. A count that under-reports is not merely imprecise, it is **non-monotonic
in the wrong direction**: closing one property can unmask two others behind it,
so the number can stay flat or rise while a milestone makes real progress — and a
gate of the form "the count must fall" would then fail for the right work. Any
future instrument that decides whether a milestone succeeded has to be able to
see the whole thing first.

**Classification:** D — missing observability. The runtime never misbehaved; the
measurement did, and it was the measurement every gate depended on.

### Corrected buckets (measured, not estimated)

| Bucket | findings | milestone |
|---|---|---|
| `background` / `border` / `border-<side>` shorthands | 34 | M6b |
| `border-radius` | 12 | M6d |
| `font-weight` | 12 | M6c |
| `font-variant-numeric` | 5 | M11 |
| layout (`line-height`, `min`/`max`, `margin: auto`, `overflow`, `white-space`, `height`, `flex`, `align-items`, `justify-content`, `text-overflow`) | 29 | M7 |
| grid (`display: grid`, `grid-template-*`, `grid-column`, `place-items`) | 9 | M8 |
| typography (`font`, `font-family`, `text-align`, `letter-spacing`, `text-transform`) | 8 | M7 |
| transitions | 5 | M10 |
| `cursor`, `box-shadow` | 3 | M10 / M6d |

### Corrected gates

| Step | `CSS-PROPERTY` gate |
|---|---|
| baseline | 185 |
| **M6a** `var()` + custom properties + `color-mix()` + `list-style` + `outline` | **123** ✅ achieved |
| M6b shorthands | 123 → **89** |
| M6c `font-weight` | 89 → **77** |
| M6d `border-radius` | 77 → **65** |
| M7 layout | 65 → **36** |
| M8 grid | 36 → **27** |
| M11 `font-variant-numeric` | 27 → **22** |

The remaining 22 (`transition`, `white-space`, `font`, `text-align`,
`letter-spacing`, `text-transform`, `cursor`, `box-shadow`, `display: inline`)
belong to M9/M10.

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
   — so its adapter findings were an acceptable standing measurement rather than a
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

`npm run report:dashboard` — the measurement is in the *Correction* section below; the
original reading of this record was wrong.

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

| MS | Bucket | Gate (finding budget — see Correction) |
|---|---|---|
| **M6** | Paint & value — `background`/`border` shorthands, `border-radius`, `font-weight`, `list-style`, `outline`, custom properties + `var()`, `color-mix()` | CSS findings 71 → **≤25**; **no change to `internal/layout`** |
| **M7** | Layout engine — inline flow, `line-height`, `min-*`/`max-*`, `margin: auto`, `height: 100%`, `box-sizing`, `overflow` + clipping | CSS → **≤12**; prose renders on one line |
| **M8** | CSS Grid + `@media` — a second layout algorithm, so **its own DRR** | CSS → **≤2**; dashboard shell lays out correctly |
| **M9** | Host capability — bounded timers, `location`/`hashchange` + router, document head, element lookup. **Own DRR**: it reverses a deliberate exclusion in [docs/SCRIPT.md](../../docs/SCRIPT.md) | `DOM-GLOBAL` 29 → **0**; routing navigates |
| **M10** | Widget & animation — `<table>` layout, `<select>` popup, `in:fade`/`transition:*` | all six pages render and navigate |

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
  scoping defect survived a published register that measured
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

**Human approver:** arief-fajri (repo owner)
**Date:** 2026-10-08
**Outcome:** confirmed, including the M6–M10 sequence as proposed.

Confirmed together: that `examples/dashboard` becomes the target application, that
the gap register becomes the acceptance metric, and the five-milestone order
(M6 paint & value → M7 layout engine → M8 CSS Grid → M9 host capabilities → M10
widgets & animation). The roadmap renumbering below is therefore in force;
native APIs and packaging move behind the rendering program.

Per the recommendation, the first implementation step is **M6 — paint & value**,
and M6's design note is written and agreed before code.

---

## Roadmap change (in force on confirmation)

| Milestone | Was | Now |
|---|---|---|
| M6 | Native APIs (fs, dialog, clipboard, window control) | Paint & value |
| M7 | Packaging + benchmarks + sample app | Layout engine |
| M8 | — | CSS Grid + `@media` |
| M9 | — | Host capabilities (timers, routing, document head) |
| M10 | — | Widgets & animation (table, select, transitions) |
| M11 | — | Text capabilities: OpenType features (`font-variant-numeric`, `tabular-nums`) |
| later | — | Native APIs, then packaging |

Nothing is deleted: `examples/gowez-dashboard` remains the committed fast
regression slice, so the runtime stays demonstrable at every point in the
program.

### M11 — why it exists, and why it is not in M6

Added after planning M6, because measurement showed `font-variant-numeric:
tabular-nums` is **not implementable with the embedded font**. The Go faces
(`goregular`, `gomedium`, `gobold` in `golang.org/x/image/font/gofont`) were
inspected at the TTF table level: they contain `cmap`, `glyf`, `head`, `hmtx`
and friends, and **no GSUB table at all** — therefore no OpenType features, ever.
Accepting `tabular-nums` would draw every digit at its default width while the
stylesheet claimed otherwise: a silent misrender, which is exactly what G-UPG-04
forbids.

So M6 does not accept the property, and M11 owns the whole class. M11 needs a
font that actually ships a `tnum` feature, which is a **new vendored font asset
and a licensing decision — Level B, own DRR when it starts**. Until then the
finding stands, honestly reported.

Weight is the opposite case and *is* in M6: Go Regular / Medium / Bold are three
real faces with different metrics, so `font-weight` can select a genuinely
different face rather than pretend.