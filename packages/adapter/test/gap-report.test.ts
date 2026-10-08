/**
 * Gap register budget over the real dashboard sample.
 *
 * `examples/dashboard` is the **target application** (DRR-008): the whole point
 * of the register is that it eventually reaches zero and all six pages render.
 * So this test is not "does the sample produce findings" — it is a *budget*:
 * a milestone is done when the number falls to a stated ceiling, and a
 * regression above the ceiling fails here instead of being noticed by eye.
 *
 * Two rules make the budget falsifiable rather than decorative:
 *
 * 1. **The ceiling only ever moves down.** `budget` below records where the
 *    program is; changing a number upward is a deliberate act that belongs in
 *    the commit message, not a side effect.
 * 2. **A *silent* drop is a failure, not a win.** The register counts
 *    *rejections*, so it cannot see output that is wrong rather than missing —
 *    component style scoping was such a defect: eight rules overrode each other
 *    while the register reported nothing. A budget that falls without the named
 *    capability landing is exactly that failure mode. `expectedCapability`
 *    therefore names what each ceiling is buying, and the per-code assertions
 *    pin that the construct is *absent* rather than merely fewer.
 */
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { cssGapProperties } from '../src/css.js';
import { report, type Code, type Finding } from '../src/index.js';

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, '..', '..', '..');
const dashboard = join(repoRoot, 'examples', 'dashboard');

/** The sample exists only after `npm install` populated the workspace. */
const sampleAvailable = existsSync(join(dashboard, 'src', 'App.svelte'));

/**
 * budget is the gap register's current state, and where each step of the program
 * must land. See DRR-008 §Correction and the M6 design note.
 *
 * `total` is the whole register; `cssProperty` is the CSS bucket the M6–M8
 * milestones are measured against. Both are ceilings, not targets to approach
 * from above on a good day: they are the number a milestone must not exceed.
 */
const budget = {
  /** Before the register was fixed: 259. After M6a: 197. After strict-reporting and explicit global-stylesheet routing: 276. */
  total: 276,
  cssProperty: 198,
  /** What the current ceiling represents — a drop without this landing is a bug. */
  expectedCapability:
    'M6a (var(), custom properties, color-mix(), list-style, outline) plus Fase 1 strict reporting (scoped KeyBlock component resolution, compile-time fallback detection, explicit global-stylesheet route)',
} as const;

/** generateReport runs report mode over the dashboard sample. */
function generateReport(): Finding[] {
  return [
    ...report(dashboard, 'src/App.svelte', {
      globalStyles: [
        {
          file: 'src/styles/ui.css',
          source: readFileSync(join(dashboard, 'src', 'styles', 'ui.css'), 'utf8'),
        },
      ],
    }).findings,
  ];
}

/** byCode counts findings per code. */
function byCode(findings: readonly Finding[]): Map<Code, number> {
  const counts = new Map<Code, number>();
  for (const f of findings) counts.set(f.code, (counts.get(f.code) ?? 0) + 1);
  return counts;
}

/** countOf is byCode with a default, so an absent code reads as 0. */
function countOf(findings: readonly Finding[], code: Code): number {
  return byCode(findings).get(code) ?? 0;
}

describe.skipIf(!sampleAvailable)('dashboard gap register budget', () => {
  it('stays within the total ceiling', () => {
    const findings = generateReport();
    // Sanity first: a register that suddenly reports nothing because the scan
    // broke is not a pass, it is a silent instrumentation failure (D).
    expect(findings.length).toBeGreaterThan(0);
    expect(findings.length).toBeLessThanOrEqual(budget.total);
  });

  it('stays within the CSS ceiling', () => {
    expect(countOf(generateReport(), 'CSS-PROPERTY')).toBeLessThanOrEqual(budget.cssProperty);
  });

  it('reports no var(), color-mix() or custom-property rejection', () => {
    // M6a is the step the current ceiling pays for. Asserting the *constructs*
    // rather than only the count means a drop caused by something else — a
    // scan that stopped reading a file, say — cannot masquerade as progress.
    const offenders = generateReport().filter((f) =>
      /var\(|color-mix\(/.test(f.message),
    );
    expect(offenders.map((f) => `${f.file}: ${f.message}`)).toEqual([]);
  });

  it('reports no list-style or outline rejection', () => {
    // Also M6a. `list-style`/`outline` accept only `none`; the sample uses
    // exactly that, so any finding here means the value stopped being honoured.
    const offenders = generateReport().filter(
      (f) => /^list-style\b|^outline\b/.test(f.message),
    );
    expect(offenders.map((f) => `${f.file}: ${f.message}`)).toEqual([]);
  });

  it('keeps the compiler-generated findings visible', () => {
    // Genuine compile-time detections are part of the ceiling, not accidents.
    // Components in {#key} must resolve, while genuinely unmodelled JS must
    // still be reported.
    const findings = generateReport();
    expect(
      findings.filter((f) => f.message.includes('could not be resolved')).map((f) => f.message),
    ).toEqual([]);
    expect(countOf(findings, 'SVELTE-UNSUPPORTED-EXPRESSION')).toBe(5);
  });

  it('keeps the still-open buckets visible', () => {
    // The opposite direction: a budget that passes because a whole category
    // silently vanished would hide a regression behind a number. These are the
    // buckets later milestones own, and they must still be *reported* until
    // the milestone that closes them lands.
    const findings = generateReport();
    const open: Array<[Code, string]> = [
      ['ELEMENT-REJECTED', 'M10: table / select / option / strong'],
      ['DOM-GLOBAL', 'M9: timers, routing, document, window'],
      ['SVELTE-HEAD', 'M9: document head'],
      ['SVELTE-TRANSITION', 'M10: in:fade'],
      ['CSS-AT-RULE', 'M8: @media'],
      ['CSS-SELECTOR', 'M8/M9: :last-child, ::placeholder'],
    ];
    for (const [code, why] of open) {
      expect(countOf(findings, code), `${code} should still be reported (${why})`)
        .toBeGreaterThan(0);
    }
  });

  it('attributes every finding to a real sample file', () => {
    for (const finding of generateReport()) {
      // Reported relative to the project root, so a build error points at a
      // path the application author recognizes.
      expect(finding.file).toMatch(/^src\//);
      expect(existsSync(join(dashboard, finding.file))).toBe(true);
    }
  });

  it('exits 0 — report mode scans rather than fails', () => {
    expect(() => generateReport()).not.toThrow();
  });

  it('is stable across runs', () => {
    // Determinism matters for a budget: a scan that reorders findings would
    // produce a diff nobody can review.
    expect(JSON.stringify(generateReport())).toBe(JSON.stringify(generateReport()));
  });
});

describe('CSS gap catalog', () => {
  it('publishes the out-of-subset properties the docs reference', () => {
    const gaps = cssGapProperties();
    for (const property of [
      'grid-template-columns',
      'overflow-y',
      'border-radius',
      'box-shadow',
      'transition',
      'position',
    ]) {
      expect(gaps).toContain(property);
    }
  });

  it('is sorted, so the generated docs section is stable', () => {
    const gaps = cssGapProperties();
    expect(gaps).toEqual([...gaps].sort());
  });
});