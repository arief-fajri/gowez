# Svelte Adapter Contract (M5)

**Status:** delivered. The compile pipeline, the instruction applier, the bundle
loader, text input and reactivity are implemented and tested.

Authoritative for `@gowez/adapter`: the pipeline (strict and report modes), the
bundle layout, the supported subset, the rejection catalog, the
`handlerId ↔ h<id>` convention, the generated **Gap register**, the documented
divergences, and the supported Svelte range.

---

## Pipeline

```text
parse each module (TypeScript stripped during the walk, decision D-2)
  → validate subset
  → emit instruction ops + component CSS
  → esbuild → single-file IIFE (no imports, es2015)
  → dist/{manifest.json, app.js, styles.css}
```

Svelte's client runtime never executes in the Go process
([DRR-007](../evidence/records/2026-10-07_m5-compile-strategy.md)). The
adapter owns the AST; the Go runtime owns layout and painting.

### Two modes

| Mode | Behavior | Used by |
|---|---|---|
| `strict` (default) | any finding → `CompileError{file, line, column, code}`, build fails, nothing written | application builds |
| `report` | walks everything, returns findings as data, always exits 0 | the gap register below |

```bash
npm run build -w @gowez/example-gowez-dashboard   # strict
npm run report -w @gowez/example-gowez-dashboard  # report
```

## Bundle layout (DRR-006)

```text
dist/
├── manifest.json  {schemaVersion:1, adapter:{name,version}, svelte:"5", script, styles}
├── app.js         single-file IIFE — one mount, one instruction batch
└── styles.css     component CSS, validated against docs/CSS-SUBSET.md
```

The mount is **one atomic batch** containing every module's ops in dependency
order. Splitting it per module would let the runtime root a child component's
wrapper before its parent attached it — a node cannot be both a root and a child.

## The single mutation door (decision D-1)

The bundle path registers **exactly one** UI mutation method: `ui.apply`.
`ui.setText` belongs to the demo scene and is *not* registered when a bundle is
loaded, so every mutation of a mounted UI is an observable instruction batch
(G-SEC-01). Pinned by `TestBundleSceneRegistersOnlyUIApply`.

Handler convention: an `addEventListener` op carries a `handlerId`; the Go side
fires the JS handler registered as `h<handlerId>`.

## Supported subset (strict mode must produce zero findings)

### Elements

`div` `span` `p` `section` `article` `aside` `nav` `header` `footer` `main`
`ul` `ol` `li` `h1`–`h4` `button` `label` `code` `form` `input`

Block/flex only. `button` and `input` receive `tabindex="0"` automatically.

### Attributes

`id` `class` `tabindex` `type` `placeholder` `novalidate` `aria-label`
`aria-hidden` `aria-current` `role`

### Events

| Svelte | Protocol event |
|---|---|
| `onclick` | `click` |
| `onpointerdown` / `onpointerup` / `onpointermove` | `pointer-down` / `pointer-up` / `pointer-move` |
| `onkeydown` / `onkeyup` | `key-down` / `key-up` |
| `oninput`, `bind:value` | `textinput` |

### Directives

`class:`, `bind:value` (on `<input>` only), relative component composition with
props and callback props.

### CSS

The subset in [docs/CSS-SUBSET.md](CSS-SUBSET.md). The adapter mirrors it in
TypeScript so a build fails before a bundle exists; `css-parity.test.ts` runs the
**real Go parser** and compares verdict-for-verdict, because a mirror that
silently disagrees is worse than no mirror.

### Component style scoping

**Each module's `<style>` is scoped to the elements that module renders.** The
adapter derives one scope class per module from its path (`src/UserList.svelte`
→ `s-UserList`), then:

- appends it to the **last compound** of every selector — the subject — and
- appends it to the `class` of every element the module renders.

```
UserList.svelte   button { padding: 4px 8px }   →   button.s-UserList
                  <li class="item">              →   class="item s-UserList"

App.svelte        button { padding: 6px 10px }  →   button.s-App
                  <button class="button">        →   class="button s-App"
```

Both halves are required. The class on the rule alone would match nothing; the
class on the element alone would let every module's rules apply to every element.

