# M6 — paint & value: design note

- **Date:** 2026-10-08
- **Status:** design agreed; **M6a complete** (gated), M6b next
- **Decision class:** A (no `protocol/` change, no root public API change, no new
  dependency). The M6 *scope* is authorised by
  [DRR-008](2026-10-08_dashboard-target.md); this note records how it is cut up
  and why.

## Why M6 is four steps

DRR-008 described M6 as one gate, "CSS findings 71 → ≤25". Measurement during
planning showed that description was wrong in two ways, so it is corrected here
rather than quietly attempted:

1. **M6 is not one subsystem.** It spans the style parser, the cascade, the text
   stack and the renderer. Bundled into one gate, a failure cannot name the
   capability that broke — the exact unfalsifiable gate DRR-008 rejected when it
   rejected "one big M6".
2. **The original number was unreachable even before the shared stylesheet was
    measured.** The component-only tables named M7/M8 work that left post-M6 CSS
    findings above the first proposed ceiling. With `src/styles/ui.css`
    included explicitly, the M6 gates below are the only current numbers; later
    gates live in DRR-008 §Correction and must be re-measured when those designs
    land. The gate below is stated as a **closed count of
    `CSS-PROPERTY`**, which is computable from the register rather than guessed.

## Verified facts this design rests on

Each was measured, not assumed.

| Fact | How | Consequence |
|---|---|---|
| The embedded Go faces have **no GSUB table** | read the TTF table directory of `goregular`, `gomedium`, `gobold`: `cmap, cvt , fpgm, gasp, glyf, head, hhea, hmtx, loca, maxp, name, post, prep` — **no GSUB** | `font-variant-numeric: tabular-nums` is **not implementable**. Excluded from M6; moved to M11 in DRR-008. Accepting it would be a silent no-op (G-UPG-04) |
| `gomedium` and `gobold` exist in `golang.org/x/image/font/gofont` | module cache listing — the module is **already a dependency** | `font-weight` can select a genuinely different face. **No new dependency** |
| `render.Renderer` exposes only `DrawRect` | `internal/render/renderer.go` | `border-radius` requires a **new command**, i.e. the first non-rectangular primitive in the contract. Authorised explicitly |
| The OpenGL backend is a stub (`ErrNotImplemented`) | `internal/render/backend/opengl/renderer.go` | G-UPG-03's "dual-backend tests" is **planned, not enforced**. Noted, not claimed |
| `ComputedStyle` has a single `BorderColor uint32` | `internal/style/computed.go:86` | `border-bottom: 1px solid red` cannot be represented. Per-side border colour is a prerequisite for M6a |
| Nothing paints list markers or outlines | `internal/paint` draws background, borders, text only | `list-style: none` / `outline: none` are **true no-ops**: accepting only these two values is honest, because the desired end state already holds |
| Shorthand expansion exists but only for 1–4 lengths | `internal/style/value.go:23` `kindBoxShorthand` | `border: 1px solid red` mixes a length, a style keyword and a colour — a new kind |

## The four steps — order corrected by measurement

The first cut of this note put the shorthands first and custom properties second.
**That order was wrong, and measuring the sample proved it.**

| Declaration | needs `var()` | literal |
|---|---|---|
| `background` | **23** | 1 (`transparent`) |
| `border` | 3 | 2 |
| `border-bottom` | 5 | 3 |
| `border-top` | 1 | 0 |

**32 of 39** shorthand declarations in the sample cannot validate until `var()`
substitution exists — a `border: 1px solid var(--border)` value is not a colour,
so the shorthand parser would reject it for the wrong reason and the finding would
stay. Implementing the shorthands first would have produced a large diff and
almost no movement on the gate that measures the milestone. `var()` is not a
separate concern that happens to be useful; it is the precondition for the largest
single group of findings in M6.

A second correction came from the same work: the gap register was **undercounting**
because the adapter's CSS validator stopped at the first bad declaration in a rule
(DRR-008 §Correction). A third correction added the shared global stylesheet
(`src/styles/ui.css`) explicitly, because the adapter never executes Vite's
`main.ts`. Every gate below is therefore stated against the complete baseline of
**198** `CSS-PROPERTY` findings, not the 65 the earlier record believed and not
the 185 component-only baseline.

| Step | Scope | Gate | `CSS-PROPERTY` | Render contract |
|---|---|---|---|---|
| **M6a** ✅ | custom properties, `var()`, `color-mix()`; `list-style: none`; `outline: none` | 0 remaining `var()` findings; cycle + fallback + inheritance tests | 185 → **123** component-only; **198** with global route | untouched |
| **M6b** | `border` / `border-<side>` / `background` shorthands (59) | golden PNG changes (dividers and literal backgrounds appear) | 198 → **139** | untouched |
| **M6c** | `font-weight` (24) | golden PNG changes (real bold), determinism re-verified | 139 → **115** | untouched |
| **M6d** | `border-radius` (18) | rounded-rect raster determinism; changelog entry for the contract change | 115 → **97** | **extended** |

Excluded and reported: `font-variant-numeric` (6 with global route) → M11.

