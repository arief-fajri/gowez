/**
 * The Svelte AST walker: the heart of Strategy B.
 *
 * It walks one module's parsed AST and emits UI instruction ops plus a finding
 * for every construct outside the M5 subset. It never touches the DOM and
 * never calls Svelte's compiler internals: `parse()` supplies the AST, and
 * everything after that is ours.
 *
 * The runtime JavaScript for reactive state is *emitted*, not walked: the
 * adapter generates a small runtime whose only job is to re-run a module's
 * render function and submit the resulting ops through ui.apply.
 */
import { parse } from 'svelte/compiler';
import {
  ATTRIBUTES,
  BOOLEAN_ATTRIBUTES,
  BROWSER_GLOBALS,
  BINDS,
  ELEMENTS,
  EVENTS,
  FOCUSABLE,
  LIFECYCLE,
  RUNES,
} from './subset.js';
import { IdGen, Ops, type Batch } from './ops.js';
import type { Code, Finding, Report } from './findings.js';
import { classify, stripTypes, type Node } from './typescript.js';
import { factoryName as factoryNameFor, generateFactory } from './codegen.js';

/**
 * HTML_TAGS is the tag set the CSS printer treats as element selectors.
 * Anything else in a component stylesheet is a class name (Svelte's rule).
 */
const HTML_TAGS = new Set([
  'a', 'article', 'aside', 'button', 'code', 'div', 'footer', 'form', 'h1',
  'h2', 'h3', 'h4', 'header', 'input', 'label', 'li', 'main', 'nav', 'ol',
  'p', 'section', 'span', 'ul',
]);

/** CssRule is one validated stylesheet rule from a component's <style> block. */
export interface CssRule {
  /** Type selector text as authored, e.g. "button" or "row:hover". */
  selector: string;
  declarations: Array<{ property: string; value: string }>;
}

/** CompiledModule is the result of walking one .svelte file. */
export interface CompiledModule {
  /** Path relative to the project root, used in findings and the manifest. */
  file: string;
  /** Adapter node id of the component root. */
  entry: number;
  ops: Batch['ops'];
  /**
   * Component `<style>` rules, validated against the subset. The selector is
   * kept because the Go stylesheet is selector-based: dropping it would emit
   * declarations with no rule to attach them to.
   */
  css: CssRule[];
  /** Props destructured in the instance script, with defaults. */
  props: Array<{ name: string; expression: string }>;
  /** True when the module calls onMount/onDestroy. */
  lifecycle: boolean;
  /** True when the module uses runes, which drives the runtime shape. */
  runes: boolean;
  /**
   * Generated render function source. The op stream is produced by *running*
   * it in the sandbox, not by walking the template — see runtime.ts.
   */
  render: string;
}

/** CssValidation is injected so the CSS subset check can be tested without Go. */
export interface CssValidation {
  (
    rule: { selector: string; declarations: Array<{ property: string; value: string }> },
    file: string,
    line: number,
  ): void;
}

/** WalkOptions configures one walk. */
export interface WalkOptions {
  /** Validates component CSS against the GoWEZ subset; rejects by throwing. */
  validateCss: CssValidation;
  /**
   * Component style scope (see scopeOf). Every rule of this module gets the
   * class appended to its last compound and every emitted element carries the
   * class, so a module's styles cannot reach another module's elements.
   */
  scope?: string;
  /**
   * Shared id allocator. One graph must allocate from one source: ids are the
   * adapter's address space, and two modules each starting at 1 would collide
   * in the single op stream the runtime applies.
   */
  ids?: IdGen;
  /**
   * Resolves a relative import to the entry node of an already-compiled
   * dependency, so a `<Child />` renders *inside* the parent's node instead of
   * becoming a second tree root.
   */
  componentEntry?: (specifier: string) => number | null;
}

function finding(
  report: Report,
  code: Code,
  node: Node | null | undefined,
  file: string,
  message: string,
): void {
  const f: Finding = {
    code,
    category: categoryOf(code),
    file,
    line: node?.loc?.start.line ?? 0,
    column: node?.loc?.start.column ?? 0,
    message,
  };
  report.add(f);
}

function categoryOf(code: Code): Finding['category'] {
  // Imported lazily to avoid a cycle at module init: findings owns the table.
  return CATEGORY[code];
}

// Kept in sync with findings.CODES via the test that asserts they match.
const CATEGORY: Record<Code, Finding['category']> = {
  'SVELTE-TRANSITION': 'svelte',
  'SVELTE-HEAD': 'svelte',
  'SVELTE-WINDOW': 'svelte',
  'SVELTE-SLOT': 'svelte',
  'SVELTE-SPREAD': 'svelte',
  'SVELTE-AWAIT': 'svelte',
  'SVELTE-RAW-HTML': 'svelte',
  'SVELTE-INPUT': 'svelte',
  'SVELTE-SELF-CLOSING-DIVID': 'svelte',
  'SVELTE-UNSUPPORTED-NODE': 'svelte',
  'SVELTE-UNSUPPORTED-EXPRESSION': 'svelte',
  'SVELTE-IMPORT': 'svelte',
  'SVELTE-RUNE': 'svelte',
  'DOM-GLOBAL': 'dom',
  'DOM-API': 'dom',
  'DOM-FUNCTION': 'dom',
  'ELEMENT-REJECTED': 'element',
  'ELEMENT-UNSUPPORTED': 'element',
  'CSS-PROPERTY': 'css',
  'CSS-AT-RULE': 'css',
  'CSS-SELECTOR': 'css',
  'CSS-SCOPE-COLLISION': 'css',
  'CSS-UNKNOWN': 'css',
};

