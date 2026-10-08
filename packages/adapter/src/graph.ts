/**
 * Module graph resolution.
 *
 * The adapter resolves relative `.svelte` imports itself rather than letting a
 * bundler do it, because the walk needs each module's findings attributed to
 * its own file and its ops wired to its own entry node. Resolution is
 * deliberately narrow: relative paths ending in `.svelte`, plus `import type`
 * which the AST drops on its own.
 */
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join, normalize, relative, resolve, sep } from 'node:path';
import { parse } from 'svelte/compiler';
import type { Node } from './typescript.js';
import type { Report } from './findings.js';

export interface ModuleGraph {
  /** Project-relative path → absolute path, in dependency-first order. */
  order: string[];
  absolute: Map<string, string>;
}

/**
 * resolveGraph walks the module graph from entry, in depth-first post-order so
 * a dependency's ops precede its importer's. Cycles are reported as a finding
 * rather than followed forever (P3: never block indefinitely).
 */
export function resolveGraph(rootDir: string, entry: string, report: Report): ModuleGraph {
  const absolute = new Map<string, string>();
  const order: string[] = [];
  const visiting = new Set<string>();

  const visit = (rel: string): void => {
    if (absolute.has(rel)) return;
    if (visiting.has(rel)) {
      report.add({
        code: 'SVELTE-IMPORT',
        category: 'svelte',
        file: rel,
        line: 0,
        column: 0,
        message: `import cycle through ${rel}`,
      });
      return;
    }
    const abs = join(rootDir, rel);
    if (!existsSync(abs)) {
      report.add({
        code: 'SVELTE-IMPORT',
        category: 'svelte',
        file: rel,
        line: 0,
        column: 0,
        message: `module not found: ${rel}`,
      });
      return;
    }

    visiting.add(rel);
    const ast = parse(readFileSync(abs, 'utf8'), {
      modern: true,
      filename: rel,
    }) as unknown as Node;

    for (const spec of importSpecifiers(ast)) {
      if (!spec.source.startsWith('.')) continue;
      const resolved = resolveRelative(rootDir, rel, spec.source);
      if (!resolved.endsWith('.svelte')) {
        report.add({
          code: 'SVELTE-IMPORT',
          category: 'svelte',
          file: rel,
          line: spec.line,
          column: spec.column,
          message: `only .svelte modules can be imported (got ${spec.source})`,
        });
        continue;
      }
      visit(resolved);
    }

    visiting.delete(rel);
    absolute.set(rel, abs);
    order.push(rel);
  };

  visit(entry);
  return { order, absolute };
}

/**
 * importSpecifiers lists the import sources with positions.
 *
 * Both script scopes are scanned: an application may put imports in the
 * instance script (`<script>`) rather than the module script
 * (`<script context="module">`), and missing the latter would silently accept
 * a dependency the adapter never compiles.
 */
function importSpecifiers(ast: Node): Array<{ source: string; line: number; column: number }> {
  const out: Array<{ source: string; line: number; column: number }> = [];
  const scopes = [
    (ast as { module?: { content?: { body?: Node[] } } }).module,
    (ast as { instance?: { content?: { body?: Node[] } } }).instance,
  ];
  const body = scopes.flatMap((scope) => scope?.content?.body ?? []);
  for (const stmt of body) {
    if (stmt.type !== 'ImportDeclaration') continue;
    // `import type` survives parse() (it is compile() that removes it), so it
    // is filtered here — a type-only import has no module to compile.
    if ((stmt as { importKind?: unknown }).importKind === 'type') continue;
    const source = String((stmt as { source?: { value?: unknown } }).source?.value ?? '');
    out.push({
      source,
      line: stmt.loc?.start.line ?? 0,
      column: stmt.loc?.start.column ?? 0,
    });
  }
  return out;
}

/** resolveRelative resolves an import specifier against the importing file. */
function resolveRelative(rootDir: string, fromFile: string, specifier: string): string {
  const dir = dirname(fromFile);
  const joined = normalize(join(dir, specifier));
  if (existsSync(join(rootDir, joined))) return joined;
  // Svelte allows extensionless relative imports for .svelte files.
  if (existsSync(join(rootDir, `${joined}.svelte`))) return `${joined}.svelte`;
  return joined;
}

/** projectRelative normalizes a path for reporting, relative to the root. */
export function projectRelative(rootDir: string, file: string): string {
  return relative(rootDir, resolve(file)).split(sep).join('/');
}
