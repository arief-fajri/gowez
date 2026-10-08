/**
 * The M5 supported subset, in one place.
 *
 * Every list here is a contract: an item is either compiled or produces a
 * finding code. The subset is published in docs/SVELTE.md and the gap register
 * is generated from the report-mode findings, so changing a list without
 * updating that document is a contract drift (G-IFACE-04, G-UPG-04).
 */

/** Elements the subset compiles. Block/flex only: no inline flow, no tables. */
export const ELEMENTS = new Set([
  'div',
  'span',
  'p',
  'section',
  'article',
  'aside',
  'nav',
  'header',
  'footer',
  'main',
  'ul',
  'ol',
  'li',
  'h1',
  'h2',
  'h3',
  'h4',
  'button',
  'label',
  'code',
  'form',
  'input',
]);

/** Elements that are focusable by default in the subset. */
export const FOCUSABLE = new Set(['button', 'input']);

/** Attributes passed through verbatim. No a11y tree is built (divergence). */
export const ATTRIBUTES = new Set([
  'id',
  'class',
  'tabindex',
  'type',
  'placeholder',
  'novalidate',
  'aria-label',
  'aria-hidden',
  'aria-current',
  'role',
]);

/** Attributes whose value must be a literal, not an expression. */
export const BOOLEAN_ATTRIBUTES = new Set(['novalidate']);

/**
 * Event map: Svelte's `on*` attribute → protocol event name. An event outside
 * this table is a finding, never a silently ignored handler.
 */
export const EVENTS: Record<string, string> = {
  onclick: 'click',
  onpointerdown: 'pointer-down',
  onpointerup: 'pointer-up',
  onpointermove: 'pointer-move',
  onkeydown: 'key-down',
  onkeyup: 'key-up',
  oninput: 'textinput',
};

/**
 * `bind:value` maps to the committed-text event. The value lives in JS state
 * and reaches the tree through ordinary ops; there is no two-way binding on the
 * Go side.
 */
export const BINDS: Record<string, string> = {
  value: 'textinput',
};

/** Runes the subset compiles. Anything else is a SVELTE-RUNE finding. */
export const RUNES = new Set(['$state', '$derived', '$effect', '$props']);

/** Lifecycle helpers imported from the bare "svelte" module. */
export const LIFECYCLE = new Set(['onMount', 'onDestroy', 'tick']);

/**
 * Browser globals the adapter rejects wherever they appear. The sandbox does
 * not provide them (docs/SCRIPT.md), so referencing one is a compile-time
 * failure rather than a runtime ReferenceError.
 */
export const BROWSER_GLOBALS = new Set([
  'document',
  'window',
  'location',
  'history',
  'navigator',
  'localStorage',
  'sessionStorage',
  'fetch',
  'XMLHttpRequest',
  'setTimeout',
  'setInterval',
  'clearTimeout',
  'clearInterval',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'queueMicrotask',
  'console',
  'process',
  'HTMLElement',
  'Element',
  'Node',
  'CustomEvent',
  'SubmitEvent',
  'HTMLInputElement',
  'HTMLSelectElement',
  'Event',
  'Promise',
  'require',
  'module',
]);

/**
 * The single mutation door emitted into every bundle (decision D-1). The Go
 * runtime registers `ui.apply` and nothing else on the bundle path, so every
 * mutation of a mounted UI is an observable instruction batch.
 */
export const APPLY_METHOD = 'ui.apply';

/** `handlerId` → JS handler name convention, documented in docs/SVELTE.md. */
export function handlerName(handlerId: number): string {
  return `h${handlerId}`;
}

/**
 * scopeOf derives the component style-scoping class for a module.
 *
 * Svelte scopes each component's styles to the elements that component
 * renders. Without it, two components declaring the same selector name bleed
 * into each other — measured on the dashboard sample: 8 selectors declared in
 * more than one module (`.search` 3×, `.bar` 2×, …). The leak grows with app
 * size, so it is fixed at the adapter rather than per sample.
 *
 * The class is derived from the path rather than a hash of anything, so a given
 * module always gets the same scope and the committed bundle does not churn.
 */
export function scopeOf(file: string): string {
  const base = file
    .replace(/\.svelte$/, '')
    .replace(/^src\//, '')
    .replace(/[^a-zA-Z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return `s-${base || 'component'}`;
}

/**
 * scopesCollide reports whether two modules derive the same scope class.
 *
 * Two paths can sanitize to the same token (`a-b.svelte` and `a_b.svelte`).
 * That would merge their styles back together — the exact bug scoping exists to
 * prevent — so it is a build error rather than a silent merge.
 */
export function scopesCollide(files: readonly string[]): string[] {
  const byScope = new Map<string, string[]>();
  for (const file of files) {
    const scope = scopeOf(file);
    const existing = byScope.get(scope);
    if (existing) existing.push(file);
    else byScope.set(scope, [file]);
  }
  return [...byScope.values()]
    .filter((group) => group.length > 1)
    .map((group) => group.join(' and '));
}

/** Instruction schema version emitted into the manifest (must match Go). */
export const INSTRUCTION_VERSION = 1;

/** manifest.json schemaVersion (must match BundleManifestSchemaVersion in Go). */
export const MANIFEST_SCHEMA_VERSION = 1;

/** Svelte major version this adapter was built against. */
export const SVELTE_MAJOR = 5;