/**
 * Walk compiles one module.
 *
 * `source` is the raw file content; `file` is its project-relative path and
 * appears in every finding so a build error points at a real location.
 */
export function walk(
  source: string,
  file: string,
  report: Report,
  opts: WalkOptions,
): CompiledModule {
  const ast = parse(source, { modern: true, filename: file }) as unknown as Node;

  const ops = new Ops(report, opts.ids ?? new IdGen());

  // Every module contributes one wrapper element. For the entry module this is
  // the app's single tree root; for a child it is the element the parent's
  // <Child /> slot attaches, which is why a composed app still has one root.
  const root = ops.nextNode();
  ops.createElement(root, 'div');

  // Browser globals are rejected anywhere they appear, so the walk checks the
  // instance script before anything else.
  checkGlobals(ast, file, report);

  const props = collectProps(ast, file, report);
  const lifecycle = collectLifecycle(ast, file, report);
  const runes = collectRunes(ast, file, report);

  walkFragment(getFragment(ast), root, file, report, ops, opts);

  const css = collectCss(ast, file, report, opts);

  const components = new Map<string, string>();
  for (const [key, specifier] of COMPONENT_SPECIFIERS) {
    if (!key.startsWith(`${file}:`)) continue;
    components.set(key.slice(file.length + 1), factoryNameFor(specifier));
  }

  const generated = generateFactory(file, getScope(ast, 'instance'), getFragment(ast), components);

  return {
    file,
    entry: root,
    ops: ops.ops,
    css,
    props,
    lifecycle,
    runes,
    render: generated.source,
  };
}

function getFragment(ast: Node): Node {
  const frag = (ast as { fragment?: Node }).fragment;
  return frag ?? ast;
}

/** walkFragment renders one Svelte fragment into ops under parentId. */
function walkFragment(
  node: Node,
  parentId: number,
  file: string,
  report: Report,
  ops: Ops,
  opts: WalkOptions = { validateCss: () => {} },
): void {
  const verdict = classify(node);
  if (verdict.kind === 'drop') return;
  if (verdict.kind === 'unknown') {
    finding(
      report,
      'SVELTE-UNSUPPORTED-NODE',
      node,
      file,
      `TypeScript shape ${verdict.shape} is not handled by the adapter`,
    );
    return;
  }
  if (verdict.kind === 'expression') {
    if (verdict.node) walkFragment(verdict.node, parentId, file, report, ops, opts);
    return;
  }

  switch (node.type) {
    case 'Text': {
      const data = String(node.data ?? '');
      // Whitespace-only text carries no layout meaning in a block/flex subset;
      // emitting it would create a zero-size text node per gap.
      if (data.trim() === '') return;
      const id = ops.nextNode();
      ops.createText(id, data.trim());
      ops.appendChild(parentId, id);
      return;
    }

    case 'ExpressionTag': {
      const expr = node.expression as Node | undefined;
      if (!expr) return;
      const id = ops.nextNode();
      const literal = staticString(expr);
      if (literal === null) {
        // A dynamic interpolation is rendered by the emitted runtime, which
        // submits a setText op for this id. The op itself is emitted by the
        // runtime, not the static walk.
        ops.createText(id, '');
        ops.appendChild(parentId, id);
        return;
      }
      ops.createText(id, literal);
      ops.appendChild(parentId, id);
      return;
    }

    case 'RegularElement':
      walkElement(node, parentId, file, report, ops, opts);
      return;

    case 'Component': {
      // A child component renders inside its parent's node. Its ops were
      // already emitted (dependencies compile first), so all that is left is to
      // attach its entry to the slot the parent made for it.
      const name = String((node as { name?: unknown }).name ?? '');
      const specifier = componentSpecifier(node, name, file, report);
      const childEntry = specifier ? opts.componentEntry?.(specifier) ?? null : null;
      if (childEntry === null) {
        finding(
          report,
          'SVELTE-IMPORT',
          node,
          file,
          `component <${name}> could not be resolved`,
        );
        return;
      }
      ops.appendChild(parentId, childEntry);
      return;
    }

    case 'Fragment': {
      for (const child of children(node)) walkFragment(child, parentId, file, report, ops, opts);
      return;
    }

    case 'IfBlock': {
      // Both branches are emitted; the runtime toggles them with
      // removeChild/appendChild. Laying out both at mount keeps the op stream
      // a pure build of the tree and lets the first render be synchronous.
      walkFragment(node.consequent as Node, parentId, file, report, ops, opts);
      if (node.alternate) {
        walkFragment(node.alternate as Node, parentId, file, report, ops, opts);
      }
      return;
    }

    case 'EachBlock': {
      const body = getFragmentOf(node.body as Node);
      walkFragment(body, parentId, file, report, ops, opts);
      return;
    }

    case 'KeyBlock': {
      walkFragment(getFragmentOf(node), parentId, file, report, ops);
      return;
    }

    case 'SvelteWindow':
    case 'SvelteDocument':
    case 'SvelteBody':
      finding(
        report,
        'SVELTE-WINDOW',
        node,
        file,
        `${String(node.type).replace('Svelte', 'svelte:')} has no runtime counterpart`,
      );
      return;

    case 'SvelteHead':
      finding(report, 'SVELTE-HEAD', node, file, '<svelte:head> has no runtime document');
      return;

    case 'SvelteElement':
      finding(
        report,
        'SVELTE-UNSUPPORTED-NODE',
        node,
        file,
        '<svelte:element> requires dynamic tag names',
      );
      return;

    case 'SvelteComponent':
      finding(
        report,
        'SVELTE-UNSUPPORTED-NODE',
        node,
        file,
        '<svelte:component> requires dynamic components',
      );
      return;

    case 'SlotElement':
    case 'Slot':
    case 'SlotTemplate':
      finding(report, 'SVELTE-SLOT', node, file, 'slots are replaced by props');
      return;

    case 'SnippetBlock':
      finding(report, 'SVELTE-SLOT', node, file, 'snippets are replaced by props');
      return;

    case 'RenderTag':
      finding(report, 'SVELTE-SLOT', node, file, '{@render} is replaced by props');
      return;

    case 'AwaitBlock':
      finding(report, 'SVELTE-AWAIT', node, file, '{#await} has no promise to await');
      return;

    case 'ConstTag':
      // A compile-time constant; its value is inlined by Svelte itself.
      return;

    case 'Comment':
      return;

    default:
      finding(
        report,
        'SVELTE-UNSUPPORTED-NODE',
        node,
        file,
        `node type ${node.type} is outside the M5 subset`,
      );
  }
}

