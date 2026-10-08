/**
 * @gowez/adapter — Svelte → GoWEZ UI instructions (Strategy B, Milestone 5).
 *
 * The adapter walks the Svelte AST and emits the instruction stream defined by
 * protocol/ui-instruction.schema.json. The Go runtime never depends on browser
 * or DOM semantics, and Svelte's own client runtime never executes in the Go
 * process (DRR-007).
 *
 * Two modes share one implementation:
 *
 * - **strict** — the default; any finding fails the build with a
 *   `CompileError{file, line, column, code}`.
 * - **report** — walks everything and returns the findings as data, which is
 *   how the gap register in docs/SVELTE.md is generated.
 */
import { readFileSync } from 'node:fs';
import { parse } from 'svelte/compiler';
import { CompileError, Report, type Finding } from './findings.js';
import { resolveGraph, projectRelative } from './graph.js';
import { EMITTED_GLOBALS, RUNTIME } from './runtime.js';
import { factoryName } from './codegen.js';
import { MANIFEST_SCHEMA_VERSION, SVELTE_MAJOR, scopeOf, scopesCollide } from './subset.js';
import { validateCssSubset } from './css.js';
import {
  recordComponentSpecifiers,
  walk,
  type CompiledModule,
  type CssRule,
} from './walk.js';
import { IdGen } from './ops.js';
import type { Node } from './typescript.js';

export { CODES, CompileError, Report } from './findings.js';
export type { Finding, Code, Category } from './findings.js';
export type { CompiledModule, CssRule } from './walk.js';
export { TS_SHAPES } from './typescript.js';
export {
  ELEMENTS,
  EVENTS,
  BINDS,
  RUNES,
  LIFECYCLE,
  BROWSER_GLOBALS,
  APPLY_METHOD,
  INSTRUCTION_VERSION,
  MANIFEST_SCHEMA_VERSION,
  SVELTE_MAJOR,
  handlerName,
} from './subset.js';

/** BuildMode selects between failing on a finding and returning it. */
export type BuildMode = 'strict' | 'report';

export interface BuildOptions {
  /** Directory the entry path is relative to; also the finding-path root. */
  rootDir: string;
  /** Project-relative path of the entry .svelte module. */
  entry: string;
  /** strict (default) fails the build on the first finding; report collects. */
  mode?: BuildMode;
  /** Validates component CSS; overridable so tests can inject a stub. */
  validateCss?: typeof validateCssSubset;
}

export interface BuildResult {
  /** Instruction ops in dependency-first order. */
  ops: CompiledModule['ops'];
  /** The entry module's root node id. */
  entry: number;
  /** Collected component CSS rules, in module order. */
  css: CssRule[];
  /** Findings, in walk order. Empty in a successful strict build. */
  findings: readonly Finding[];
  /** Generated bundle source: runtime prelude plus one block per module. */
  bundle: string;
  /** manifest.json content. */
  manifest: Manifest;
}

export interface Manifest {
  schemaVersion: number;
  adapter: { name: string; version: string };
  svelte: number;
  script: string;
  styles: string;
}

export const ADAPTER_NAME = 'gowez-adapter';
export const ADAPTER_VERSION = '1.0.0';

/**
 * build compiles one Svelte entry point into ops, CSS, a bundle, and a manifest.
 *
 * In strict mode the first finding throws a CompileError, so a build either
 * produces a complete bundle or fails with an actionable location — never a
 * half-compiled artifact (G-UPG-04, P4).
 */
