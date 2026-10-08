/**
 * Code generation: Svelte AST → a JavaScript render function.
 *
 * This is the compile backend behind the re-render model. Each module becomes
 * one factory function that returns a fresh node tree; the runtime diffs it
 * against the previous tree and submits the difference through `ui.apply`.
 *
 * The generated code is deliberately plain — `el(...)`, `txt(...)`,
 * `frag(...)`, `__each(...)` — so no helper hides control flow. That matters
 * because every construct outside the subset has to fail at compile time rather
 * than misbehave at runtime (G-UPG-04), and readable output is what makes the
 * emitted bundle auditable.
 *
 * Runes are compiled, not interpreted:
 * - `$state(x)`      → a runtime cell holding `x`
 * - `$derived(expr)`  → a function recomputed on every render
 * - `$effect(fn)`     → run after each flush
 * - `$props()`        → the factory's `props` argument, with defaults
 */
import { BINDS, BOOLEAN_ATTRIBUTES, ELEMENTS, EVENTS, FOCUSABLE, scopeOf } from './subset.js';
import type { Node } from './typescript.js';

/** GenContext is the state one module's generation walks with. */
export interface GenContext {
  /** Local names declared by the instance script. */
  declared: Set<string>;
  /** Function names declared by the instance script. */
  functions: Set<string>;
  /** Child factories, keyed by the imported component name. */
  components: Map<string, string>;
  /** Cell names created for `$state` declarations. */
  cells: Set<string>;
  /** Getter names created for `$derived` declarations. */
  derived: Set<string>;
  /** Every name the generated module may reference. */
  scope: Set<string>;
  /**
   * Component style scope class merged into every element's class attribute.
   * Derived from the module path with scopeOf, the same function that scopes the
   * module's CSS selectors — one source of truth for both sides.
   */
  styleScope: string;
}

/** GenResult is one generated module. */
export interface GenResult {
  /** Factory variable name. */
  name: string;
  /** Complete factory source. */
  source: string;
  /** True when the module registers effects or lifecycle hooks. */
  lifecycle: boolean;
}