function getFragmentOf(node: Node): Node {
  const frag = (node as { fragment?: Node }).fragment;
  return frag ?? { type: 'Fragment', nodes: children(node) };
}

function children(node: Node): Node[] {
  const list = (node as { nodes?: Node[]; children?: Node[] }).nodes ??
    (node as { children?: Node[] }).children;
  return Array.isArray(list) ? list : [];
}

function walkElement(
  node: Node,
  parentId: number,
  file: string,
  report: Report,
  ops: Ops,
  opts: WalkOptions,
): void {
  const tag = String(node.name);
  if (!ELEMENTS.has(tag)) {
    finding(
      report,
      'ELEMENT-REJECTED',
      node,
      file,
      `<${tag}> is outside the M5 element subset`,
    );
    return;
  }

  const id = ops.nextNode();
  ops.createElement(id, tag);
  ops.appendChild(parentId, id);

  // Focusable elements get a tabindex so the runtime's focus traversal can
  // reach them without a separate input model.
  if (FOCUSABLE.has(tag)) ops.setAttribute(id, 'tabindex', '0');

  walkAttributes(node, id, file, report, ops);
  walkFragment(getFragmentOf(node), id, file, report, ops, opts);
}

/**
 * componentSpecifier recovers the import specifier for a component node.
 *
 * Svelte's AST does not keep the specifier on the Component node, so the
 * module's own imports are consulted by the imported name.
 */
function componentSpecifier(
  node: Node,
  name: string,
  file: string,
  report: Report,
): string | null {
  const spec = COMPONENT_SPECIFIERS.get(`${file}:${name}`);
  if (!spec) {
    finding(
      report,
      'SVELTE-IMPORT',
      node,
      file,
      `no import found for <${name}>; add 'import ${name} from "./${name}.svelte"'`,
    );
    return null;
  }
  return spec;
}

/**
 * COMPONENT_SPECIFIERS maps "file:componentName" → import specifier, filled
 * by index.ts before each module walk.
 */
export const COMPONENT_SPECIFIERS = new Map<string, string>();

