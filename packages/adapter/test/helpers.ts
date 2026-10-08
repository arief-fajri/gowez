import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { build, report } from '../src/index.js';

/** project writes a fixture set to a temp dir and returns its root. */
export function project(files: Record<string, string>): string {
  const root = mkdtempSync(join(tmpdir(), 'gowez-adapter-'));
  for (const [rel, content] of Object.entries(files)) {
    const abs = join(root, rel);
    // Fixtures live under src/, so the directory has to exist before the file
    // is written. A nested path gets mkdirSync rather than a flat assumption.
    mkdirSync(dirname(abs), { recursive: true });
    writeFileSync(abs, content, 'utf8');
  }
  return root;
}

/** buildFixture compiles a single-module fixture in strict mode. */
export function buildFixture(source: string, extra: Record<string, string> = {}) {
  const root = project({ 'src/App.svelte': source, ...extra });
  return build({ rootDir: root, entry: 'src/App.svelte' });
}

/** reportFixture compiles a single-module fixture in report mode. */
export function reportFixture(source: string, extra: Record<string, string> = {}) {
  const root = project({ 'src/App.svelte': source, ...extra });
  return report(root, 'src/App.svelte');
}