Why it matters: without scoping a rule matches elements it did not author.
Measured on the dashboard sample, **8 selectors were declared by more than one
module** (`.search` 3×, `.bar` 2×, `.nav-item` 2×, …), so the last rule silently
won for all of them. After scoping, 0 author selectors resolve to more than one
rule. The leak grows with app size, which is why it is fixed in the adapter
rather than worked around in any sample.

Rules:

- the scope goes on the **last compound**, so `.row` becomes `.row.s-App` and
  `.row:hover` becomes `.row.s-App:hover` — before the pseudo-class, not after.
- an earlier compound in a descendant chain is left alone: `.shell .row` becomes
  `.shell .row.s-App`. Scoping the subject is enough to stop cross-module
  matching.
- the scope is **appended**, never substituted: an author class always survives.
- two modules that sanitize to the same class fail the build with
  `CSS-SCOPE-COLLISION`, because a shared scope is the one bug scoping exists to
  prevent.
- `internal/style` needs no change: its selector parser already accepts several
  classes on one compound.

`TestBundleStylesAreScoped` (tests/golden) verifies this end-to-end through
`internal/style`: it reads the committed bundle and asserts each `<button>`
resolves to the padding declared by *its own* module.

## Reactivity model

**A full re-render per state change, diffed against the previous node tree by
node id.** There is no virtual DOM: the Go runtime already owns layout and
painting, so a JS-side DOM would duplicate that work rather than share it.

```
state write → __flush() → render() → fresh node tree
            → diff against the mounted tree
            → one instruction batch through ui.apply
```

- **Deterministic.** The same state always yields the same op stream, which the
  golden test and the op-count metrics both rely on (Module 6).
- **Atomic.** One flush is one batch; a rejected batch leaves the previous tree
  exactly as it was (invariant I1).
- **Synchronous.** There is no scheduler — the runtime has no timers — so a
  state change paints in the same JS turn that caused it.

### What each construct compiles to

| Construct | Compiles to |
|---|---|
| `$state(x)` | a runtime cell; every read unwraps it, every write schedules a flush |
| `$derived(expr)` | a getter recomputed on each render — no caching, because a subset this small does not need one |
| `$effect(fn)` | registered and run after every flush |
| `$props()` | the factory's `props` argument, with defaults applied only when a prop is absent |
| `{#if}` / `{:else}` | a ternary — only the taken arm becomes a node |
| `{#each}` | a keyed loop; the runtime matches by key, so a reorder emits nothing and an insert adds one node |
| `{#key}` | transparent. It is a remount hint in Svelte; the subset re-renders from state anyway, so it is a documented no-op rather than a faked remount |
| `bind:value` | a `textinput` handler that assigns the committed text back to the state cell |
| `on*` | a handler attached at node creation; an unchanged node keeps its listener across a re-render |

### Proof

[`reactivity_test.go`](../internal/app/reactivity_test.go) drives the **committed
slice bundle** — the artifact the adapter actually produced, not a fixture that
copies the runtime — through the real path (manifest → CSS → sandbox eval →
`ui.apply` → layout):

- `TestReactivityMountRendersCurrentState` — one `{#if}` arm, one node per list item
- `TestReactivityStateChangeUpdatesUI` — click → state → re-render → diff reaches the tree
- `TestReactivityListUpdateRemovesOnlyOne` — removing one row of three emits a bounded diff
- `TestReactivityFilteredListReactsToInput` — platform text → handler → state → list shrinks
- `TestReactivityRepeatClicksStayBounded` — five clicks add no nodes and one batch each
- `TestReactivityRepaintAfterStateChange` — the updated tree relayouts and repaints

### Known divergences

- **Whole-tree re-render.** A state write re-runs the render function, so a
  component with many siblings does work proportional to its own size. The diff
  keeps the *instruction* count small; it does not make the render cheap.
- **No dependency tracking.** Every render reads every cell it touches, so a
  write to one counter re-renders the whole app. Precise tracking would add a
  graph the subset does not need; for a runtime this size it is the right trade,
  but it is a trade.
- **`{#key}` is a no-op** (see above).

## Gap register