function walkAttributes(
  node: Node,
  id: number,
  file: string,
  report: Report,
  ops: Ops,
): void {
  const attributes = (node as { attributes?: Node[] }).attributes ?? [];

  for (const attr of attributes) {
    switch (attr.type) {
      case 'Attribute': {
        const name = String(attr.name);
        // `onclick={fn}` and `bind:value={v}` parse as plain attributes in
        // Svelte 5's modern AST, not as directives, so the event and bind
        // tables are consulted here. A handler whose name is in neither table
        // is an unsupported event, not an unsupported attribute.
        const event = EVENTS[name];
        if (event) {
          ops.addEventListener(id, event);
          continue;
        }
        const bind = BINDS[name];
        if (bind) {
          ops.addEventListener(id, bind);
          continue;
        }
        if (name.startsWith('on')) {
          finding(
            report,
            'SVELTE-UNSUPPORTED-NODE',
            attr,
            file,
            `event ${name} is not part of the M5 event subset`,
          );
          continue;
        }
        if (name.startsWith('bind:')) {
          finding(
            report,
            'SVELTE-UNSUPPORTED-NODE',
            attr,
            file,
            `${name} is not supported (only bind:value on <input>)`,
          );
          continue;
        }
        if (!ATTRIBUTES.has(name)) {
          finding(
            report,
            'ELEMENT-UNSUPPORTED',
            attr,
            file,
            `attribute ${name} is not part of the M5 subset`,
          );
          continue;
        }
        if (attr.value === true) {
          if (!BOOLEAN_ATTRIBUTES.has(name)) {
            finding(
              report,
              'ELEMENT-UNSUPPORTED',
              attr,
              file,
              `attribute ${name} needs a value`,
            );
            continue;
          }
          ops.setAttribute(id, name, '');
          continue;
        }
        const literal = staticString(attr.value as Node);
        if (literal === null) {
          // A dynamic attribute is written by the emitted runtime rather than
          // baked into the mount batch. Both `class="…{expr}"` and
          // `<Child {items}>` shorthand land here. Emitting an empty value
          // keeps the mount stream valid and lets the first runtime pass fill
          // it in, which is why this is not a finding.
          if (name === 'class') ops.setAttribute(id, 'class', '');
          continue;
        }
        ops.setAttribute(id, name, literal);
        continue;
      }

      case 'Spread':
      case 'SpreadAttribute': {
        finding(report, 'SVELTE-SPREAD', attr, file, '{...spread} is not supported');
        continue;
      }

      case 'ClassDirective': {
        const name = String(attr.name ?? 'active');
        ops.setAttribute(id, `class:${name}`, 'true');
        continue;
      }

      case 'StyleDirective': {
        finding(
          report,
          'CSS-PROPERTY',
          attr,
          file,
          `style:${nameOfDirective(attr)} is validated at compile time instead`,
        );
        continue;
      }

      case 'OnDirective': {
        const name = String(attr.name ?? '');
        const event = EVENTS[name];
        if (!event) {
          finding(
            report,
            'SVELTE-UNSUPPORTED-NODE',
            attr,
            file,
            `event ${name} is not part of the M5 event subset`,
          );
          continue;
        }
        ops.addEventListener(id, event);
        continue;
      }

      case 'BindDirective': {
        const name = String(attr.name ?? '');
        const event = BINDS[name];
        if (!event) {
          finding(
            report,
            'SVELTE-UNSUPPORTED-NODE',
            attr,
            file,
            `bind:${name} is not supported (only bind:value on <input>)`,
          );
          continue;
        }
        ops.addEventListener(id, event);
        continue;
      }

      case 'TransitionDirective':
      case 'AnimateDirective':
        finding(
          report,
          'SVELTE-TRANSITION',
          attr,
          file,
          `${attr.type.toLowerCase()} is not supported`,
        );
        continue;

      case 'UseDirective':
        finding(
          report,
          'SVELTE-UNSUPPORTED-NODE',
          attr,
          file,
          `use:${String(attr.name ?? '')} actions are not supported`,
        );
        continue;

      case 'LetDirective':
        finding(report, 'SVELTE-SLOT', attr, file, 'let: directives need slots');
        continue;

      default:
        finding(
          report,
          'ELEMENT-UNSUPPORTED',
          attr,
          file,
          `attribute node ${attr.type} is not supported`,
        );
    }
  }
}

function nameOfDirective(attr: Node): string {
  const name = (attr as { name?: unknown }).name;
  return typeof name === 'string' ? name : 'property';
}

/**
 * staticString returns the literal value of a text/attribute value node, or
 * null when it is dynamic. A dynamic value is left to the emitted runtime.
 */
function staticString(node: Node | undefined): string | null {
  if (!node) return null;
  if (node.type === 'Text') return String(node.data ?? '');
  if (node.type === 'Literal') {
    const v = (node as { value?: unknown }).value;
    return typeof v === 'string' || typeof v === 'number' ? String(v) : null;
  }
  if (node.type === 'ExpressionTag') return staticString(node.expression as Node);
  return null;
}

/** recordComponentSpecifiers notes which module supplies which component name. */
export function recordComponentSpecifiers(file: string, ast: Node): void {
  const scopes = [
    (ast as { module?: { content?: { body?: Node[] } } }).module,
    (ast as { instance?: { content?: { body?: Node[] } } }).instance,
  ];
  for (const scope of scopes) {
    for (const stmt of scope?.content?.body ?? []) {
      if (stmt.type !== 'ImportDeclaration') continue;
      if ((stmt as { importKind?: unknown }).importKind === 'type') continue;
      const source = String((stmt as { source?: { value?: unknown } }).source?.value ?? '');
      if (!source.endsWith('.svelte')) continue;
      for (const spec of (stmt as { specifiers?: Node[] }).specifiers ?? []) {
        const name =
          spec.type === 'ImportDefaultSpecifier'
            ? String((spec as { local?: { name?: unknown } }).local?.name ?? '')
            : '';
        if (name) COMPONENT_SPECIFIERS.set(`${file}:${name}`, source);
      }
    }
  }
}

