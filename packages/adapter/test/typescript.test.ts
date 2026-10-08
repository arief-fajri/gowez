/**
 * TypeScript shape handling (decision D-2, DRR-007).
 *
 * `parse()` does not strip TypeScript and `remove_typescript_nodes` is
 * unexported, so the adapter owns its stripping. The contract is *closed*: an
 * unrecognized TS shape fails the build rather than reaching codegen with a
 * residual annotation (G-UPG-04).
 */
import { describe, expect, it } from 'vitest';
import { parse } from 'svelte/compiler';
import { classify, stripTypes, TS_SHAPES, type Node } from '../src/typescript.js';
import { buildFixture, reportFixture } from './helpers.js';

/** astOf parses source into the modern AST the adapter walks. */
function astOf(source: string): Node {
  return parse(source, { modern: true, filename: 'App.svelte' }) as unknown as Node;
}

describe('TS shape classification', () => {
  it('a lang="ts" module really does carry the TS shapes the adapter strips', () => {
    // The premise of D-2, asserted rather than assumed: parse() keeps TS nodes,
    // which is why the adapter strips them itself.
    const shapes = collectShapes(
      astOf(`<script lang="ts">
        let a: string = 'x';
        interface I { n: number }
        type T = string;
      </script><section><p>a</p></section>`),
    );
    expect(shapes.has('TSTypeAnnotation')).toBe(true);
    expect(shapes.has('TSInterfaceDeclaration')).toBe(true);
    expect(shapes.has('TSTypeAliasDeclaration')).toBe(true);
  });

  it('stripTypes removes type-only declarations from the tree', () => {
    const ast = astOf(`<script lang="ts">
      interface I { n: number }
      let a = 1;
    </script><section><p>a</p></section>`);
    const scope = (ast as { instance?: unknown }).instance;
    stripTypes(scope as Node, () => {
      /* visit */
    });
    const remaining = collectShapes(scope);
    expect(remaining.has('TSInterfaceDeclaration')).toBe(false);
  });

  it('reports an unknown TS shape instead of passing it through', () => {
    // A shape outside the closed set must be classified 'unknown'.
    const fake = { type: 'TSFutureThing' } as unknown as Node;
    expect(classify(fake)).toEqual({ kind: 'unknown', shape: 'TSFutureThing' });
  });

  it('passes non-TS nodes through unchanged', () => {
    const node = { type: 'Identifier', name: 'x' } as unknown as Node;
    expect(classify(node)).toEqual({ kind: 'passthrough' });
  });

  it('drops type-only declarations entirely', () => {
    for (const shape of TS_SHAPES.drop) {
      expect(classify({ type: shape } as unknown as Node)).toEqual({ kind: 'drop' });
    }
  });

  it('unwraps a wrapper with no expression to a dropped node', () => {
    expect(classify({ type: 'TSTypeAnnotation' } as unknown as Node)).toEqual({
      kind: 'expression',
      node: null,
    });
  });
});

describe('TS modules compile', () => {
  it('a lang="ts" module with type annotations produces no findings', () => {
    const source = `<script lang="ts">
      interface User { id: number; name: string }
      type Slug = 'a' | 'b';
      let users: User[] = [{ id: 1, name: 'one' }];
      let count: number = users.length;
    </script>
    <section>
      <p>count: {count}</p>
      {#each users as u (u.id)}
        <p>{u.name}</p>
      {/each}
    </section>`;
    const result = buildFixture(source);
    expect(result.findings).toEqual([]);
  });

  it('import type vanishes without needing handling', () => {
    const source = `<script lang="ts">
      import type { User } from './types';
      let u: User | null = null;
    </script>
    <section><p>{u ? 'yes' : 'no'}</p></section>`;
    const result = reportFixture(source, {
      'src/types.ts': 'export interface User { id: number }',
    });
    // `import type` survives parse() with importKind='type'; the adapter drops
    // it, so no import finding is produced. A value import would be one.
    expect(result.findings.map((f) => f.code)).not.toContain('SVELTE-IMPORT');
  });

  it('a non-type import of a .ts module is an explicit finding', () => {
    const source = `<script lang="ts">
      import { helper } from './helpers';
    </script>
    <section><p>{helper()}</p></section>`;
    const result = reportFixture(source, {
      'src/helpers.ts': 'export function helper() { return 1; }',
    });
    expect(result.findings.map((f) => f.code)).toContain('SVELTE-IMPORT');
  });

  it('type-only code never reaches the emitted ops', () => {
    const source = `<script lang="ts">
      interface Hidden { secret: string }
      let visible: string = 'shown';
    </script>
    <section><p>{visible}</p></section>`;
    const result = buildFixture(source);
    const json = JSON.stringify(result.ops);
    expect(json).not.toContain('Hidden');
    expect(json).not.toContain('secret');
    expect(json).not.toContain('TS');
  });
});

/** collectShapes gathers every node type present in an AST. */
function collectShapes(root: unknown): Set<string> {
  const out = new Set<string>();
  const walk = (n: unknown): void => {
    if (Array.isArray(n)) {
      n.forEach(walk);
      return;
    }
    if (!n || typeof n !== 'object') return;
    const node = n as Node;
    if (typeof node.type === 'string') out.add(node.type);
    for (const key of Object.keys(node)) {
      if (key === 'loc' || key === 'parent') continue;
      walk((node as Record<string, unknown>)[key]);
    }
  };
  walk(root);
  return out;
}