/** factoryName derives a stable JS identifier from a module path. */
export function factoryName(file: string): string {
  const base = file
    .replace(/^src\//, '')
    .replace(/\.svelte$/, '')
    .replace(/[^a-zA-Z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '');
  return `_${base || 'module'}`;
}

/**
 * extractPrologue separates the instance script into the pieces codegen needs:
 * state cells, derived getters, prop destructuring, plain functions, and effects.
 */
export interface Prologue {
  /** Statements to place at the top of the factory body. */
  lines: string[];
  /** True when the module used onMount/onDestroy/$effect. */
  lifecycle: boolean;
}

/**
 * generateFactory renders one module's render function.
 *
 * `instance` is the module's instance script (already TypeScript-stripped), or
 * null when the component has no script.
 */
export function generateFactory(
  file: string,
  instance: Node | null,
  fragment: Node,
  components: Map<string, string>,
): GenResult {
  const ctx: GenContext = {
    declared: new Set(),
    functions: new Set(),
    components,
    cells: new Set<string>(),
    derived: new Set<string>(),
    scope: new Set<string>(),
    styleScope: scopeOf(file),
  };

  const body: string[] = [];
  let lifecycle = false;

  if (instance) {
    const prologue = generatePrologue(instance, ctx);
    body.push(...prologue.lines);
    lifecycle = prologue.lifecycle;
  }

  const tree = renderFragment(fragment, ctx);

  // The factory runs ONCE: it creates the state cells and returns a render
  // function. Declaring the cells inside the render body would re-create them
  // on every re-render, so a counter would reset to its initial value the
  // instant it changed — the diff would then see no difference at all.
  return {
    name: factoryName(file),
    source: [
      `// --- ${file} ---`,
      `var ${factoryName(file)} = function (props) {`,
      ...body.map((line) => (line ? `  ${line}` : '')),
      '',
      '  // State lives above this closure; only the tree is rebuilt per render.',
      `  return function () { return ${tree}; };`,
      '};',
    ].join('\n'),
    lifecycle,
  };
}

/**
 * generatePrologue renders the instance script.
 *
 * Anything the subset cannot express — a bare import of a value from another
 * module, a loop at the top level — is rejected here rather than emitted, so
 * the module fails to compile instead of producing a component whose behaviour
 * silently differs from its source.
 */
function generatePrologue(instance: Node, ctx: GenContext): Prologue {
  const lines: string[] = [];
  let lifecycle = false;

  const content = (instance as { content?: Node }).content;
  const body = (content as { body?: Node[] } | undefined)?.body ?? [];
  // Props are destructured first: they are inputs, and a cell may default to one.
  const order = [
    ...body.filter((s) => isPropsDeclaration(s)),
    ...body.filter((s) => !isPropsDeclaration(s)),
  ];

  for (const stmt of order) {
    switch (stmt.type) {
      case 'ImportDeclaration':
        // Lifecycle helpers are runtime functions the runtime provides; a
        // component import is resolved by the caller. Both are no-ops here.
        continue;

      case 'VariableDeclaration': {
        const decls = ((stmt as { declarations?: Node[] }).declarations ?? []).map((d) =>
          renderVariableDeclaration(d as Node, ctx),
        );
        for (const line of decls) lines.push(line);
        continue;
      }

      case 'FunctionDeclaration': {
        const name = String((stmt as { id?: { name?: unknown } }).id?.name ?? '');
        const params = ((stmt as { params?: Node[] }).params ?? []).map((p) =>
          paramName(p as Node),
        );
        for (const p of params) ctx.declared.add(p);
        const fnBody = (stmt as { body?: Node }).body as Node;
        ctx.functions.add(name);
        ctx.declared.add(name);
        lines.push(`function ${name}(${params.join(', ')}) {`);
        lines.push(...renderBlock(fnBody, ctx).map((l) => `  ${l}`));
        lines.push('}');
        continue;
      }

      case 'ExpressionStatement': {
        const expr = (stmt as { expression?: Node }).expression as Node;
        // `$effect(fn)` and `onMount(fn)` register for the post-flush phase.
        if (expr?.type === 'CallExpression') {
          const callee = (expr as { callee?: Node }).callee as Node;
          const name = callee?.type === 'Identifier' ? String(callee.name) : '';
          if (name === '$effect' || name === 'onMount' || name === 'onDestroy') {
            lifecycle = true;
            lines.push(`${runtimeCall(name)}(${renderArgumentList(expr as Node, ctx)});`);
            continue;
          }
        }
        lines.push(`${expression(expr, ctx)};`);
        continue;
      }

      default:
        lines.push(`${statement(stmt, ctx)};`);
    }
  }

  return { lines, lifecycle };
}

/** isPropsDeclaration reports a `let {…} = $props()` statement. */
function isPropsDeclaration(stmt: Node): boolean {
  if (stmt.type !== 'VariableDeclaration') return false;
  return ((stmt as { declarations?: Node[] }).declarations ?? []).some(
    (d) => initOf(d as Node)?.type === 'CallExpression' && isRuneCall(initOf(d as Node) as Node, '$props'),
  );
}

/** initOf returns a declarator's initializer. */
function initOf(decl: Node): Node | undefined {
  return (decl as { init?: Node }).init;
}

/** idOf returns a declarator's binding pattern. */
function idOf(decl: Node): Node | undefined {
  return (decl as { id?: Node }).id;
}

/** isRuneCall reports whether node is a call to the named rune. */
function isRuneCall(node: Node, rune: string): boolean {
  const callee = (node as { callee?: Node }).callee as Node;
  return callee?.type === 'Identifier' && String(callee.name) === rune;
}

/** renderVariableDeclaration renders one declarator, unwrapping runes. */
function renderVariableDeclaration(decl: Node, ctx: GenContext): string {
  const id = idOf(decl);
  const init = initOf(decl);

  // `let x = $state(initial)` → a runtime cell.
  if (init && init.type === 'CallExpression' && isRuneCall(init, '$state')) {
    const name = String((id as { name?: unknown } | undefined)?.name ?? '');
    ctx.cells.add(name);
    ctx.declared.add(name);
    const arg = (init as { arguments?: Node[] }).arguments?.[0];
    return `var ${name} = state(${arg ? expression(arg, ctx) : 'undefined'});`;
  }

  // `let x = $derived(expr)` → a getter recomputed every render.
  if (init && init.type === 'CallExpression' && isRuneCall(init, '$derived')) {
    const name = String((id as { name?: unknown } | undefined)?.name ?? '');
    ctx.derived.add(name);
    ctx.declared.add(name);
    const arg = (init as { arguments?: Node[] }).arguments?.[0];
    return `var ${name} = function () { return ${arg ? expression(arg, ctx) : 'undefined'}; };`;
  }

  // `let { a = 1 } = $props()` → prop reads with defaults.
  if (init && init.type === 'CallExpression' && isRuneCall(init, '$props')) {
    return `${printBinding(id as Node, ctx)};`;
  }

  if (!id) return '/* unnamed declaration */';
  const name = id.type === 'Identifier' ? String(id.name) : printBinding(id, ctx);
  ctx.declared.add(name);
  return `var ${name} = ${init ? expression(init, ctx) : 'undefined'};`;
}

/**
 * printBinding renders a binding pattern as a `var` declaration.
 *
 * A destructured pattern cannot become several `var`s in one statement, so it
 * is bound through a helper: the runtime reads each key from `props` and applies
 * the default only when the prop is absent.
 */
function printBinding(node: Node, ctx: GenContext): string {
  if (node.type === 'Identifier') {
    const name = String(node.name);
    ctx.declared.add(name);
    return `var ${name}`;
  }
  if (node.type === 'ObjectPattern') {
    // A prop read is `props.name`, so a default only applies when the prop is
    // absent — the browser's behavior for a missing prop.
    // Each binding becomes a real local, so the rest of the generated module
    // refers to `items`, not `props.items`. The default applies only when the
    // prop is absent, which is what a browser does for a missing prop.
    const parts = ((node as { properties?: Node[] }).properties ?? []).map((p) => {
      const key = p.key as Node;
      const value = (p as { value?: Node }).value as Node;
      const keyName =
        key.type === 'Identifier' ? String(key.name) : String((key as { value?: unknown }).value ?? '');
      const read = `(${JSON.stringify(keyName)} in props ? props[${JSON.stringify(keyName)}]`;
      if (value?.type === 'AssignmentPattern') {
        const local = paramName(value.left as Node);
        ctx.declared.add(local);
        return `${local} = ${read} : ${expression(value.right as Node, ctx)})`;
      }
      const local = paramName(value);
      ctx.declared.add(local);
      return `${local} = ${read} : undefined)`;
    });
    return `var ${parts.join(', ')}`;
  }
  return 'var __p = {}';
}

/** renderBlock renders a statement block body. */
function renderBlock(node: Node, ctx: GenContext): string[] {
  const body = (node as { body?: Node[] }).body ?? [];
  const out: string[] = [];
  for (const stmt of body) out.push(statement(stmt, ctx));
  return out;
}

/** renderArgumentList renders call arguments as a comma-joined source list. */
function renderArgumentList(call: Node, ctx: GenContext): string {
  return ((call as { arguments?: Node[] }).arguments ?? [])
    .map((a) => expression(a, ctx))
    .join(', ');
}

/** statement renders one statement as source. */
function statement(node: Node, ctx: GenContext): string {
  switch (node.type) {
    case 'ExpressionStatement':
      return expression((node as { expression?: Node }).expression as Node, ctx);
    case 'VariableDeclaration':
      return renderVariableDeclaration(
        ((node as { declarations?: Node[] }).declarations ?? [])[0] as Node,
        ctx,
      );
    case 'ReturnStatement':
      return `return ${node.argument ? expression(node.argument as Node, ctx) : 'undefined'}`;
    case 'IfStatement': {
      const test = expression((node as { test?: Node }).test as Node, ctx);
      const consequent = renderBlock((node as { consequent?: Node }).consequent as Node, ctx);
      const body = consequent.join(' ');
      const alternate = node.alternate
        ? ` else { ${renderBlock((node as { alternate?: Node }).alternate as Node, ctx).join(' ')} }`
        : '';
      return `if (${test}) { ${body} }${alternate}`;
    }
    case 'BlockStatement':
      return `{ ${renderBlock(node, ctx).join(' ')} }`;
    case 'FunctionDeclaration': {
      const name = String((node as { id?: { name?: unknown } }).id?.name ?? '');
      const params = ((node as { params?: Node[] }).params ?? []).map((p) => paramName(p as Node));
      for (const p of params) ctx.declared.add(p);
      const body = (node as { body?: Node }).body as Node;
      return `function ${name}(${params.join(', ')}) { ${renderBlock(body, ctx).join(' ')} }`;
    }
    case 'ForOfStatement':
    case 'ForStatement':
    case 'WhileStatement':
      return printGeneric(node);
    default:
      return printGeneric(node);
  }
}

/** runtimeCall maps a lifecycle helper onto the emitted runtime's name. */
function runtimeCall(name: string): string {
  switch (name) {
    case '$effect':
      return 'effect';
    case 'onMount':
      return 'onMount';
    case 'onDestroy':
      return 'onDestroy';
    default:
      return name;
  }
}

// --- template rendering -------------------------------------------------

/** renderFragment renders a node list as a node-tree expression. */
function renderFragment(node: Node, ctx: GenContext): string {
  const parts = children(node).map((c) => renderNode(c, ctx)).filter((s): s is string => s !== null);
  if (parts.length === 0) return 'frag([])';
  if (parts.length === 1) return parts[0]!;
  return `frag([${parts.join(', ')}])`;
}

/** renderNode renders one node, or null when it contributes nothing. */
function renderNode(node: Node, ctx: GenContext): string | null {
  switch (node.type) {
    case 'Text': {
      const data = String(node.data ?? '');
      // Whitespace-only text has no layout meaning in a block/flex subset;
      // emitting it would create a zero-size text node per gap.
      if (data.trim() === '') return null;
      return `txt(${JSON.stringify(data.trim())})`;
    }
    case 'ExpressionTag': {
      const expr = node.expression as Node | undefined;
      if (!expr) return null;
      return `txt(${expression(expr, ctx)})`;
    }
    case 'RegularElement':
      return renderElement(node, ctx);
    case 'Component': {
      const name = String((node as { name?: unknown }).name ?? '');
      const factory = ctx.components.get(name);
      if (!factory) return null;
      // A child factory returns a render function, so it is called once to build
      // it and once to render it.
      return `${factory}(${renderComponentProps(node, ctx)})()`;
    }
    case 'Fragment':
      return renderFragment(node, ctx);
    case 'IfBlock':
      return renderIf(node, ctx);
    case 'EachBlock':
      return renderEach(node, ctx);
    case 'KeyBlock':
      // {#key} is a remount hint; the subset re-renders from state anyway, so
      // it is transparent rather than faked (docs/SVELTE.md §divergences).
      return renderFragment(node, ctx);
    case 'ConstTag':
      return null;
    case 'Comment':
      return null;
    default:
      return null;
  }
}

/** renderIf renders a conditional: only the taken branch becomes a node. */
function renderIf(node: Node, ctx: GenContext): string {
  const test = node.test ? expression(node.test as Node, ctx) : 'false';
  const consequent = renderFragment(asFragment(node.consequent as Node), ctx);
  const alternate = node.alternate
    ? renderFragment(asFragment(node.alternate as Node), ctx)
    : 'frag([])';
  return `(${test} ? ${consequent} : ${alternate})`;
}

/** renderEach renders a loop with keyed reconciliation. */
function renderEach(node: Node, ctx: GenContext): string {
  const subject = node.expression ? expression(node.expression as Node, ctx) : '[]';
  const index = String((node as { index?: unknown }).index ?? 'i');
  const itemNode = (node as { context?: { name?: unknown } | null }).context;
  const item = String(itemNode?.name ?? 'item');

  // A keyed body must not reference names from the enclosing scope that the
  // loop shadows, so the body renders in a child context.
  const inner: GenContext = { ...ctx, declared: new Set(ctx.declared) };
  inner.declared.add(item);
  inner.declared.add(index);

  const body = renderFragment(asFragment(node.body as Node), inner);
  const keyExpr = node.key ? expression(node.key as Node, inner) : index;

  return `each(${subject}, function (${item}, ${index}) { return ${body}; }, function (${item}, ${index}) { return ${keyExpr}; })`;
}

/** asFragment normalizes a branch body into a fragment-like node. */
function asFragment(node: Node): Node {
  return { type: 'Fragment', nodes: children(node) } as Node;
}

/** renderComponentProps renders the props object for a child component. */
function renderComponentProps(node: Node, ctx: GenContext): string {
  const props: string[] = [];
  for (const attr of ((node as { attributes?: Node[] }).attributes ?? []) as Node[]) {
    if (attr.type !== 'Attribute') continue;
    const name = String(attr.name);
    if (name.startsWith('on') && EVENTS[name]) continue;
    if (name.startsWith('bind:') && BINDS[name]) continue;
    // `<Child {items} />` is shorthand for `items={items}`.
    const shorthand = (attr as { shorthand?: boolean }).shorthand === true;
    if (shorthand) {
      props.push(`${JSON.stringify(name)}: ${name}`);
      continue;
    }
    const value = ((attr as { value?: { expression?: Node } }).value as { expression?: Node } | undefined)?.expression;
    props.push(`${JSON.stringify(name)}: ${value ? expression(value, ctx) : 'undefined'}`);
  }
  return props.length > 0 ? `{ ${props.join(', ')} }` : '{}';
}

/** renderElement renders one element with attributes, handlers and children. */
function renderElement(node: Node, ctx: GenContext): string {
  const tag = String(node.name);
  const attrs: string[] = [];
  const handlers: string[] = [];

  // Focusable elements get a tabindex so the runtime's focus traversal can
  // reach them without a separate input model.
  if (FOCUSABLE.has(tag)) attrs.push(`"tabindex": "0"`);

  const classDirectives: string[] = [];

  for (const attr of ((node as { attributes?: Node[] }).attributes ?? []) as Node[]) {
    switch (attr.type) {
      case 'Attribute': {
        const name = String(attr.name);
        const event = EVENTS[name];
        if (event) {
          handlers.push(...renderHandler(attr, event, ctx));
          continue;
        }
        const bind = BINDS[name];
        if (bind) {
          handlers.push(...renderBind(attr, bind, ctx));
          continue;
        }
        if (name === 'class') {
          const expr = ((attr as { value?: { expression?: Node } }).value as { expression?: Node } | undefined)?.expression;
          if (expr) {
            attrs.push(`"class": ${expression(expr, ctx)}`);
            continue;
          }
          const literal = staticText((attr as { value?: Node }).value as Node | undefined);
          // An empty class attribute carries no styling information; emitting
          // it would put a no-op attribute on every node.
          if (literal !== '') {
            attrs.push(`"class": ${JSON.stringify(literal ?? '')}`);
          }
          continue;
        }
        const value = (attr.value as Node | undefined);
        if ((value as unknown) === true || BOOLEAN_ATTRIBUTES.has(name)) {
          attrs.push(`${JSON.stringify(name)}: ""`);
          continue;
        }
        const literal = staticText((value as unknown));
        if (literal !== null) {
          attrs.push(`${JSON.stringify(name)}: ${JSON.stringify(literal)}`);
          continue;
        }
        const dynamic = dynamicValue(value as unknown, ctx);
        attrs.push(`${JSON.stringify(name)}: ${dynamic ?? 'undefined'}`);
        continue;
      }
      case 'ClassDirective': {
        // class: directives fold into one class expression so the CSS subset
        // needs no dynamic-class rule of its own.
        const name = String(attr.name ?? 'active');
        const test = (attr as { expression?: Node }).expression;
        classDirectives.push(
          `(${test ? expression(test, ctx) : 'true'}) ? ${JSON.stringify(` ${name}`)} : ""`,
        );
        continue;
      }
      case 'OnDirective':
      case 'BindDirective': {
        const name = String(attr.name ?? '');
        const event = attr.type === 'OnDirective' ? EVENTS[name] : BINDS[name];
        if (event) handlers.push(...renderDirective(attr, event, ctx));
        continue;
      }
      default:
        continue;
    }
  }

  // The style scope is appended to *every* element of the module, including the
  // ones the author gave no class: the module's CSS may select them by tag or
  // by a class the author set only on some siblings. Elements that already have
  // a class keep it — the scope is a token appended to the list, never a
  // replacement.
  const baseIndex = attrs.findIndex((a) => a.startsWith('"class"'));
  const baseValue = baseIndex >= 0 ? attrs[baseIndex]!.slice('"class": '.length) : '""';
  const parts = [baseValue, ...classDirectives];
  if (ctx.styleScope) parts.push(JSON.stringify(ctx.styleScope));
  if (baseIndex >= 0) attrs.splice(baseIndex, 1);
  // filter(Boolean) keeps an element with no author class from rendering a
  // leading space, which would otherwise differ only in whitespace from a
  // hand-written class list.
  attrs.push(
    `"class": [${parts.join(', ')}].filter(Boolean).join(" ")`,
  );

  const childrenSrc = children(node)
    .map((c) => renderNode(c, ctx))
    .filter((s): s is string => s !== null);

  const args = [`${JSON.stringify(tag)}`, `{${attrs.join(', ')}}`, `[${childrenSrc.join(', ')}]`];
  if (handlers.length > 0) args.push(`{${handlers.join(', ')}}`);
  return `el(${args.join(', ')})`;
}

/**
 * renderHandler emits the handler entries for an `on*` attribute.
 *
 * Handlers are attached to the node at creation, so an unchanged node keeps its
 * listener across a re-render and a recreated node gets a fresh one.
 */
function renderHandler(attr: Node, event: string, ctx: GenContext): string[] {
  const value = attr.value as Node | undefined;
  const fn = value?.type === 'ExpressionTag' ? (value.expression as Node) : value;
  if (!fn) return [];
  const body = handlerBody(fn, ctx);
  return [`${JSON.stringify(event)}: ${body}`];
}

/** renderBind emits the handler for bind:value, which writes state on input. */
function renderBind(attr: Node, event: string, ctx: GenContext): string[] {
  const target = (attr as { expression?: Node }).expression;
  if (!target) return [];
  // bind:value is the two-way binding: the committed text is written back to
  // the state cell, which is what makes the value update (docs/SVELTE.md).
  // The target keeps its raw name — writing to get(cell) would assign to a
  // temporary and lose the value.
  if (target.type === 'Identifier' && ctx.cells.has(String(target.name))) {
    return [`${JSON.stringify(event)}: function (ev) { set(${String(target.name)}, ev.text); }`];
  }
  return [`${JSON.stringify(event)}: function (ev) { (${expression(target, ctx)} = ev.text); }`];
}

/** renderDirective emits a handler entry for an explicit on:/bind: directive. */
function renderDirective(attr: Node, event: string, ctx: GenContext): string[] {
  const name = String(attr.name ?? '');
  const target = (attr as { expression?: Node }).expression;
  if (name === 'value' && attr.type === 'BindDirective' && target) {
    return renderBind(attr, event, ctx);
  }
  const fn = attr.expression as Node | undefined;
  if (!fn) return [];
  return [`${JSON.stringify(event)}: ${handlerBody(fn, ctx)}`];
}

/**
 * handlerBody renders the JS function for a handler.
 *
 * `onclick={fn}` where `fn` is a declared function becomes a direct call, so a
 * handler can keep its parameters; an inline arrow is emitted as written.
 */
function handlerBody(fn: Node, ctx: GenContext): string {
  if (fn.type === 'ArrowFunctionExpression' || fn.type === 'FunctionExpression') {
    return printFunction(fn, ctx);
  }
  const ref = expression(fn, ctx);
  // A bare reference is wrapped rather than passed directly, so the handler
  // always receives the event and never depends on `this`.
  return `function (ev) { if (typeof ${ref} === "function") { ${ref}(ev); } }`;
}

/** printFunction renders an arrow or function expression as a `function`. */
function printFunction(fn: Node, ctx: GenContext): string {
  const params = ((fn as { params?: Node[] }).params ?? []).map((p) => paramName(p as Node));
  const body = (fn as { body?: Node }).body as Node;
  if (!body) return `function (${params.join(', ')}) {}`;
  if (body.type === 'BlockStatement') {
    return `function (${params.join(', ')}) { ${renderBlock(body, ctx).join(' ')} }`;
  }
  return `function (${params.join(', ')}) { return ${expression(body, ctx)}; }`;
}

/** paramName renders a parameter as a plain name. */
function paramName(node: Node): string {
  if (node.type === 'Identifier') return String(node.name);
  if (node.type === 'AssignmentPattern') return paramName(node.left as Node);
  if (node.type === 'ObjectPattern') {
    return `{ ${((node as { properties?: Node[] }).properties ?? [])
      .map((p) => {
        const key = p.key as Node;
        const value = (p as { value?: Node }).value as Node;
        const k = key.type === 'Identifier' ? String(key.name) : String((key as { value?: unknown }).value ?? '');
        return `${k}: ${paramName(value)}`;
      })
      .join(', ')} }`;
  }
  return '__arg';
}

// --- expressions --------------------------------------------------------

/** expression renders an expression node as JavaScript source. */
export function expression(node: Node, ctx: GenContext): string {
  switch (node.type) {
    case 'Identifier': {
      const name = String(node.name);
      // A $state binding holds a cell, so every read unwraps it; a $derived
      // binding is a getter, so every read calls it. Without this a template
      // would interpolate "[object Object]".
      if (ctx.cells.has(name)) return `get(${name})`;
      if (ctx.derived.has(name)) return `${name}()`;
      return name;
    }

    case 'Literal': {
      const v = (node as { value?: unknown }).value;
      if (typeof v === 'string') return JSON.stringify(v);
      if (v === null) return 'null';
      if (typeof v === 'bigint') return `${String(v)}n`;
      return String(v);
    }

    case 'MemberExpression': {
      const object = expression((node as { object?: Node }).object as Node, ctx);
      const prop = node.property as Node;
      if (!node.computed && prop?.type === 'Identifier') {
        return `${object}.${String(prop.name)}`;
      }
      return `${object}[${expression(prop, ctx)}]`;
    }

    case 'CallExpression': {
      const callee = node.callee as Node;
      // A rune call in a value position cannot be compiled: a rune is a
      // declaration, not an expression, so it is left to the printer, which
      // produces an explicit error rather than a wrong value.
      const args = renderArgumentList(node, ctx);
      return `${expression(callee, ctx)}(${args})`;
    }

    case 'ArrowFunctionExpression':
    case 'FunctionExpression':
      return printFunction(node, ctx);

    case 'BinaryExpression': {
      const op = String((node as { operator?: unknown }).operator ?? '+');
      return `${expression((node as { left?: Node }).left as Node, ctx)} ${op} ${expression((node as { right?: Node }).right as Node, ctx)}`;
    }

    case 'LogicalExpression': {
      const op = String((node as { operator?: unknown }).operator ?? '&&');
      return `${expression((node as { left?: Node }).left as Node, ctx)} ${op} ${expression((node as { right?: Node }).right as Node, ctx)}`;
    }

    case 'UnaryExpression': {
      const op = String((node as { operator?: unknown }).operator ?? '');
      const arg = expression((node as { argument?: Node }).argument as Node, ctx);
      return op === '!' ? `!${arg}` : `${op}${arg}`;
    }

    case 'AssignmentExpression': {
      const op = String((node as { operator?: unknown }).operator ?? '=');
      const leftNode = (node as { left?: Node }).left as Node;
      const right = expression((node as { right?: Node }).right as Node, ctx);

      // A write to a state cell is what schedules the next flush; routing it
      // through set() is what makes `$state` reactive without a dependency
      // graph. The left side keeps the raw name — assigning to get(count)
      // would write a temporary.
      if (leftNode.type === 'Identifier') {
        const name = String(leftNode.name);
        if (ctx.cells.has(name)) {
          if (op === '=') return `set(${name}, ${right})`;
          // A compound assignment reads the current value and stores the
          // result once — mutating it in place as well would double-apply.
          const binary = op.replace('=', '');
          return `set(${name}, (${name}.value ${binary} ${right}))`;
        }
      }
      return `(${expression(leftNode, ctx)} ${op} ${right})`;
    }

    case 'UpdateExpression': {
      const op = String((node as { operator?: unknown }).operator ?? '++');
      const argNode = (node as { argument?: Node }).argument as Node;
      if (argNode.type === 'Identifier') {
        const name = String(argNode.name);
        if (ctx.cells.has(name)) {
          return op === '++'
            ? `set(${name}, ${name}.value + 1)`
            : `set(${name}, ${name}.value - 1)`;
        }
      }
      return `${expression(argNode, ctx)}${op}`;
    }

    case 'ConditionalExpression':
      return `(${expression((node as { test?: Node }).test as Node, ctx)} ? ${expression((node as { consequent?: Node }).consequent as Node, ctx)} : ${expression((node as { alternate?: Node }).alternate as Node, ctx)})`;

    case 'ArrayExpression':
      return `[${(((node as { elements?: Node[] }).elements ?? []) as Node[]).map((e) => expression(e, ctx)).join(', ')}]`;

    case 'ObjectExpression':
      return `{ ${(((node as { properties?: Node[] }).properties ?? []) as Node[])
        .filter((p) => p.type === 'Property')
        .map((p) => {
          const key = p.key as Node;
          const value = (p as { value?: Node }).value as Node;
          const k =
            key.type === 'Identifier'
              ? JSON.stringify(String(key.name))
              : expression(key, ctx);
          const v =
            value.type === 'AssignmentPattern'
              ? expression((value as { right?: Node }).right as Node, ctx)
              : expression(value, ctx);
          return `${k}: ${v}`;
        })
        .join(', ')} }`;

    case 'SequenceExpression':
      return `(${(((node as { expressions?: Node[] }).expressions ?? []) as Node[]).map((e) => expression(e, ctx)).join(', ')})`;

    case 'ParenthesizedExpression':
      return expression((node as { expression?: Node }).expression as Node, ctx);

    case 'TemplateLiteral': {
      const quasis = ((node as { quasis?: Node[] }).quasis ?? []).map((q) =>
        String((q as { value?: { raw?: string } }).value?.raw ?? ''),
      );
      const exprs = ((node as { expressions?: Node[] }).expressions ?? []).map((e) => expression(e, ctx));
      let out = JSON.stringify(quasis[0] ?? '');
      for (let i = 0; i < exprs.length; i++) {
        out += ` + ${exprs[i]} + ${JSON.stringify(quasis[i + 1] ?? '')}`;
      }
      return out;
    }

    default:
      return `__unsupported_expression(${JSON.stringify(node.type)})`;
  }
}

/**
 * staticText returns a literal attribute value, or null when it is dynamic.
 *
 * A value is a *list* of chunks, so `class="a b"` arrives as one Text chunk and
 * `class="a{x}"` as two. Only a single non-empty Text/Literal chunk is static;
 * reading the value as one node is why class attributes came out empty.
 */
function staticText(node: unknown): string | null {
  if (node === undefined || node === null) return null;
  if (Array.isArray(node)) {
    if (node.length === 0) return '';
    if (node.length !== 1) return null;
    return staticText(node[0]);
  }
  const n = node as Node;
  if (n.type === 'Text') return String(n.data ?? '');
  if (n.type === 'Literal') {
    const v = (n as { value?: unknown }).value;
    if (typeof v === 'string' || typeof v === 'number') return String(v);
  }
  return null;
}

/** valueChunks normalizes an attribute value into its chunk list. */
function valueChunks(value: unknown): Node[] {
  if (Array.isArray(value)) return value as Node[];
  if (value === undefined || value === null || (value as unknown) === true) return [];
  return [value as Node];
}

/**
 * dynamicValue renders a non-static attribute value as an expression, or
 * returns null when the value is static (handled by staticText).
 */
function dynamicValue(value: unknown, ctx: GenContext): string | null {
  const chunks = valueChunks(value);
  if (chunks.length === 0) return null;
  if (chunks.length === 1 && chunks[0]?.type === 'Text') return null;
  const parts = chunks.map((chunk) => {
    if (chunk?.type === 'Text') return JSON.stringify(String(chunk.data ?? ''));
    // An ExpressionTag chunk contributes its value; get() unwraps a state cell
    // so `class={cell}` reads the same as `class={cell.value}`.
    const inner = ((chunk as { expression?: Node }).expression as Node | undefined);
    if (inner?.type === 'MemberExpression' && !inner.computed) {
      const prop = inner.property as Node;
      if (prop?.type === 'Identifier' && String(prop.name) === 'value') {
        return `get(${expression(inner.object as Node, ctx)})`;
      }
    }
    return `String(${inner ? expression(inner, ctx) : 'undefined'})`;
  });
  return parts.join(' + ');
}

/** children returns a node's child nodes across the shapes Svelte uses. */
function children(node: Node): Node[] {
  const frag = (node as { fragment?: Node }).fragment;
  if (frag) return children(frag);
  const list =
    (node as { nodes?: Node[] }).nodes ?? (node as { children?: Node[] }).children;
  return Array.isArray(list) ? list : [];
}

/** printGeneric renders a node the subset does not model, as an explicit call. */
function printGeneric(node: Node): string {
  return `__unsupported_statement(${JSON.stringify(node.type)})`;
}

/** ELEMENTS is re-exported so the walk can share one source of truth. */
export { ELEMENTS };