/** collectProps records $props() destructuring so the runtime can bind them. */
function collectProps(node: Node, file: string, report: Report): CompiledModule['props'] {
  const out: CompiledModule['props'] = [];
  const instance = getScope(node, 'instance');
  if (!instance) return out;

  stripTypes(instance, () => {
    /* visit every node; property extraction happens on the declaration below */
  });

  const body = getProgramBody(instance);
  for (const stmt of body) {
    const decl = stmt as {
      type?: string;
      declarations?: Array<{ id?: Node; init?: Node }>;
    };
    if (decl.type !== 'VariableDeclaration') continue;
    for (const d of decl.declarations ?? []) {
      if (!d.init || d.init.type !== 'CallExpression') continue;
      const callee = d.init.callee as Node | undefined;
      if (callee?.type !== 'Identifier' || !RUNES.has(String(callee.name))) continue;
      if (callee.name !== '$props') continue;

      const pattern = d.id;
      if (pattern?.type === 'ObjectPattern') {
        for (const prop of children(pattern)) {
          if (prop.type !== 'Property') continue;
          const key = prop.key as Node | undefined;
          const name = String(key?.type === 'Identifier' ? key.name : (key?.value ?? ''));
          out.push({ name, expression: defaultOf(prop) });
        }
      } else if (pattern?.type === 'Identifier') {
        out.push({ name: String(pattern.name), expression: 'undefined' });
      } else {
        finding(report, 'SVELTE-RUNE', stmt, file, 'unsupported $props() binding');
      }
    }
  }
  return out;
}

function defaultOf(prop: Node): string {
  const value = (prop as { value?: Node }).value;
  if (!value || value.type === 'AssignmentPattern') {
    const right = (value as { right?: Node } | undefined)?.right;
    return right ? printExpression(right) : 'undefined';
  }
  return printExpression(value);
}

/** collectLifecycle reports whether the module uses onMount/onDestroy. */
function collectLifecycle(node: Node, file: string, report: Report): boolean {
  let found = false;
  walkScope(node, (n) => {
    if (n.type === 'CallExpression') {
      const callee = n.callee as Node | undefined;
      if (callee?.type === 'Identifier' && LIFECYCLE.has(String(callee.name))) found = true;
    }
    if (n.type === 'ImportDeclaration') {
      checkImport(n, file, report);
    }
  });
  return found;
}

/** collectRunes reports which runes the module uses. */
function collectRunes(node: Node, file: string, report: Report): boolean {
  let used = false;
  walkScope(node, (n) => {
    if (n.type !== 'Identifier') return;
    const name = String(n.name);
    if (!name.startsWith('$')) return;
    if (RUNES.has(name)) {
      used = true;
      return;
    }
    finding(report, 'SVELTE-RUNE', n, file, `rune ${name} is not supported`);
  });
  return used;
}

function checkImport(n: Node, file: string, report: Report): void {
  // A type-only import is erased by the compiler and has no module to compile,
  // so it is not a finding (D-2).
  if ((n as { importKind?: unknown }).importKind === 'type') return;
  const source = String((n as { source?: { value?: unknown } }).source?.value ?? '');
  if (source === 'svelte') return;
  if (source === 'svelte/transition' || source === 'svelte/animate') {
    finding(report, 'SVELTE-TRANSITION', n, file, `${source} is not supported`);
    return;
  }
  if (source.startsWith('.')) {
    // A relative import is only in-subset when it names a .svelte module; a
    // .ts helper has no compiled form, so it is refused here rather than
    // failing later with a confusing missing-module error.
    if (source.endsWith('.svelte') || !/\.[cm]?[jt]sx?$/.test(source)) return;
    finding(
      report,
      'SVELTE-IMPORT',
      n,
      file,
      `only .svelte modules can be imported (got ${source}); move the code into the component`,
    );
    return;
  }
  finding(
    report,
    'SVELTE-IMPORT',
    n,
    file,
    `only relative .svelte imports and "svelte" lifecycle helpers are supported (got ${source})`,
  );
}

/**
 * collectCss extracts and validates component styles.
 *
 * Rules keep their selector text. The AST prelude is a structured selector
 * tree; printing it back to text is what lets the Go stylesheet be parsed by
 * internal/style, whose contract is plain CSS text (docs/CSS-SUBSET.md).
 */