`font-weight` and `border-radius` have no remaining `var()` findings in the sample
(24 and 18 findings), which is why they could be built in any order; they
sit last only because they carry the higher risk.

### What M6a actually changed beyond `var()`

- **`ComputedStyle.BorderColor` is now `[4]uint32`** instead of a single value,
  with `border-color` expanded to four longhands. This is a prerequisite, not a
  convenience: `border-bottom: 1px solid red` is a divider and one colour cannot
  express it. `drawBorders` now skips a transparent side individually.
- **The adapter's CSS validator collects instead of throwing** in report mode, so
  the register is complete. Strict mode still throws on the first failure — that
  is the one an author needs.

## Decisions

> **Heading correction.** The subsection below was written before implementation
> and labelled "M6a", which was wrong twice over: the *border colour* decision
> is M6b work (it landed as a prerequisite of M6a because the per-side change was
> needed to type a M6a test), and the shorthands it describes have not been
> built. It is now **M6b**. The record's own scope table above is the
> authoritative step list.

### M6b — shorthands

- **`border-color` becomes a per-side shorthand**, and `ComputedStyle.BorderColor`
  becomes `[4]uint32` (top, right, bottom, left). This landed during M6a as a
  prerequisite; the rest is still open.
- **`border-<side>` takes width and colour in any order**, plus the keywords
  `solid` (accepted and ignored — there is exactly one border style) and `none`
  (width 0, colour transparent).
- **`background` maps to `background-color` only.** Every other `background`
  component (`url()`, `image`, gradients, position, size, repeat) is **rejected by
  name** rather than ignored: silently dropping `background: url(hero.png)` would
  paint an empty box while the stylesheet looked honoured.
- **`list-style` and `outline` accept exactly one value each: `none`.** Anything
  else is a named finding. This is the honest-no-op rule: the value is accepted
  because the outcome is already true, and only because it is.

### M6a — custom properties (as built)

- Custom property values are stored as a **raw token stream**, not parsed, so
  `--x: 2px` is legal for any property that later consumes it.
- Substitution happens at **computed-value time** — after the cascade, before the
  substituted text is validated. That is the only order under which
  `--brand: #f00` + `border-color: var(--brand)` can type-check.
- Custom properties **inherit their computed (already-substituted) value**, per CSS
  Variables 1 §2.2. This is pinned by a test because it is counter-intuitive: a
  descendant that overrides the *referenced* property does not change a value that
  was substituted higher up. A descendant that declares its own chain resolves
  against its own scope, which is how the dashboard's theme override works.
- `var(--x, fallback)` uses the fallback when `--x` is unset **or empty**.
- A **reference cycle is an error with position**, never a silent empty value.
  `--a: var(--b); --b: var(--a)` has no least fixed point.
- Declaration order within a rule does not matter: every custom property a matching
  rule sets is collected first, then resolved. Resolving as encountered made the
  answer depend on Go's map iteration order — `var()` failures appeared
  intermittently under `-count`, which is the worst shape a test can have.
- A **shorthand containing `var()` expands after substitution**, not before: its
  value is not yet a known token list at parse time.
- `color-mix()` supports `in srgb` only, two colours, optional percentages, with
  **premultiplied alpha** interpolation. Other colour spaces are refused **by
  name**: mixing in srgb while the author asked for oklch returns a different
  colour from every browser, which is a silent wrong answer rather than a refusal.
- `list-style` and `outline` accept exactly `none`. The outcome is already true —
  nothing paints a marker or an outline — so `none` is *honest*, while every other
  value is refused rather than swallowed.

### M6c — font weight

- Weights are matched to the **nearest available face**: 400 → Regular,
  500 → Medium, 700 → Bold. The dashboard asks for 550, 600 and 650, which render
  as the nearest face. **That is a documented divergence, not a silent one** —
  a synthesised (faux bold) face is explicitly not used, because synthesising
  changes advances in a way that would silently alter every measurement.

### M6d — rounded rectangles

- `render.Renderer` gains a rounded-rect command. The contract change is recorded
  in the G-UPG-03 changelog with a test pinning it.
- Radii are clamped to half the corresponding side (CSS behaviour) rather than
  rejected; `999px` and `50%` therefore mean "pill" and "circle".
- Percentages resolve against the box's own side, so `50%` is circular.

## What is explicitly not done in M6

- `internal/layout` is **not touched**. `line-height`, `min`/`max`, `margin: auto`,
  `overflow` and percentage heights are M7; grid is M8. This keeps M6's blast
  radius to parsing, cascade, text selection and one paint primitive.
- No faux-bold synthesis, no accepting-and-ignoring a property whose effect is
  absent, and no `!important`.

## Evidence required per step

- `tests/parity` verdict-for-verdict against the Go parser for every new
  property/value pair, on both the accept and the reject side.
- One capability fixture per step with its own golden, so a failure names the
  capability.
- The gap register re-measured at the step's gate.
- Full battery: `gofmt`, `go vet`, `go test ./... -count=1`, `-race`,
  `-tags integration`, `npm test`, adapter typecheck.