export function build(options: BuildOptions): BuildResult {
  const mode = options.mode ?? 'strict';
  const validateCss = options.validateCss ?? validateCssSubset;
  const report = new Report();

  const graph = resolveGraph(options.rootDir, options.entry, report);

  // One id allocator for the whole graph: node ids form a single flat address
  // space, so a per-module counter would collide in the single op stream.
  const ids = new IdGen();

  // Two passes over the graph: compile dependencies first (so their ops exist
  // before an importer references them), then the entry module last.
  const modules: CompiledModule[] = [];
  const entries = new Map<string, number>();
  const componentEntry = (specifier: string): number | null => {
    const abs = graph.absolute.get(specifier);
    if (!abs) return null;
    return entries.get(specifier) ?? null;
  };

  // Style scoping: every module's rules get a scope class derived from its path
  // and every element it renders carries that class. Two modules that sanitize
  // to the same class would leak styles back together, which is the one bug
  // scoping exists to prevent, so it fails the build instead.
  for (const group of scopesCollide(graph.order)) {
    report.add({
      code: 'CSS-SCOPE-COLLISION',
      category: 'css',
      file: group,
      line: 0,
      column: 0,
      message: `modules ${group} derive the same style scope; rename one of them`,
    });
  }

  for (const rel of graph.order) {
    const abs = graph.absolute.get(rel);
    if (!abs) continue;
    const source = readModule(abs);
    const displayPath = projectRelative(options.rootDir, abs);
    recordComponentSpecifiers(displayPath, parseSvelte(source, displayPath));
    const compiled = walk(source, displayPath, report, {
      ids,
      scope: scopeOf(rel),
      componentEntry: (specifier) => componentEntry(resolveSpecifier(displayPath, specifier)),
      validateCss: (rule, file, line) => {
        // CSS findings are collected rather than thrown so report mode can show
        // the full picture; strict mode turns them into a CompileError below.
        try {
          validateCss(rule.declarations, file);
        } catch (err) {
          report.add({
            code: cssCodeOf(err),
            category: 'css',
            file,
            line,
            column: 0,
            message: err instanceof Error ? err.message : String(err),
          });
        }
      },
    });
    modules.push(compiled);
    entries.set(rel, compiled.entry);
  }

  if (mode === 'strict') {
    const first = report.all[0];
    if (first) throw new CompileError(first);
  }

  const css = modules.flatMap((m) => m.css);
  const entryModule = modules[modules.length - 1];
  const entry = entryModule?.entry ?? 0;

  // The bundle is the runtime prelude plus one factory per module, mounted in
  // dependency order with the entry last.
  const bundle = emitBundle(modules);

  return {
    ops: modules.flatMap((m) => m.ops),
    entry,
    css,
    findings: report.all,
    bundle,
    manifest: {
      schemaVersion: MANIFEST_SCHEMA_VERSION,
      adapter: { name: ADAPTER_NAME, version: ADAPTER_VERSION },
      svelte: SVELTE_MAJOR,
      script: 'app.js',
      styles: 'styles.css',
    },
  };
}

/**
 * report walks the whole project and returns findings without throwing. This is
 * the gap-register generator: the dashboard sample's unsupported constructs are
 * data, not build errors.
 */
export function report(
  rootDir: string,
  entry: string,
  opts: { validateCss?: typeof validateCssSubset } = {},
): BuildResult {
  return build({ rootDir, entry, mode: 'report', validateCss: opts.validateCss });
}

/**
 * emitBundle renders the application script.
 *
 * Factories are emitted before the mount call and mounted in dependency order,
 * so a child component exists by the time its parent renders it.
 */
function emitBundle(modules: ReadonlyArray<CompiledModule>): string {
  const entryModule = modules[modules.length - 1];
  if (!entryModule) throw new Error('adapter: no modules compiled');
  const lines: string[] = [RUNTIME, ''];

  for (const module of modules) {
    // The generated source already declares the factory and labels the file.
    lines.push(module.render);
    lines.push('');
  }

  // Only the entry module is mounted. A child component is reached by its
  // parent's factory call, not mounted on its own — mounting every module
  // would give the app several independent tree roots.
  lines.push('// The entry module owns the tree; children render inside it.');
  lines.push(`mount(${factoryName(entryModule.file)}, {});`);
  lines.push('');

  return lines.join('\n');
}

/**
 * cssToText renders collected rules as a stylesheet.
 *
 * Selectors are preserved: internal/style parses plain CSS text, so a rule
 * without its selector would have nothing to attach declarations to. Rules are
 * emitted in walk order (which follows source order), because cascade
 * resolution in Go depends on it — reordering would change which rule wins.
 */
export function cssToText(rules: ReadonlyArray<CssRule>): string {
  if (rules.length === 0) return '';
  return rules
    .map((rule) => {
      const body = rule.declarations
        .map((d) => `  ${d.property}: ${d.value};`)
        .join('\n');
      return `${rule.selector} {\n${body}\n}`;
    })
    .join('\n');
}

function cssCodeOf(err: unknown): Finding['code'] {
  const message = err instanceof Error ? err.message : String(err);
  return message.startsWith('@') ? 'CSS-AT-RULE' : 'CSS-PROPERTY';
}

function readModule(abs: string): string {
  return readFileSync(abs, 'utf8');
}

/** parseSvelte parses a module for the import scan. */
function parseSvelte(source: string, file: string): Node {
  return parse(source, { modern: true, filename: file }) as unknown as Node;
}

/**
 * resolveSpecifier resolves an import specifier against the importing file, so
 * the component map is keyed by the same relative path the graph uses.
 */
function resolveSpecifier(fromFile: string, specifier: string): string {
  const dir = fromFile.split('/').slice(0, -1).join('/');
  const joined = normalizePath(dir === '' ? specifier : `${dir}/${specifier}`);
  return joined;
}

/** normalizePath collapses "." and ".." segments without touching the disk. */
function normalizePath(p: string): string {
  const out: string[] = [];
  for (const part of p.split('/')) {
    if (part === '.' || part === '') continue;
    if (part === '..') {
      out.pop();
      continue;
    }
    out.push(part);
  }
  return out.join('/');
}

/** The globals the emitted runtime defines; a bundle must not define them again. */
export const RUNTIME_GLOBALS = EMITTED_GLOBALS;