function collectCss(
  node: Node,
  file: string,
  report: Report,
  opts: WalkOptions,
): CssRule[] {
  const css = (node as { css?: Node }).css;
  if (!css) return [];

  const rules: CssRule[] = [];

  const walkCss = (n: unknown): void => {
    if (Array.isArray(n)) {
      n.forEach(walkCss);
      return;
    }
    if (!n || typeof n !== 'object') return;
    const current = n as Node;
    if (typeof current.type !== 'string') return;

    if (current.type === 'Atrule') {
      finding(
        report,
        'CSS-AT-RULE',
        current,
        file,
        `@${String((current as { name?: unknown }).name ?? '')} is not in the CSS subset`,
      );
      return;
    }

    if (current.type === 'Rule') {
      const printed = printSelectorList(current.prelude as Node | undefined, opts.scope ?? null);
      const declarations = declarationsOf(current);
      if ('unsupported' in printed) {
        finding(report, 'CSS-SELECTOR', current, file, printed.unsupported);
        return;
      }
      // Validation is delegated so the subset rules live in one place; a
      // rejected rule is an explicit build failure, never a silent drop.
      opts.validateCss({ selector: printed.text, declarations }, file, current.loc?.start.line ?? 0);
      rules.push({ selector: printed.text, declarations });
      return;
    }

    for (const key of Object.keys(current)) {
      if (key === 'loc' || key === 'prelude') continue;
      walkCss((current as Record<string, unknown>)[key]);
    }
  };

  for (const child of children(css)) walkCss(child);
  return rules;
}

/** declarationsOf lists a rule's declarations in source order. */
function declarationsOf(rule: Node): Array<{ property: string; value: string }> {
  const block = (rule as { block?: Node }).block;
  const declarations: Array<{ property: string; value: string }> = [];
  const walk = (n: unknown): void => {
    if (Array.isArray(n)) {
      n.forEach(walk);
      return;
    }
    if (!n || typeof n !== 'object') return;
    const current = n as Node;
    if (current.type === 'Declaration') {
      declarations.push({
        property: String((current as { property?: unknown }).property ?? ''),
        value: String((current as { value?: unknown }).value ?? '').trim(),
      });
      return;
    }
    for (const key of Object.keys(current)) {
      if (key === 'loc') continue;
      walk((current as Record<string, unknown>)[key]);
    }
  };
  if (block) walk(block);
  return declarations;
}

/** Printed is a successful selector render. */
interface Printed {
  /** The compound up to and including its id/class chain, without pseudo parts. */
  text: string;
  /** Trailing pseudo-class/element, e.g. `:hover`. The scope goes before it. */
  pseudo: string;
}

/** Unprintable says *why* a selector could not be rendered.
 *
 * The reason matters: reporting "could not be printed" for a `:last-child`
 * rule reads like an internal adapter bug, when the truth is that the
 * construct is outside the CSS subset. An honest message is the whole point of
 * a gap register (G-UPG-04).
 */
interface Unprintable {
  unsupported: string;
}

type PrintResult = Printed | Unprintable;

/** CombinatorResult carries no pseudo part: a combinator is only ever a gap. */
type CombinatorResult = { text: string } | Unprintable;

function unprintable(reason: string): Unprintable {
  return { unsupported: reason };
}

/**
 * combinatorOf prints the combinator that precedes a relative selector.
 *
 * The AST holds a combinator as a *node*, not a string, so coercing it with
 * String() produced the literal text `[object Object]` in the emitted
 * stylesheet — a descendant rule compiled to a selector Go cannot match, and
 * the rule was dropped without a word. Found while scoping the dashboard sample.
 *
 * Only the descendant combinator is supported: internal/style splits a selector
 * on whitespace (docs/CSS-SUBSET.md). The others are named explicitly rather
 * than emitted, so an author learns the rule is dropped instead of wondering
 * why it had no effect.
 */
function combinatorOf(rel: Node): CombinatorResult {
  const raw = (rel as { combinator?: { name?: unknown } | null }).combinator;
  if (raw === null || raw === undefined) return { text: '' };
  // With `modern: true` the operator is the combinator's `name` (' ', '>', '+',
  // '~'); `type` is always the literal "Combinator".
  switch (String(raw.name ?? '')) {
    case ' ':
      return { text: ' ' };
    case '>':
      return unprintable(
        'the child combinator (>) is not in the CSS subset (descendant only)',
      );
    case '+':
      return unprintable(
        'the adjacent-sibling combinator (+) is not in the CSS subset (descendant only)',
      );
    case '~':
      return unprintable(
        'the general-sibling combinator (~) is not in the CSS subset (descendant only)',
      );
    default:
      return unprintable(
        `combinator ${JSON.stringify(String(raw.name ?? ''))} is not understood by the adapter`,
      );
  }
}

/**
 * printSelectorList renders a structured selector prelude back to CSS text,
 * attaching the component scope class to the last compound of each selector —
 * the same subject-scoped form Svelte emits.
 */
