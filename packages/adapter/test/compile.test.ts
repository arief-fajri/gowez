/**
 * Subset compilation: the in-subset fixtures must compile with zero findings and
 * emit a well-formed op stream. This is the "Svelte component can be compiled"
 * checklist item.
 */
import { describe, expect, it } from 'vitest';
import { buildFixture, project } from './helpers.js';
import {
  compositionFixture,
  conditionalFixture,
  counterFixture,
  derivedFixture,
  keyFixture,
  lifecycleFixture,
} from './fixtures.js';
import { build, cssToText, report } from '../src/index.js';
import { INSTRUCTION_VERSION } from '../src/subset.js';

const SUBSET_FIXTURES: Array<[string, string]> = [
  ['counter', counterFixture],
  ['conditional + each + class:', conditionalFixture],
  ['composition', compositionFixture],
  ['key + bind:value', keyFixture],
  ['lifecycle', lifecycleFixture],
  ['derived', derivedFixture],
];

describe('strict build of in-subset fixtures', () => {
  for (const [name, source] of SUBSET_FIXTURES) {
    it(`${name} compiles with zero findings`, () => {
      const extra: Record<string, string> =
        name === 'composition' ? { 'src/Child.svelte': childOfComposition() } : {};
      const result = buildFixture(source, extra);
      expect(result.findings).toEqual([]);
      expect(result.ops.length).toBeGreaterThan(0);
      expect(result.entry).toBeGreaterThan(0);
    });
  }

  it('emits ops that satisfy the instruction schema shape', () => {
    const result = buildFixture(counterFixture);
    for (const op of result.ops) {
      expect(op.kind).toMatch(
        /^(createElement|createText|setAttribute|setStyle|setText|appendChild|removeChild|addEventListener|removeEventListener)$/,
      );
      // nodeId 0 is the schema's "absent" value; every op that has one must be
      // at least 1 (schema minimum).
      if (op.nodeId !== undefined) expect(op.nodeId).toBeGreaterThanOrEqual(1);
      if (op.parentId !== undefined) expect(op.parentId).toBeGreaterThanOrEqual(1);
      if (op.childId !== undefined) expect(op.childId).toBeGreaterThanOrEqual(1);
      if (op.handlerId !== undefined) expect(op.handlerId).toBeGreaterThanOrEqual(1);
    }
  });

  it('manifest matches the Go-side contract', () => {
    const result = buildFixture(counterFixture);
    expect(result.manifest).toEqual({
      schemaVersion: 1,
      adapter: { name: 'gowez-adapter', version: '1.0.0' },
      svelte: 5,
      script: 'app.js',
      styles: 'styles.css',
    });
  });

  it('the bundle uses only the documented host surface', () => {
    const result = buildFixture(counterFixture);
    // The runtime prelude plus module code. Only gowez.* may reach the host.
    expect(result.bundle).toContain('gowez.invoke');
    for (const banned of ['document', 'window.', 'setTimeout', 'require(', 'process.']) {
      expect(result.bundle).not.toContain(banned);
    }
  });

  it('emits an addEventListener op for onclick with a handler id', () => {
    const result = buildFixture(counterFixture);
    const click = result.ops.find((op) => op.kind === 'addEventListener');
    expect(click).toBeDefined();
    expect(click?.event).toBe('click');
  });

  it('maps bind:value on an input to the textinput event', () => {
    const result = buildFixture(keyFixture);
    const events = result.ops
      .filter((op) => op.kind === 'addEventListener')
      .map((op) => op.event);
    expect(events).toContain('textinput');
  });

  it('marks focusable elements with tabindex so focus traversal can reach them', () => {
    const result = buildFixture(counterFixture);
    const tabindex = result.ops.filter(
      (op) => op.kind === 'setAttribute' && op.name === 'tabindex',
    );
    expect(tabindex.length).toBeGreaterThan(0);
  });

  it('keeps the selector on every CSS rule so Go can resolve it', () => {
    const result = buildFixture(counterFixture);
    expect(result.css.length).toBeGreaterThan(0);
    for (const rule of result.css) {
      // A bare identifier in a component stylesheet is a class, matching how
      // Svelte authors `shell { }` for `<section class="shell">`.
      expect(rule.selector).toMatch(/^[a-z.]/);
      expect(rule.declarations.length).toBeGreaterThan(0);
    }
    // `panel { }` on a <section class="panel"> must print as a class selector,
    // or the rule matches nothing and the layout silently collapses. The scope
    // class rides along on the subject compound, so `.panel` becomes `.panel.s-App`.
    expect(result.css.map((r) => r.selector)).toContain('.panel.s-App');
    expect(result.css.map((r) => r.selector)).toContain('button.s-App');
    const properties = result.css.flatMap((r) => r.declarations.map((d) => d.property));
    expect(properties).toContain('background-color');
    expect(properties).toContain('padding');
  });

  it('renders rules as CSS text with selectors intact', () => {
    const result = buildFixture(counterFixture);
    const css = cssToText(result.css);
    expect(css).toMatch(/button\.s-App \{/);
    expect(css).toMatch(/background-color: #2f6feb;/);
    // Declarations must never appear without a selector above them.
    expect(css.trimStart().startsWith('background-color')).toBe(false);
  });

  it('compiles a child module before its importer', () => {
    const root = project({
      'src/App.svelte': compositionFixture,
      'src/Child.svelte': childOfComposition(),
    });
    const result = build({ rootDir: root, entry: 'src/App.svelte' });
    expect(result.findings).toEqual([]);

    // The child's ops must precede the entry module's, so a single batch can be
    // applied in stream order: the parent can only attach a child that already
    // exists.
    const entryOpIndex = result.ops.findIndex((op) => op.nodeId === result.entry);
    expect(entryOpIndex).toBeGreaterThan(0);
    expect(result.ops[0]!.kind).toBe('createElement');
    expect(result.ops[0]!.nodeId).not.toBe(result.entry);

    // Exactly one tree root: the child's wrapper is attached by the parent, not
    // rooted on its own.
    const roots = result.ops.filter((op) => op.kind === 'createElement' && op.nodeId === result.entry);
    expect(roots).toHaveLength(1);
  });

  it('resolves components nested inside a {#key} block', () => {
    const root = project({
      'src/App.svelte': `<script>
  import Child from './Nested.svelte';
  let page = $state('one');
</script>
{#key page}
  <Child />
{/key}`,
      'src/Nested.svelte': `<p>child</p>`,
    });
    const result = build({ rootDir: root, entry: 'src/App.svelte' });

    // The static walk descends through {#key}; losing the module options there
    // used to make every nested component look unresolvable.
    expect(result.findings).toEqual([]);
    const attachment = result.ops.find(
      (op) =>
        op.kind === 'appendChild' &&
        op.parentId === result.entry &&
        op.childId !== result.entry,
    );
    expect(attachment).toBeDefined();
  });

  it('reports unmodelled expressions and statements at compile time', () => {
    const root = project({
      'src/App.svelte': `<script>
  let stamp = new Date();
  while (false) {
    stamp = new Date();
  }
</script>
<p>{stamp}</p>`,
    });
    const result = report(root, 'src/App.svelte');
    const codes = result.findings.map((finding) => finding.code);

    // The emitted bundle keeps a fail-closed marker, but strict mode must never
    // reach it: both shapes are compile-time findings.
    expect(codes).toContain('SVELTE-UNSUPPORTED-EXPRESSION');
    expect(codes).toContain('SVELTE-UNSUPPORTED-NODE');
  });

  it('treats explicit global CSS as an unscoped base layer', () => {
    const root = project({
      'src/App.svelte': `<p class="other">child</p>
<style>.other { color: #ffffff; }</style>`,
    });
    const result = report(root, 'src/App.svelte', {
      globalStyles: [
        {
          file: 'src/styles/ui.css',
          source: `body { margin: 0; }
.base { background-color: #ffffff; }
* { color: #ffffff; }`,
        },
      ],
    });

    // A caller-supplied global stylesheet is the base layer: its selectors are
    // printed exactly as written and come before component CSS. It also cannot
    // suppress its own gaps.
    expect(result.css.map((rule) => rule.selector)).toEqual([
      'body',
      '.base',
      '.other.s-App',
    ]);
    expect(result.findings.map((finding) => finding.code)).toEqual(['CSS-SELECTOR']);
    expect(result.findings[0]!.file).toBe('src/styles/ui.css');
    expect(result.findings[0]!.message).toMatch(/universal selector \* is not in the CSS subset/);
  });

  it('reuses one instruction version across the manifest and the runtime', () => {
    const result = buildFixture(counterFixture);
    expect(result.bundle).toContain(`var __version = ${INSTRUCTION_VERSION};`);
  });
});

/** childOfComposition mirrors fixtures.compositionFixture's import. */
function childOfComposition(): string {
  return `<script>
  let { items = [], onRemove = () => {} } = $props();
</script>
<article>
  {#each items as item (item.id)}
    <p>{item.label}</p>
  {/each}
  <button onclick={onRemove}>clear</button>
</article>`;
}
