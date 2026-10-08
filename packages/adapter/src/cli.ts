#!/usr/bin/env node
/**
 * @gowez/adapter CLI.
 *
 * Usage:
 *   gowez-adapter build  --root <dir> --entry <App.svelte> --out <dist>
 *   gowez-adapter report --root <dir> --entry <App.svelte> [--json] [--out <file>]
 *
 * `build` is strict: any finding fails with a CompileError and nothing is
 * written. `report` is the gap-register generator: it always exits 0 and prints
 * findings, because its output is data about unsupported constructs rather than
 * a build failure.
 */
import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { build, CompileError } from './index.js';
import { buildApp } from './app-build.js';
import type { Finding } from './findings.js';

interface Args {
  command: 'build' | 'report';
  root: string;
  entry: string;
  out?: string;
  json: boolean;
}

function parseArgs(argv: string[]): Args {
  const command = argv[0];
  if (command !== 'build' && command !== 'report') {
    fail(`unknown command ${command ?? '(none)'}; expected "build" or "report"`);
  }
  const get = (name: string): string | undefined => {
    const i = argv.indexOf(`--${name}`);
    return i >= 0 ? argv[i + 1] : undefined;
  };
  const args: Args = {
    command,
    root: get('root') ?? process.cwd(),
    entry: get('entry') ?? 'src/App.svelte',
    out: get('out'),
    json: argv.includes('--json'),
  };
  if (!args.entry) fail('--entry needs a value');
  return args;
}

function fail(message: string): never {
  process.stderr.write(`gowez-adapter: ${message}\n`);
  process.exit(1);
}

async function main(): Promise<void> {
  const args = parseArgs(process.argv.slice(2));

  try {
    if (args.command === 'report') {
      const result = build({
        rootDir: resolve(args.root),
        entry: args.entry,
        mode: 'report',
      });
      emitReport(result.findings, args);
      return;
    }

    if (!args.out) fail('build requires --out <dist>');
    const written = await buildApp({
      rootDir: resolve(args.root),
      entry: args.entry,
      outDir: resolve(args.out),
    });
    process.stdout.write(
      `gowez-adapter: wrote ${written.ops} ops to ${args.out}\n`,
    );
  } catch (err) {
    if (err instanceof CompileError) {
      process.stderr.write(`${err.message}\n`);
      process.exit(1);
    }
    fail(err instanceof Error ? err.message : String(err));
  }
}

function emitReport(findings: readonly Finding[], args: Args): void {
  if (args.json) {
    process.stdout.write(`${JSON.stringify(findings, null, 2)}\n`);
    return;
  }

  const byCode = new Map<string, number>();
  for (const f of findings) byCode.set(f.code, (byCode.get(f.code) ?? 0) + 1);

  process.stdout.write(`gowez-adapter report: ${findings.length} findings\n`);
  for (const [code, count] of [...byCode].sort((a, b) => b[1] - a[1])) {
    process.stdout.write(`  ${String(count).padStart(4)}  ${code}\n`);
  }
  if (args.out) {
    writeFileSync(resolve(args.out), `${JSON.stringify(findings, null, 2)}\n`, 'utf8');
  }
}

// The bundled CLI is CommonJS (esbuild is required at runtime), which forbids
// top-level await, so the entry is an explicit promise chain.
void main();