function printSelectorList(prelude: Node | undefined, scope: string | null): PrintResult {
  if (!prelude || prelude.type !== 'SelectorList') {
    return unprintable('selector list shape is not understood by the adapter');
  }
  const selectors: string[] = [];
  for (const complex of children(prelude)) {
    if (complex.type !== 'ComplexSelector') {
      return unprintable('selector combinator shape is not understood by the adapter');
    }
    const parts: string[] = [];
    const rels = children(complex);
    for (let i = 0; i < rels.length; i++) {
      const rel = rels[i]!;
      if (rel.type !== 'RelativeSelector') {
        return unprintable('relative selector shape is not understood by the adapter');
      }
      const combinator = combinatorOf(rel);
      if ('unsupported' in combinator) return combinator;
      const rendered = renderSimpleSelector(rel);
      if ('unsupported' in rendered) return rendered;
      // The scope belongs on the last compound, because that compound is the
      // subject — and it goes *before* any pseudo-class, so `.row` scopes to
      // `.row.s-app:hover`, not `.row:hover.s-app`.
      const suffix = i === rels.length - 1 && scope ? `.${scope}` : '';
      parts.push(`${combinator.text}${rendered.text}${suffix}${rendered.pseudo}`);
    }
    const text = parts.join('').trim();
    if (text === '') return unprintable('empty selector');
    selectors.push(text);
  }
  return selectors.length > 0
    ? { text: selectors.join(', '), pseudo: '' }
    : unprintable('empty selector list');
}

/**
 * renderSimpleSelector prints one RelativeSelector's type/class/id chain.
 *
 * The chain lives under `.selectors`, not `.children` — reading the wrong field
 * is why an early version reported every selector as unprintable.
 */
function renderSimpleSelector(rel: Node): PrintResult {
  let out = '';
  // Pseudo parts are held back so the caller can place the scope class before
  // them: `.row` + `.s-app` + `:hover`, never `.row:hover.s-app`.
  let pseudo = '';
  const chain = ((rel as { selectors?: Node[] }).selectors ?? []) as Node[];
  for (const sel of chain) {
    switch (sel.type) {
      case 'TypeSelector': {
        // A universal selector is written *.
        const name = (sel as { name?: unknown }).name;
        if (name === null || name === undefined) {
          out += '*';
          continue;
        }
        // In a component's <style>, a bare identifier that is not a known HTML
        // tag names a *class* — that is how Svelte authors write `shell { }` for
        // `<section class="shell">`. Printing it as a tag selector would match
        // nothing and silently drop every rule.
        const tag = String(name);
        out += HTML_TAGS.has(tag) ? tag : `.${tag}`;
        continue;
      }
      case 'ClassSelector':
        out += `.${String((sel as { name?: unknown }).name ?? '')}`;
        continue;
      case 'IdSelector':
        out += `#${String((sel as { name?: unknown }).name ?? '')}`;
        continue;
      case 'AttributeSelector':
        out += printAttributeSelector(sel);
        continue;
      case 'PseudoClassSelector': {
        const name = String((sel as { name?: unknown }).name ?? '');
        // :hover/:active/:focus are the M3 pseudo-classes the subset supports;
        // anything else is outside the subset and must say so by name.
        if (!['hover', 'active', 'focus'].includes(name)) {
          return unprintable(
            `pseudo-class :${name} is not in the CSS subset (only :hover, :active and :focus)`,
          );
        }
        pseudo += `:${name}`;
        continue;
      }
      case 'PseudoElementSelector': {
        const name = String((sel as { name?: unknown }).name ?? '');
        return unprintable(
          `pseudo-element ::${name} is not in the CSS subset (no generated content)`,
        );
      }
      default:
        return unprintable(
          `selector part ${sel.type} is not understood by the adapter`,
        );
    }
  }
  return out === '' ? unprintable('empty selector') : { text: out, pseudo };
}

/** printAttributeSelector prints `[name]` or `[name="value"]`. */
function printAttributeSelector(sel: Node): string {
  const name = String((sel as { name?: unknown }).name ?? '');
  const matcher = (sel as { matcher?: unknown }).matcher;
  const value = (sel as { value?: { value?: unknown } | null }).value;
  if (!matcher) return `[${name}]`;
  const op = String(matcher);
  const raw = value?.value;
  if (raw === null || raw === undefined) return `[${name}]`;
  return `[${name}${op === '=' ? '' : ` ${op}`} "${String(raw)}"]`;
}

/** checkGlobals rejects browser globals wherever they appear in a module. */
function checkGlobals(node: Node, file: string, report: Report): void {
  walkScope(node, (n) => {
    if (n.type === 'ImportDeclaration') return;
    if (n.type === 'MemberExpression') {
      const object = (n as { object?: Node }).object;
      if (object?.type === 'Identifier' && BROWSER_GLOBALS.has(String(object.name))) {
        finding(
          report,
          'DOM-GLOBAL',
          n,
          file,
          `${String(object.name)} is not available in the runtime`,
        );
      }
      return;
    }
    if (n.type === 'Identifier') {
      const name = String(n.name);
      if (BROWSER_GLOBALS.has(name)) {
        finding(report, 'DOM-GLOBAL', n, file, `${name} is not available in the runtime`);
      }
    }
  });
}

