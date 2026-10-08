/**
 * Finding codes emitted by the adapter.
 *
 * A finding is the adapter's explicit refusal to compile a construct it cannot
 * represent in the GoWEZ UI subset (G-UPG-04, G-IFACE-04). Every code is
 * published in the generated gap register (docs/SVELTE.md), so adding a code is
 * a contract change and must be documented there.
 *
 * The `category` groups findings for the report mode summary; the `remedy`
 * says what an application author is expected to do.
 */
export type Category = 'svelte' | 'dom' | 'css' | 'element';

export interface FindingCodeSpec {
  category: Category;
  /** One-line explanation, published in the gap register. */
  remedy: string;
}

/**
 * CODES is the complete finding catalog. Keeping it closed means an unknown
 * code is a bug in the adapter rather than a silently new behavior.
 */
export const CODES = {
  // --- Svelte constructs outside the subset -------------------------------
  'SVELTE-TRANSITION': {
    category: 'svelte',
    remedy: 'Remove the transition/animation directive; the subset has no animation.',
  },
  'SVELTE-HEAD': {
    category: 'svelte',
    remedy: 'Remove <svelte:head>; the runtime has no document head.',
  },
  'SVELTE-WINDOW': {
    category: 'svelte',
    remedy: 'Remove svelte:window/svelte:document/svelte:body; attach the handler on the element instead.',
  },
  'SVELTE-SLOT': {
    category: 'svelte',
    remedy: 'Replace slots/snippets with explicit props and callback props.',
  },
  'SVELTE-SPREAD': {
    category: 'svelte',
    remedy: 'Replace {...spread} with explicit attributes.',
  },
  'SVELTE-AWAIT': {
    category: 'svelte',
    remedy: 'Remove {#await}; there is no promise in the runtime.',
  },
  'SVELTE-RAW-HTML': {
    category: 'svelte',
    remedy: 'Remove {@html}; the runtime never parses HTML strings.',
  },
  'SVELTE-INPUT': {
    category: 'svelte',
    remedy: 'Move the <script> block out; only instance/module scripts are compiled.',
  },
  'SVELTE-SELF-CLOSING-DIVID': {
    category: 'svelte',
    remedy: 'Close the non-void element explicitly (<div></div>).',
  },
  'SVELTE-UNSUPPORTED-NODE': {
    category: 'svelte',
    remedy: 'The node type is outside the compiled subset.',
  },
  'SVELTE-UNSUPPORTED-EXPRESSION': {
    category: 'svelte',
    remedy: 'The expression is outside the supported JavaScript subset.',
  },
  'SVELTE-IMPORT': {
    category: 'svelte',
    remedy: 'Import only relative .svelte modules, or the lifecycle helpers from "svelte".',
  },
  'SVELTE-RUNE': {
    category: 'svelte',
    remedy: 'Only $state, $derived, $effect and $props are supported; see docs/SVELTE.md.',
  },

  // --- Browser/host APIs the sandbox does not provide ----------------------
  'DOM-GLOBAL': {
    category: 'dom',
    remedy: 'Remove the browser global; the runtime exposes only gowez.* (docs/SCRIPT.md).',
  },
  'DOM-API': {
    category: 'dom',
    remedy: 'Remove the browser API call; there is no DOM in the runtime.',
  },
  'DOM-FUNCTION': {
    category: 'dom',
    remedy: 'Remove the browser function reference (timers, location, console, fetch).',
  },

  // --- Elements outside the subset -----------------------------------------
  'ELEMENT-REJECTED': {
    category: 'element',
    remedy: 'Use an in-subset element; the subset is block/flex, no inline flow or tables.',
  },
  'ELEMENT-UNSUPPORTED': {
    category: 'element',
    remedy: 'The element is not part of the M5 subset.',
  },

  // --- CSS outside the subset ----------------------------------------------
  'CSS-PROPERTY': {
    category: 'css',
    remedy: 'Remove the property; the CSS subset is documented in docs/CSS-SUBSET.md.',
  },
  'CSS-AT-RULE': {
    category: 'css',
    remedy: 'Remove the at-rule; the subset has no @media/@supports.',
  },
  'CSS-SELECTOR': {
    category: 'css',
    remedy: 'Rewrite the selector; the subset matches type, class, id and :hover/:active/:focus only.',
  },
  'CSS-SCOPE-COLLISION': {
    category: 'css',
    remedy: 'Two modules derive the same style scope; rename one so scoping stays unique.',
  },
  'CSS-UNKNOWN': {
    category: 'css',
    remedy: 'The declaration could not be validated against the subset.',
  },
} as const satisfies Record<string, FindingCodeSpec>;

export type Code = keyof typeof CODES;

export interface Finding {
  /** Stable finding code; see CODES. */
  code: Code;
  category: Category;
  /** Source file, relative to the project root. */
  file: string;
  /** 1-based line of the offending construct. */
  line: number;
  /** 0-based column, matching the compiler AST. */
  column: number;
  /** Human-readable detail, including the offending name where useful. */
  message: string;
}

/** CompileError is thrown by strict mode: the first finding stops the build. */
export class CompileError extends Error {
  constructor(public readonly finding: Finding) {
    super(
      `${finding.file}:${finding.line}:${finding.column}: ` +
        `${finding.code} — ${finding.message}`,
    );
    this.name = 'CompileError';
  }
}

/** Report collects findings instead of throwing. */
export class Report {
  private readonly findings: Finding[] = [];

  add(finding: Finding): void {
    this.findings.push(finding);
  }

  get all(): readonly Finding[] {
    return this.findings;
  }

  get length(): number {
    return this.findings.length;
  }

  /** byCode counts findings per code, sorted by descending count. */
  byCode(): Array<{ code: Code; count: number; category: Category; remedy: string }> {
    const counts = new Map<Code, number>();
    for (const f of this.findings) counts.set(f.code, (counts.get(f.code) ?? 0) + 1);
    return [...counts.entries()]
      .map(([code, count]) => ({
        code,
        count,
        category: CODES[code].category,
        remedy: CODES[code].remedy,
      }))
      .sort((a, b) => b.count - a.count || a.code.localeCompare(b.code));
  }
}
