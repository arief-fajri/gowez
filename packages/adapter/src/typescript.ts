/**
 * TypeScript handling for the AST walk (decision D-2, DRR-007).
 *
 * `svelte/compiler.parse()` does **not** strip TypeScript: the AST retains
 * `TSTypeAnnotation`, `TSInterfaceDeclaration` and friends. The helper that
 * `compile()` uses internally (`remove_typescript_nodes`) is not exported and
 * deep-importing it fails with ERR_PACKAGE_PATH_NOT_EXPORTED, so the adapter
 * owns its stripping. This costs nothing: `compile()`'s job is generating
 * JavaScript, which Strategy B does not use.
 *
 * The contract is closed on purpose. A TS shape that is neither unwrapped nor
 * dropped is a **build failure**, never a residual annotation that reaches
 * codegen (G-UPG-04).
 */

/** Shapes whose runtime value is the inner expression. */
const TS_UNWRAP = new Set([
  'TSTypeAnnotation',
  'TSAsExpression',
  'TSSatisfiesExpression',
  'TSNonNullExpression',
  'TSTypeAssertion',
  'TSInstantiationExpression',
  'TSParameterProperty',
]);

/** Shapes that carry no runtime value at all. */
const TS_DROP = new Set([
  'TSInterfaceDeclaration',
  'TSTypeAliasDeclaration',
  'TSEnumDeclaration',
  'TSDeclareFunction',
  'TSDeclareMethod',
  'TSModuleDeclaration',
  'TSImportEqualsDeclaration',
  'TSExportAssignment',
  'TSNamespaceExportDeclaration',
]);

export const TS_SHAPES = {
  unwrap: [...TS_UNWRAP].sort(),
  drop: [...TS_DROP].sort(),
} as const;

/** UnwrapKind is the outcome of classifying a node as TypeScript. */
export type UnwrapKind =
  | { kind: 'passthrough' } // not a TS node: return as-is
  | { kind: 'expression'; node: Node | null } // unwrap to the inner expression
  | { kind: 'drop' } // type-only: emit nothing
  | { kind: 'unknown'; shape: string }; // unrecognized TS node: fail the build

/**
 * classify decides what to do with a node that may be a TypeScript shape.
 * Returns 'unknown' for a `TS*` node outside the closed set, so the caller can
 * fail the build explicitly instead of emitting code with a residual type.
 */
export function classify(node: unknown): UnwrapKind {
  if (!isNode(node)) return { kind: 'passthrough' };
  const type = String(node.type);
  if (TS_UNWRAP.has(type)) {
    return { kind: 'expression', node: unwrapExpression(node) };
  }
  if (TS_DROP.has(type)) return { kind: 'drop' };
  if (type.startsWith('TS')) return { kind: 'unknown', shape: type };
  return { kind: 'passthrough' };
}

/**
 * unwrapExpression strips one layer of type wrapper and returns the expression
 * underneath. A wrapper without an `expression` (e.g. a type annotation on a
 * declarator id, whose type is not a runtime value) yields null.
 */
function unwrapExpression(node: Node): Node | null {
  const inner = (node as unknown as { expression?: unknown }).expression;
  return isNode(inner) ? inner : null;
}

/**
 * stripTypes deep-strips TypeScript from a subtree, calling visit on every
 * surviving node in the same order the walker would.
 *
 * Returning `null` removes the node from its parent. Dropped nodes are removed
 * rather than replaced with a placeholder so no empty statement survives into
 * codegen.
 */
export function stripTypes<T extends Node>(
  root: T,
  visit: (node: Node) => void,
): T | null {
  return walkTypes(root, visit) as T | null;
}

function walkTypes(node: unknown, visit: (node: Node) => void): Node | null {
  const verdict = classify(node);
  switch (verdict.kind) {
    case 'drop':
      return null;
    case 'unknown':
      // Left for the caller to surface as a CompileError; kept in the tree so
      // the walk reaches it and reports the exact position.
      break;
    case 'expression':
      return verdict.node === null ? null : walkTypes(verdict.node, visit);
    case 'passthrough':
      break;
  }

  const n = node as Node;
  visit(n);
  for (const key of Object.keys(n)) {
    if (key === 'loc' || key === 'start' || key === 'end' || key === 'parent') continue;
    const child = (n as unknown as Record<string, unknown>)[key];
    if (Array.isArray(child)) {
      const kept = child
        .map((c) => walkTypes(c, visit))
        .filter((c): c is Node => c !== null);
      (n as unknown as Record<string, unknown>)[key] = kept;
    } else if (isNode(child)) {
      const kept = walkTypes(child, visit);
      (n as unknown as Record<string, unknown>)[key] = kept;
    }
  }
  return n;
}

function isNode(v: unknown): v is Node {
  return typeof v === 'object' && v !== null && typeof (v as { type?: unknown }).type === 'string';
}

/** Svelte's AST node, narrowed to what the adapter touches. */
export interface Node {
  type: string;
  start?: number;
  end?: number;
  loc?: { start: { line: number; column: number } };
  [key: string]: unknown;
}