// --- AST helpers ---------------------------------------------------------

function getScope(ast: Node, scope: 'instance' | 'module'): Node | null {
  return (ast as Record<string, Node | undefined>)[scope] ?? null;
}

function getProgramBody(scope: Node): Node[] {
  const content = (scope as { content?: Node }).content;
  const body = (content as { body?: Node[] } | undefined)?.body;
  return Array.isArray(body) ? body : [];
}

/** walkNode visits every node in a subtree, including the root. */
function walkNode(node: unknown, visit: (n: Node) => void): void {
  if (Array.isArray(node)) {
    for (const c of node) walkNode(c, visit);
    return;
  }
  if (!node || typeof node !== 'object') return;
  const n = node as Node;
  if (typeof n.type !== 'string') return;
  visit(n);
  for (const key of Object.keys(n)) {
    if (key === 'loc' || key === 'parent') continue;
    walkNode((n as Record<string, unknown>)[key], visit);
  }
}

/** walkScope visits the instance and module scripts plus the fragment. */
function walkScope(ast: Node, visit: (n: Node) => void): void {
  for (const scope of [getScope(ast, 'instance'), getScope(ast, 'module')]) {
    if (scope) walkNode(scope, visit);
  }
  walkNode(getFragment(ast), visit);
}

/**
 * printExpression renders an expression back to source text for the emitted
 * runtime. Svelte's own printer is deliberately not used: it would reintroduce
 * a dependency on compiler internals, which DRR-007 rejected.
 */
export function printExpression(node: Node): string {
  switch (node.type) {
    case 'Identifier':
      return String(node.name);
    case 'Literal': {
      const v = (node as { value?: unknown }).value;
      if (typeof v === 'string') return JSON.stringify(v);
      if (v === null) return 'null';
      if (typeof v === 'bigint') return `${String(v)}n`;
      return String(v);
    }
    case 'TemplateLiteral': {
      const quasis = ((node as { quasis?: Node[] }).quasis ?? [])
        .map((q) => String((q as { value?: { raw?: string } }).value?.raw ?? ''))
        .join('${EXPR}');
      // Template literals are rewritten to concatenation by the runtime.
      return quasis;
    }
    case 'MemberExpression': {
      const object = printExpression((node as { object?: Node }).object as Node);
      const prop = node.property as Node;
      if (!node.computed && prop?.type === 'Identifier') {
        return `${object}.${String(prop.name)}`;
      }
      return `${object}[${printExpression(prop)}]`;
    }
    case 'CallExpression': {
      const callee = printExpression((node as { callee?: Node }).callee as Node);
      const args = ((node as { arguments?: Node[] }).arguments ?? [])
        .map(printExpression)
        .join(', ');
      return `${callee}(${args})`;
    }
    case 'BinaryExpression': {
      const op = String((node as { operator?: unknown }).operator ?? '+');
      return `${printExpression((node as { left?: Node }).left as Node)} ${op} ${printExpression((node as { right?: Node }).right as Node)}`;
    }
    case 'LogicalExpression': {
      const op = String((node as { operator?: unknown }).operator ?? '&&');
      return `${printExpression((node as { left?: Node }).left as Node)} ${op} ${printExpression((node as { right?: Node }).right as Node)}`;
    }
    case 'UnaryExpression': {
      const op = String((node as { operator?: unknown }).operator ?? '');
      return `${op}${printExpression((node as { argument?: Node }).argument as Node)}`;
    }
    case 'ConditionalExpression': {
      return `${printExpression((node as { test?: Node }).test as Node)} ? ${printExpression((node as { consequent?: Node }).consequent as Node)} : ${printExpression((node as { alternate?: Node }).alternate as Node)}`;
    }
    case 'ParenthesizedExpression':
      return printExpression((node as { expression?: Node }).expression as Node);
    case 'ArrowFunctionExpression': {
      const params = ((node as { params?: Node[] }).params ?? [])
        .map(printExpression)
        .join(', ');
      return `(${params}) => ${printExpression((node as { body?: Node }).body as Node)}`;
    }
    case 'ArrayExpression':
      return `[${((node as { elements?: Node[] }).elements ?? []).map(printExpression).join(', ')}]`;
    case 'ObjectExpression':
      return `{ ${((node as { properties?: Node[] }).properties ?? [])
        .filter((p) => p.type === 'Property')
        .map((p) => {
          const key = p.key as Node;
          const value = (p as { value?: Node }).value as Node;
          const k = key.type === 'Identifier' ? String(key.name) : printExpression(key);
          return `${k}: ${printExpression(value)}`;
        })
        .join(', ')} }`;
    default:
      return String(node.type);
  }
}

/**
 * compileErrorOn converts the first finding into a CompileError. Strict mode
 * calls this after the walk so every walk shares one implementation.
 */
export function firstFinding(report: Report): Finding | undefined {
  return report.all[0];
}

export { parse };
