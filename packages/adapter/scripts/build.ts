/**
 * Developer entry: bundles the adapter CLI for Node.
 *
 * The CLI ships as CommonJS because esbuild itself must be `require`d at
 * runtime — bundling it into an ESM file fails with "Dynamic require of fs is
 * not supported", since esbuild reaches for node builtins dynamically.
 * That in turn forbids top-level await, which is why cli.ts ends with an
 * explicit `void main()` rather than `await main()`.
 */
import { build as esbuild } from 'esbuild';
import { mkdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const packageRoot = resolve(here, '..');

/** bundleCli produces the runnable CLI entry. */
export async function bundleCli(
  outFile = join(packageRoot, 'dist', 'cli.cjs'),
): Promise<string> {
  mkdirSync(dirname(outFile), { recursive: true });
  await esbuild({
    entryPoints: [join(packageRoot, 'src', 'cli.ts')],
    outfile: outFile,
    bundle: true,
    platform: 'node',
    format: 'cjs',
    target: 'node20',
    // esbuild, svelte and svelte/compiler stay external: they are build-time
    // dependencies resolved at runtime, and inlining esbuild is what breaks
    // the dynamic require above.
    external: ['esbuild', 'svelte', 'svelte/compiler'],
    logLevel: 'silent',
  });
  return outFile;
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(fileURLToPath(import.meta.url))) {
  void bundleCli().then((out) => {
    process.stdout.write(`adapter: bundled CLI at ${out}\n`);
  });
}
