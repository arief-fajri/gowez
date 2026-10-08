/**
 * Gap report over the real dashboard sample.
 *
 * The full `examples/dashboard` app cannot render in M5 — its CSS grid,
 * scrolling, tables, selects, routing and transitions are all outside the
 * subset (docs/PLAN-M5 §0.2). Rather than leave that as prose, the adapter
 * *reports* it: every unsupported construct becomes a finding with a file and
 * a position, and this snapshot is the evidence that the gap register is
 * accurate.
 *
 * The snapshot is intentionally tracked. When the sample evolves, the diff is
 * reviewed on purpose (conscious update, not a silent regression).
 */
import { existsSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { cssGapProperties } from '../src/css.js';
import { report, type Finding } from '../src/index.js';

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, '..', '..', '..');
const dashboard = join(repoRoot, 'examples', 'dashboard');

/** The sample exists only after `npm install` populated the workspace. */
const sampleAvailable = existsSync(join(dashboard, 'src', 'App.svelte'));

/**
 * generateReport runs report mode over the dashboard sample.
 *
 * The library entry is used rather than the CLI: the CLI is a plain .ts entry
 * that needs a loader, and the point of this test is the finding set, not the
 * command-line plumbing (which the CLI test covers separately).
 */
function generateReport(): Finding[] {
  return [...report(dashboard, 'src/App.svelte').findings];
}

describe.skipIf(!sampleAvailable)('dashboard gap report', () => {
  it('produces findings for the constructs the sample uses outside the subset', () => {
    const findings = generateReport();
    expect(findings.length).toBeGreaterThan(0);

    const codes = new Set(findings.map((f) => f.code));
    // The four gap groups the plan named up front, verified against the sample
    // rather than trusted.
    expect(codes.has('ELEMENT-REJECTED')).toBe(true); // table, select, option, strong
    expect(codes.has('CSS-PROPERTY')).toBe(true); // grid, radius, shadow, var()
    expect(codes.has('DOM-GLOBAL')).toBe(true); // location, window, setTimeout, document
    expect(codes.has('SVELTE-TRANSITION')).toBe(true); // in:fade, transition:*
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
    // Determinism matters for a tracked snapshot: a build that reorders
    // findings would produce a diff nobody can review.
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
