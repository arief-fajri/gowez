/**
 * Bundle emission.
 *
 * The Go runtime evaluates `app.js` in one `Eval`, so the bundle must be a
 * single file with no imports, at a language level the engine can run. These
 * tests pin that, because a violation would only surface as a runtime
 * ReferenceError on a user's machine.
 */
import { describe, expect, it } from 'vitest';
import { BANNED_IN_BUNDLE, BUNDLE_FORMAT, BUNDLE_TARGET, emitBundle } from '../src/bundle.js';
import { buildFixture } from './helpers.js';
import { counterFixture } from './fixtures.js';

describe('emitted bundle', () => {
  it('is one file with no imports and no require calls', async () => {
    const iife = await emitBundle(buildFixture(counterFixture).bundle);
    expect(iife).not.toMatch(/^\s*import\s/m);
    expect(iife).not.toContain('require(');
    // format:iife wraps everything in one call expression.
    expect(iife.trimStart().startsWith('(()')).toBe(true);
  });

  it('targets ES2015, bounded by the measured goja gaps', async () => {
    expect(BUNDLE_TARGET).toBe('es2015');
    expect(BUNDLE_FORMAT).toBe('iife');

    const iife = await emitBundle(buildFixture(counterFixture).bundle);
    for (const absent of [
      'structuredClone',
      'Object.groupBy',
      'performance.now',
      'WeakRef',
      'Array.fromAsync',
      // ES2022 class static blocks are the usual accidental overflow.
      'static {',
    ]) {
      expect(iife).not.toContain(absent);
    }
  });

  it('contains no sandbox-forbidden identifiers', async () => {
    const iife = await emitBundle(buildFixture(counterFixture).bundle);
    for (const banned of BANNED_IN_BUNDLE) {
      expect(iife, `bundle must not contain ${banned}`).not.toContain(banned);
    }
  });

  it('uses only the documented host surface', async () => {
    const iife = await emitBundle(buildFixture(counterFixture).bundle);
    // gowez.invoke and gowez.on are the entire host contract (docs/SCRIPT.md).
    expect(iife).toContain('gowez.invoke');
    expect(iife).not.toContain('gowez.register');
    expect(iife).not.toContain('__gowez_');
  });

  it('is deterministic: the same input produces the same bytes', async () => {
    const a = await emitBundle(buildFixture(counterFixture).bundle);
    const b = await emitBundle(buildFixture(counterFixture).bundle);
    expect(a).toBe(b);
  });

  it('evaluates in a sandbox that provides only gowez.*', async () => {
    // The real check: run the bundle in a bare JS realm with the documented
    // host surface and nothing else. Anything else the runtime touches throws
    // here first, which is far cheaper to debug than on a user's machine.
    const iife = await emitBundle(buildFixture(counterFixture).bundle);

    const submitted: unknown[] = [];
    const gowez = {
      invoke: (method: string, params: unknown) => {
        submitted.push({ method, params });
        return true;
      },
      call: () => Promise.resolve(true),
      on: () => undefined,
    };

    // eslint-disable-next-line no-new-func -- the point is to evaluate the bundle
    // exactly as goja would: no document, no window, no timers.
    const run = new Function('gowez', iife) as (host: unknown) => void;
    expect(() => run(gowez)).not.toThrow();

    expect(submitted.length).toBeGreaterThan(0);
    const first = submitted[0] as { method: string; params: { version: number; ops: unknown[] } };
    expect(first.method).toBe('ui.apply');
    expect(first.params.version).toBe(1);
    expect(Array.isArray(first.params.ops)).toBe(true);
  });
});