Generated from `report` mode over `examples/dashboard` — the browser test bed,
deliberately written as a full admin dashboard rather than to the subset. Every
unsupported construct becomes a finding with a file, a line and a stable code.

```bash
npm run report:dashboard          # counts by code
npm run report:dashboard -- --json  # the full register
```

The sample is out of the subset by design, so this table is the honest statement
of what a M5 application may not use.

**Current baseline: 197 findings across 7 modules** (123 `CSS-PROPERTY`), re-measured
2026-10-08 after the register was corrected — see below. This number is
why it is a script rather than prose: it is the candidate acceptance metric for
the follow-on milestones, proposed in
[DRR-008](../evidence/records/2026-10-08_dashboard-target.md) (confirmed 2026-10-08). Note what it does **not** measure — it counts *rejections*, so a
construct that compiles to wrong output is invisible to it (component style
scoping was such a defect; see §Component style scoping).

| Code | Count | Category | What it rejects |
|---|---|---|---|
  | `ELEMENT-REJECTED` | 11 | element | `<table>` `<tr>` `<td>` `<select>` `<option>` `<strong>` and any element outside the list above |
  | `ELEMENT-UNSUPPORTED` | 12 | element | an attribute outside the list above |
  | `SVELTE-TRANSITION` | 2 | svelte | `in:fade`, `transition:*`, `animate:*`, `svelte/transition`, `svelte/animate` |
  | `SVELTE-HEAD` | 6 | svelte | `<svelte:head>`, `<title>` — there is no document |
  | `SVELTE-WINDOW` | 0 | svelte | `svelte:window`, `svelte:document`, `svelte:body` |
  | `SVELTE-SLOT` | 0 | svelte | slots, snippets, `{@render}` |
  | `SVELTE-SPREAD` | 0 | svelte | `{...spread}` |
  | `SVELTE-AWAIT` | 0 | svelte | `{#await}` — no promise exists |
  | `SVELTE-RAW-HTML` | 0 | svelte | `{@html}` |
  | `SVELTE-RUNE` | 0 | svelte | any rune outside `$state` `$derived` `$effect` `$props` |
  | `SVELTE-IMPORT` | 6 | svelte | bare modules, non-`.svelte` relative imports, unresolved components |
  | `SVELTE-UNSUPPORTED-NODE` | 2 | svelte | a node type outside the subset; also an event outside the event table |
  | `DOM-GLOBAL` | 29 | dom | `document` `window` `location` `history` `navigator` `setTimeout` `setInterval` `fetch` `console` `process` and the DOM constructor set |
  | `DOM-API`, `DOM-FUNCTION` | 0 | dom | a call to one of the above |
  | `CSS-PROPERTY` | 123 | css | a property or value outside the CSS subset — grid, `overflow-*`, `border-radius`, `box-shadow`, `transition`, `position`, `z-index`, percentage heights. `var()` and `color-mix()` are **supported** since M6a |
|`CSS-AT-RULE` | 2 | css | `@media`, `@supports`, `@keyframes` — the subset has none |
|`CSS-SELECTOR` | 4 | css | a selector outside the subset, named explicitly: an unsupported pseudo-class (`:last-child`, `:nth-child`, …), a pseudo-element (`::placeholder`), or a combinator other than descendant (`>`, `+`, `~`) |
|`CSS-SCOPE-COLLISION` | 0 | css | two modules derive the same style scope, which would merge their styles back together |
|`CSS-UNKNOWN` | 0 | css | a declaration that could not be validated against the subset |

`CSS-SELECTOR` replaced a `CSS-UNKNOWN` message that read *"selector could not be
printed"*. That wording blamed the adapter for a decision the subset had made:
a `:last-child` rule is **outside the subset**, not mis-printed. Naming the
construct is the whole point of a gap register — an author has to be able to tell
"the adapter is broken" from "this is not supported yet" (G-UPG-04).

Every code carries a published remedy in
[`findings.ts`](../packages/adapter/src/findings.ts), and the codes are closed:
an unknown code is a bug in the adapter, not new behavior.

## Divergences from browser and Svelte semantics

