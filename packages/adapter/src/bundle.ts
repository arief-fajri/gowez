/**
 * Emitting the runtime bundle.
 *
 * The Go runtime evaluates `app.js` with a single `Eval` inside the M4 sandbox.
 * That imposes three hard constraints the bundler must respect:
 *
 * 1. **One file, no imports.** goja has no module system, so anything left as
 *    an import would fail at load time rather than degrade.
 * 2. **ES2015 at most.** Measured goja gaps — `structuredClone`,
 *    `Object.groupBy`, `performance`, `WeakRef`, `Array.fromAsync` — are all
 *    post-ES2015, so a higher target would emit code the engine cannot run.
 * 3. **No globals beyond what the host provides.** The bundle may use
 *    `gowez.invoke` / `gowez.on` (docs/SCRIPT.md) and nothing else.
 */
import { build as esbuild } from 'esbuild';

/** BUNDLE_TARGET is the language level the sandbox can execute. */
export const BUNDLE_TARGET = 'es2015';

/** BUNDLE_FORMAT is fixed: one IIFE, no code splitting. */
export const BUNDLE_FORMAT = 'iife';

/**
 * emitBundle turns generated module code into the single-file IIFE the runtime
 * evaluates.
 */
export async function emitBundle(source: string): Promise<string> {
  const result = await esbuild({
    stdin: {
      contents: source,
      resolveDir: process.cwd(),
      sourcefile: 'gowez-bundle.js',
      loader: 'js',
    },
    bundle: true,
    write: false,
    format: BUNDLE_FORMAT,
    target: BUNDLE_TARGET,
    splitting: false,
    platform: 'neutral',
    minify: false,
    // The bundle must not pull in anything: the runtime has no filesystem, no
    // network, and no node builtins. Refusing to resolve them turns a mistake
    // into a build failure instead of a runtime ReferenceError.
    external: [],
  });

  const output = result.outputFiles?.[0];
  if (!output) throw new Error('adapter: esbuild produced no output');
  return output.text;
}

/**
 * BANNED_IN_BUNDLE lists identifiers that must never appear in the emitted
 * bundle. The tests assert their absence, because each one would be either a
 * silent capability leak or an immediate sandbox ReferenceError.
 */
export const BANNED_IN_BUNDLE = [
  'document',
  'window',
  'setTimeout',
  'setInterval',
  'fetch(',
  'require(',
  'process.',
  'import(',
  'XMLHttpRequest',
  'localStorage',
] as const;
