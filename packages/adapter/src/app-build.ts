/**
 * Application build: project in, dist directory out.
 *
 * Lives in src/ rather than scripts/ so the bundled CLI can use it: the CLI is
 * CommonJS (esbuild is required at runtime), where `import.meta` does not
 * exist. scripts/build.ts is the developer entry that also bundles the CLI.
 */
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { build, cssToText, type BuildOptions } from './index.js';
import { emitBundle } from './bundle.js';

export interface AppBuildOptions {
  rootDir: string;
  entry: string;
  outDir: string;
  /** Overridable for tests; defaults to the adapter's own CSS subset check. */
  validateCss?: BuildOptions['validateCss'];
  /** Raw global stylesheets to validate and place before component CSS. */
  globalStyles?: BuildOptions['globalStyles'];
}

export interface AppBuildResult {
  ops: number;
  findings: number;
  files: string[];
}

/**
 * buildApp compiles one project into dist/: manifest.json, app.js (the single
 * IIFE the Go runtime evaluates) and styles.css.
 *
 * Strict mode: a finding throws before anything is written, so a failed build
 * never leaves a half-updated dist directory behind (I1).
 */
export async function buildApp(options: AppBuildOptions): Promise<AppBuildResult> {
  const result = build({
    rootDir: options.rootDir,
    entry: options.entry,
    mode: 'strict',
    validateCss: options.validateCss,
    globalStyles: options.globalStyles,
  });

  const iife = await emitBundle(result.bundle);

  mkdirSync(options.outDir, { recursive: true });
  const files = ['app.js', 'styles.css', 'manifest.json'];
  writeFileSync(join(options.outDir, 'app.js'), iife, 'utf8');
  writeFileSync(join(options.outDir, 'styles.css'), `${cssToText(result.css)}\n`, 'utf8');
  writeFileSync(
    join(options.outDir, 'manifest.json'),
    `${JSON.stringify(result.manifest, null, 2)}\n`,
    'utf8',
  );

  return { ops: result.ops.length, findings: result.findings.length, files };
}