- **No DOM.** The sandbox provides `gowez.invoke` / `gowez.call` / `gowez.on`
  only ([docs/SCRIPT.md](SCRIPT.md)). Every browser global is a compile error.
- **No accessibility tree.** `aria-*` and `role` pass through as attributes so
  authored markup survives, but nothing consumes them.
- **No inline flow.** The subset lays out block and flex. `<strong>` inside text
  is an element rejection, not an inline box.
- **Text is committed, never derived from keys.** `textinput` carries text; the
  runtime never reconstructs characters from keycodes.
- **No caret, no selection, no IME candidate window.** IME preedit renders in the
  value mirror; the candidate window belongs to the platform UI and is out of
  scope ([docs/EVENTS.md](EVENTS.md) §text input).
- **Deletion is the application's job.** Backspace arrives as `key-down`; the
  JS handler rewrites the value and submits ops.
- **One file, no modules.** goja has no module system; `app.js` is a single IIFE
  at `es2015` (bounded by measured goja gaps: `structuredClone`, `Object.groupBy`,
  `performance`, `WeakRef` are absent). "Modules" here means *JavaScript* modules:
  `.svelte` components are compiled into that one file as separate factory
  functions, each with its own scope class.
- **Scoping is class-based, and the scope is observable.** Svelte implements
  component scoping by adding a hash class to elements; the adapter does the same
  with a readable path-derived class (`s-App`, `s-UserList`) so a stylesheet can
  be read and debugged by hand. The visible consequence: **every element carries
  one extra class token**, and a class must be matched per token rather than by
  comparing the whole attribute — use `ui.Node.HasClass`, not `== "class"`.
- **Only the descendant combinator.** `:hover`, `:active` and `:focus` match;
  `:last-child`, `:nth-child`, `::placeholder` and the `>`/`+`/`~` combinators do
  not, because `internal/style` matches a selector as a whitespace-separated
  chain of compounds. Each is reported as `CSS-SELECTOR` by name.

## Svelte compatibility

Built and tested against **svelte 5.57.2**; the manifest records the major
version and the runtime refuses a mismatch at startup (G-IFACE-05, G-UPG-02).

The adapter depends on the **AST** shape (`parse({modern: true})`), not on
Svelte's client internals — which is why a Svelte upgrade is a compatibility
review rather than a rewrite (see DRR-007's measured rejection of Strategy A).

TypeScript is stripped by the adapter itself: `parse()` retains `TSTypeAnnotation`
and `TSInterfaceDeclaration`, and `remove_typescript_nodes` is unexported and
deep-import-blocked. The strip set is closed — an unrecognized TS shape fails the
build rather than reaching codegen.

## Error catalog

| Situation | Result |
|---|---|
| unsupported construct | `CompileError` with `file:line:column` and a code |
| CSS outside the subset | `CompileError` (`CSS-PROPERTY` / `CSS-AT-RULE`) |
| missing `manifest.json` / `app.js` / `styles.css` | explicit startup failure, never an empty window |
| manifest `schemaVersion` or `svelte` mismatch | explicit startup failure |
| CSS that Go rejects at parse time | `style: line X:Y` diagnostic |
| mount script throws or exceeds the eval budget | startup aborts (G-REL-01, I12) |
| op batch over the inline limit | `-32602` to JavaScript, nothing applied |
| invalid op batch | rejected atomically, counted in `UIOpsRejected`, reported as `Diagnostic{Component:"ui"}` |

## Evidence

- Pipeline and subset: [`packages/adapter/test`](../packages/adapter/test)
  (compile, reject, typescript, bundle, css-parity, gap-report).
- Applier atomicity: [`apply_test.go`](../internal/ui/apply_test.go).
- Mount, click, text input, surface: [`bundle_test.go`](../internal/app/bundle_test.go),
  [`input_text_test.go`](../internal/app/input_text_test.go).
- Pixels and determinism: [`golden_test.go`](../tests/golden/bundle_test.go),
  [`gowez-dashboard.png`](../tests/golden/testdata/gowez-dashboard.png).
- Strategy decision: [DRR-007](../evidence/records/2026-10-07_m5-compile-strategy.md),
  [spike record](../evidence/experiments/2026-10-07_m5_compile-strategy-spike.md).
