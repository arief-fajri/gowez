/**
 * Component style scoping.
 *
 * Without scoping, a rule in one module matches elements rendered by another:
 * measured on the dashboard sample, 8 selectors were declared in more than one
 * module. The leak grows with app size, so it is fixed in the adapter rather
 * than in any one sample. These tests pin the two halves of the contract — the
 * scope class on every selector, and the same class on every element — because
 * scoping works only when both are present.
 */
import { describe, expect, it } from 'vitest';
import { project } from './helpers.js';
import { build, cssToText, report } from '../src/index.js';
import { scopeOf, scopesCollide } from '../src/subset.js';

const PARENT = `
<script>import Child from './UserList.svelte';</script>
<section class="shell">
  <button class="button">save</button>
  <Child />
</section>
<style>
  .shell { background-color: #111827; }
  button { padding: 6px 10px; }
  .row:hover { color: #ffffff; }
  .shell .row { display: flex; }
</style>
`;

const CHILD = `
<ul class="list">
  <li class="item">one</li>
  <li>two</li>
</ul>
<style>
  button { padding: 4px 8px; }
  .item { color: #22c55e; }
  li { display: block; }
</style>
`;

/** bundleTextOf builds the two-module fixture and returns its CSS text. */
function bundleCss(): string {
  const root = project({ 'src/App.svelte': PARENT, 'src/UserList.svelte': CHILD });
  return cssToText(build({ rootDir: root, entry: 'src/App.svelte' }).css);
}

/**
 * classListsOfEveryElement reads the class list of every element in the bundle.
 *
 * The assertions read the emitted bundle rather than `result.ops`, because the
 * runtime executes app.js and never the ops batch: pinning the ops would test a
 * build artifact nothing runs, and would let the two paths drift apart silently.
 */
function classListsOfEveryElement(root: string, entry: string): string[] {
  const bundle = build({ rootDir: root, entry }).bundle;
  return [...bundle.matchAll(/"class": \[([^\]]*)\]/g)].map((m) =>
    m[1]!
      .split(',')
      .map((part) => part.trim().replace(/^"|"$/g, '').trim())
      .filter(Boolean)
      .join(' '),
  );
}

describe('style scope derivation', () => {
  it('derives a stable class from the module path', () => {
    expect(scopeOf('src/App.svelte')).toBe('s-App');
    expect(scopeOf('src/UserList.svelte')).toBe('s-UserList');
    expect(scopeOf('src/pages/Settings.svelte')).toBe('s-pages-Settings');
  });

  it('reports two modules that sanitize to the same class', () => {
    expect(scopesCollide(['src/App.svelte', 'src/Child.svelte'])).toEqual([]);
    expect(scopesCollide(['src/a-b.svelte', 'src/a_b.svelte'])).toEqual([
      'src/a-b.svelte and src/a_b.svelte',
    ]);
  });

  it('fails the build on a scope collision instead of merging styles', () => {
    const root = project({
      'src/App.svelte': `<script>import A from './a-b.svelte'; import B from './a_b.svelte';</script>
<A /><B />`,
      'src/a-b.svelte': '<p class="x">one</p>',
      'src/a_b.svelte': '<p class="x">two</p>',
    });
    const findings = report(root, 'src/App.svelte').findings;
    expect(findings.map((f) => f.code)).toContain('CSS-SCOPE-COLLISION');
  });
});

