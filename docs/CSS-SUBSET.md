# CSS Subset (Milestones 2–3)

The style layer supports **only** the constructs listed here. Anything else is a
parse/resolve error with a position — never an ignored declaration and never a
silent misrender (Module 2 §2.4, G-UPG-04, hard rule 6). The renderer never
receives styles; it receives commands derived from computed styles and layout.

Owner package: `internal/style`. Consumers: `internal/layout`, `internal/paint`.

## Selectors

### Supported

| Form | Example | Notes |
|---|---|---|
| Type | `button`, `text` | matches `Node.Tag` (`text` matches text nodes) |
| Class | `.card` | matches any space-separated token in the `class` attribute |
| ID | `#title` | matches the `id` attribute |
| Pseudo-class | `button:hover`, `swatch:focus` | `:hover`, `:active`, `:focus` — matches `Node.State` set by input (see [EVENTS.md](EVENTS.md)) |
| Descendant | `card title` / `card .row` | one or more space-separated compound selectors |
| Group | `h1, h2` | comma-separated selectors sharing one block |
| Inline style | `<node style="color: #fff">` | highest priority, no selector matching |

A compound selector is `type?` followed by any `.class` / `#id` /
`:pseudo` suffixes in any order (`button.primary:hover`).

### Rejected at parse time (explicit error)

`*` (universal), `>` `+` `~` (other combinators), attribute selectors
(`[href]`), unsupported pseudo-classes (`:focus-visible`, `:nth-child`,
…), pseudo-elements (`::before`), `@`-rules (`@media`, `@import`),
`!important`.

## Properties

Units: `px` and `%` (see per-property notes) and the keyword `auto`.
Colors: `#RGB`, `#RRGGBB`, `#RRGGBBAA`, plus the keywords `black`, `white`,
`transparent`. Numbers are decimal (`0.5`).

| Property | Values | Initial | Used by |
|---|---|---|---|
| `display` | `block` \| `flex` \| `none` | `block` | layout |
| `width` | `auto` \| `px` \| `%` | `auto` | layout |
| `height` | `auto` \| `px` | `auto` | layout |
| `margin` | shorthand: 1–4 lengths (`px` only) | `0` | layout |
| `margin-top/right/bottom/left` | `px` | `0` | layout |
| `padding` | shorthand: 1–4 lengths (`px` only) | `0` | layout |
| `padding-top/right/bottom/left` | `px` | `0` | layout |
| `border-width` | shorthand: 1–4 lengths (`px` only) | `0` | layout + paint |
| `border-color` | color | `transparent` | paint |
| `background-color` | color | `transparent` | paint |
| `color` | color | `#000000` | paint (text) |
| `font-size` | `px` (positive) | `16` | layout (measurement) + paint |
| `flex-direction` | `row` \| `column` | `row` | layout |
| `justify-content` | `start` \| `center` \| `end` \| `space-between` | `start` | layout |
| `align-items` | `stretch` \| `start` \| `center` \| `end` | `stretch` | layout |
| `gap` | `px` | `0` | layout |
| `flex-grow` | number ≥ 0 | `0` | layout |
| `flex-shrink` | number ≥ 0 | `1` | layout |

Unknown properties, unknown values, `%` in `height`/`margin`/`padding`/
`border-width`, and malformed declarations are **errors with position**, not
warnings.

## Cascade

1. Declarations are collected in order: stylesheet rules by source order, then
   the inline `style` attribute last.
2. Specificity tuple `(id, class, type)`; a pseudo-class counts in the
   class column (`button:hover` = `(0,1,1)`); inline style beats every
   selector.
3. Higher specificity wins; equal specificity → later source order wins.
4. `display: none` wins over everything for that node: the node produces no
   layout geometry and is not painted.

Initial values are applied exactly once, at resolve time — a node with no
matching rule still gets a full `ComputedStyle`.

## Inheritance

Exactly two properties inherit: **`color`** and **`font-size`**. A node
without a matching declaration takes the parent's computed value (roots
start from the initial value). Every other property is non-inherited —
`display`, `width`, `background-color`, and the flex properties never pass
to children.

## Documented divergences from CSS

These are deliberate (G-UPG-04) and must stay in sync with the implementation:

- **No margin collapsing.** Adjacent sibling/parent-child margins never merge;
  the layout adds them.
- **Always content-box.** `box-sizing` does not exist; `width` is the content
  box (the CSS initial value, minus the ability to change it).
- **No `overflow`.** Content that does not fit is drawn outside its box; there
  is no scroll container, no clipping, and no scroll events in M2/M3 (see
  [EVENTS.md](EVENTS.md) §divergences).
- **Text nodes are block-level boxes.** There is no inline flow: sibling text
  and elements stack vertically, and a word wider than its content box stays
  on one line (it is not broken).
- **No layout-affecting properties beyond the table**: `position`, `float`,
  `z-index`, `border-radius`, `min-*`/`max-*`, `line-height`, `display:inline`
  block formatting (the `inline` display value exists in the type system for
  forward compatibility but is rejected at resolve until implemented).
- **Percentage height is rejected** (CSS would treat it as `auto` for
  auto-height parents; rejecting is explicit, treating it as auto silently is
  not).
- **Only three pseudo-classes exist** (`:hover`, `:active`, `:focus`);
  state bits come from input ([EVENTS.md](EVENTS.md)), there is no focus
  ring (`outline` does not exist) and no cursor/`pointer` property.

## Error contract

- `style.Parse` / `style.ParseInline` return a wrapped error containing the
  1-based line and column of the offending token.
- `style.Resolve` returns a wrapped error naming the node and the property.
- Callers that surface a failure use `observe.Diagnostic{Component: "style"}`.
