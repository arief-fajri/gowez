/**
 * Compile fixtures for the M5 subset.
 *
 * Each fixture is a complete .svelte module that must produce zero findings in
 * strict mode, and whose op stream is pinned by a snapshot. The fixtures are the
 * executable definition of "the subset"; docs/SVELTE.md is generated from the
 * same information, and the two must not drift (G-IFACE-04).
 */

/** counterFixture: state, an event handler, and a dynamic text node. */
export const counterFixture = `
<script>
  let count = $state(0);
  function inc() { count += 1; }
</script>

<section class="panel">
  <h1>Counter</h1>
  <p>count: {count}</p>
  <button onclick={inc}>+1</button>
</section>

<style>
  panel { background-color: #202430; padding: 12px; }
  h1 { color: #ffffff; font-size: 22px; }
  button { background-color: #2f6feb; color: #ffffff; padding: 6px 10px; }
</style>
`;

/** conditionalFixture: {#if}/{:else} plus a class: directive. */
export const conditionalFixture = `
<script>
  let active = $state(true);
  let items = $state(['a', 'b']);
</script>

<nav>
  {#if active}
    <p>active</p>
  {:else}
    <p>inactive</p>
  {/if}
  <ul>
    {#each items as item, i (item)}
      <li class:active={i === 0}>{item}</li>
    {/each}
  </ul>
</nav>
`;

/** compositionFixture: a child component with props and a callback prop. */
export const compositionFixture = `
<script>
  import Child from './Child.svelte';
  let items = $state([{ id: 1, label: 'one' }]);
  function onRemove() { items = []; }
</script>

<section>
  <Child {items} onRemove={onRemove} />
</section>
`;

/** childFixture: the child of compositionFixture. */
export const childFixture = `
<script>
  let { items = [], onRemove = () => {} } = $props();
</script>

<article>
  {#each items as item (item.id)}
    <p>{item.label}</p>
  {/each}
  <button onclick={onRemove}>clear</button>
</article>
`;

/** keyFixture: {#key} remount and a bind:value input. */
export const keyFixture = `
<script>
  let page = $state('a');
  let query = $state('');
</script>

<section>
  {#key page}
    <p>page {page}</p>
  {/key}
  <input bind:value={query} placeholder="search" />
  <p>query: {query}</p>
</section>
`;

/** lifecycleFixture: onMount with a cleanup return. */
export const lifecycleFixture = `
<script>
  import { onMount } from 'svelte';
  let ready = $state(false);
  onMount(() => {
    ready = true;
    return () => { ready = false; };
  });
</script>

<section>
  <p>ready: {ready}</p>
</section>
`;

/** derivedFixture: $derived over a list. */
export const derivedFixture = `
<script>
  let items = $state([1, 2, 3]);
  let total = $derived(items.length);
</script>

<section>
  <p>total: {total}</p>
  {#each items as n (n)}
    <p>{n}</p>
  {/each}
</section>
`;

/**
 * Rejected fixtures, one per unsupported construct. Each must produce exactly
 * one finding with the expected code, which is the "unsupported Svelte/browser
 * behavior is explicit" checklist item.
 */
export const rejectedFixtures: Array<{ name: string; code: string; source: string }> = [
  {
    name: 'transition directive',
    code: 'SVELTE-TRANSITION',
    source: `<script>import { fade } from 'svelte/transition';</script>
<section transition:fade><p>x</p></section>`,
  },
  {
    name: 'svelte:head',
    code: 'SVELTE-HEAD',
    source: `<svelte:head><title>t</title></svelte:head><section><p>x</p></section>`,
  },
  {
    name: 'window handler',
    code: 'SVELTE-WINDOW',
    source: `<svelte:window onresize={() => {}} /><section><p>x</p></section>`,
  },
  {
    name: 'select element',
    code: 'ELEMENT-REJECTED',
    source: `<section><select><option>a</option></select></section>`,
  },
  {
    name: 'table element',
    code: 'ELEMENT-REJECTED',
    source: `<section><table><tr><td>a</td></tr></table></section>`,
  },
  {
    name: 'inline strong',
    code: 'ELEMENT-REJECTED',
    source: `<section><p>a <strong>b</strong></p></section>`,
  },
  {
    name: 'onsubmit',
    code: 'SVELTE-UNSUPPORTED-NODE',
    source: `<form onsubmit={(e) => {}}><p>x</p></form>`,
  },
  {
    name: 'document global',
    code: 'DOM-GLOBAL',
    source: `<script>const el = document.getElementById('x');</script><section><p>x</p></section>`,
  },
  {
    name: 'setTimeout',
    code: 'DOM-GLOBAL',
    source: `<script>setTimeout(() => {}, 10);</script><section><p>x</p></section>`,
  },
  {
    name: 'location global',
    code: 'DOM-GLOBAL',
    source: `<script>const h = location.hash;</script><section><p>x</p></section>`,
  },
  {
    name: 'fetch',
    code: 'DOM-GLOBAL',
    source: `<script>fetch('/x');</script><section><p>x</p></section>`,
  },
  {
    name: 'console',
    code: 'DOM-GLOBAL',
    source: `<script>console.log(1);</script><section><p>x</p></section>`,
  },
  {
    name: 'spread attributes',
    code: 'SVELTE-SPREAD',
    source: `<script>let a = {id: 'x'};</script><section><p {...a}>y</p></section>`,
  },
  {
    name: 'await block',
    code: 'SVELTE-AWAIT',
    source: `<script>let p = 1;</script><section>{#await p}<p>x</p>{/await}</section>`,
  },
  {
    name: 'slot',
    code: 'SVELTE-SLOT',
    source: `<section><slot /></section>`,
  },
  {
    name: 'unsupported rune',
    code: 'SVELTE-RUNE',
    source: `<script>let v = $state(1); let w = $inspect(v);</script><section><p>x</p></section>`,
  },
  {
    name: 'bare module import',
    code: 'SVELTE-IMPORT',
    source: `<script>import x from 'lodash';</script><section><p>{x}</p></section>`,
  },
  {
    name: 'grid css',
    code: 'CSS-PROPERTY',
    source: `<section><p>x</p></section>
<style>section { grid-template-columns: 1fr 1fr; }</style>`,
  },
  {
    name: 'media query',
    code: 'CSS-AT-RULE',
    source: `<section><p>x</p></section>
<style>@media (min-width: 600px) { section { color: #fff; } }</style>`,
  },
  {
    name: 'border radius',
    code: 'CSS-PROPERTY',
    source: `<section><p>x</p></section>
<style>section { border-radius: 4px; }</style>`,
  },
  // Note: a *well-formed but unresolvable* reference — `var(--nope)`, or
  // `var(fg)` with a name that is not a custom property — is NOT reported here.
  // It is a resolve-time error in Go, because a value containing var() cannot be
  // validated before substitution, and substitution needs the cascade. The
  // adapter mirror cannot do that, and guessing would mean rejecting stylesheets
  // that are valid.
  {
    name: 'unterminated var()',
    code: 'CSS-PROPERTY',
    source: `<section><p>x</p></section>
<style>section { color: var(--fg; }</style>`,
  },
  {
    name: 'color-mix in a colour space the subset does not implement',
    code: 'CSS-PROPERTY',
    source: `<section><p>x</p></section>
<style>section { color: color-mix(in oklch, #000, #fff); }</style>`,
  },
] as const;