describe('scoped selectors', () => {
  it('appends the scope to the last compound of every selector', () => {
    const css = bundleCss();
    // The subject compound is scoped...
    expect(css).toMatch(/\.shell\.s-App \{/);
    expect(css).toMatch(/button\.s-App \{/);
    // ...including when a pseudo-class follows it.
    expect(css).toMatch(/\.row\.s-App:hover/);
    // ...and an earlier compound in a descendant chain is not scoped: the last
    // compound is the subject, and scoping the subject is enough.
    expect(css).toMatch(/\.shell \.row\.s-App \{/);
  });

  it('gives each module its own scope so rules cannot cross over', () => {
    const css = bundleCss();
    // `button` is declared by both modules; the two rules must stay separate.
    expect(css).toMatch(/button\.s-App \{[^}]*padding: 6px 10px/);
    expect(css).toMatch(/button\.s-UserList \{[^}]*padding: 4px 8px/);
  });

  it('never puts both scopes in one rule', () => {
    for (const rule of bundleCss().split('}')) {
      const scopes = [...new Set([...rule.matchAll(/s-[A-Za-z0-9-]+/g)].map((m) => m[0]))];
      if (scopes.length === 0) continue;
      expect(scopes).toHaveLength(1);
    }
  });

  it('prints a descendant combinator as a space, not as an AST node', () => {
    const css = bundleCss();
    // The combinator is a node in the Svelte AST, so coercing it to a string
    // used to emit the literal text `[object Object]` — a selector Go cannot
    // match, so the whole rule was dropped without a finding.
    expect(css).not.toContain('[object Object]');
    expect(css).toMatch(/\.shell \.row\.s-App \{/);
  });

  it('names an unsupported combinator instead of emitting it', () => {
    for (const [combinator, phrase] of [
      ['>', 'child combinator'],
      ['+', 'adjacent-sibling'],
      ['~', 'general-sibling'],
    ]) {
      const root = project({
        'src/App.svelte': `
<section class="shell"><div class="row">x</div></section>
<style>
  .shell ${combinator} .row { display: flex; }
</style>
`,
      });
      const findings = report(root, 'src/App.svelte').findings;
      expect(findings.map((f) => f.code)).toEqual(['CSS-SELECTOR']);
      expect(findings[0]!.message).toContain(phrase);
    }
  });
});

describe('scoped elements', () => {
  it('puts the scope on every element, including those with no class', () => {
    const root = project({ 'src/App.svelte': CHILD });
    const classes = classListsOfEveryElement(root, 'src/App.svelte');
    // ul, li.item, li — the last one has no author class at all.
    expect(classes).toHaveLength(3);
    for (const classList of classes) {
      expect(classList.split(/\s+/)).toContain('s-App');
    }
    // No leading space from the empty author class.
    expect(classes.every((c) => c === c.trim())).toBe(true);
  });

  it('keeps the author class alongside the scope class', () => {
    const root = project({ 'src/App.svelte': PARENT, 'src/UserList.svelte': CHILD });
    const classes = classListsOfEveryElement(root, 'src/App.svelte');
    expect(classes).toContain('shell s-App');
    expect(classes).toContain('button s-App');
    // The child renders its own elements with its own scope.
    expect(classes).toContain('list s-UserList');
    expect(classes).toContain('item s-UserList');
  });

  it('scopes each module to its own elements only', () => {
    const root = project({ 'src/App.svelte': PARENT, 'src/UserList.svelte': CHILD });
    const bundle = build({ rootDir: root, entry: 'src/App.svelte' }).bundle;
    // The App factory must not stamp its scope onto the child's elements.
    const appFactory = bundle.slice(bundle.indexOf('function _App'), bundle.indexOf('function _UserList'));
    const userFactory = bundle.slice(bundle.indexOf('function _UserList'));
    expect(appFactory).not.toContain('s-UserList');
    expect(userFactory).not.toContain('s-App');
  });
});

describe('selector gap reporting', () => {
  it('names the unsupported pseudo-class instead of blaming the printer', () => {
    const root = project({
      'src/App.svelte': `
<ul>
  <li class="a">one</li>
  <li class="b">two</li>
</ul>
<style>
  li:last-child { color: #ffffff; }
</style>
`,
    });
    const findings = report(root, 'src/App.svelte').findings;
    // "could not be printed" reads like an adapter bug; the truth is that
    // :last-child is outside the subset.
    expect(findings).toHaveLength(1);
    expect(findings[0]!.code).toBe('CSS-SELECTOR');
    expect(findings[0]!.message).toMatch(/:last-child is not in the CSS subset/);
  });

  it('names the unsupported pseudo-element', () => {
    const root = project({
      'src/App.svelte': `
<input class="search" />
<style>
  .search::placeholder { color: #999999; }
</style>
`,
    });
    const findings = report(root, 'src/App.svelte').findings;
    expect(findings.map((f) => f.code)).toEqual(['CSS-SELECTOR']);
    expect(findings[0]!.message).toMatch(/::placeholder is not in the CSS subset/);
  });

  it('rejects attribute selectors even though the printer could print them', () => {
    const root = project({
      'src/App.svelte': `
<p id="note">one</p>
<style>
  [id] { color: #ffffff; }
</style>
`,
    });
    const findings = report(root, 'src/App.svelte').findings;

    // Printing [id] would let a strict build pass and then fail Go's
    // stylesheet parser at startup, so the adapter rejects it by name instead.
    expect(findings.map((f) => f.code)).toEqual(['CSS-SELECTOR']);
    expect(findings[0]!.message).toMatch(/attribute selector \[id\] is not in the CSS subset/);
  });

  it('rejects universal selectors before Go has to reject the stylesheet', () => {
    const root = project({
      'src/App.svelte': `
<p>one</p>
<style>
  * { color: #ffffff; }
</style>
`,
    });
    const findings = report(root, 'src/App.svelte').findings;
    expect(findings.map((f) => f.code)).toEqual(['CSS-SELECTOR']);
    expect(findings[0]!.message).toMatch(/universal selector \* is not in the CSS subset/);
  });
});
