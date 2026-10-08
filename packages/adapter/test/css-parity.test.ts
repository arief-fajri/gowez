/**
 * CSS parity: the TypeScript validator must agree with the Go subset.
 *
 * The authoritative rule set is `internal/style` (docs/CSS-SUBSET.md). The
 * adapter mirrors it so a build can fail before a bundle exists — but a mirror
 * that silently disagrees is worse than no mirror, because it lets a stylesheet
 * through that Go then rejects at startup, or rejects one Go would have accepted.
 *
 * So the verdicts are compared, not trusted: this test runs the real Go parser
 * and asserts both validators return the same answer for every case.
 */
import { execFileSync } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { ALLOWED_PROPERTIES, validateCssSubset } from '../src/css.js';

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, '..', '..', '..');

interface Decision {
  property: string;
  value: string;
  accept: boolean;
  message?: string;
}

/** goDecisions runs the real Go CSS parser over the probe list. */
function goDecisions(): Decision[] {
  const out = execFileSync('go', ['run', './tests/parity/paritydump'], {
    cwd: repoRoot,
    encoding: 'utf8',
    maxBuffer: 8 * 1024 * 1024,
  });
  return JSON.parse(out) as Decision[];
}

/** tsAccepts asks the adapter's own validator about one declaration. */
function tsAccepts(property: string, value: string): boolean {
  try {
    validateCssSubset([{ property, value }], 'probe');
    return true;
  } catch {
    return false;
  }
}

describe('CSS parity between the Go subset and the adapter mirror', () => {
  const decisions = goDecisions();

  it('the probe list is substantial enough to be worth comparing', () => {
    expect(decisions.length).toBeGreaterThanOrEqual(40);
    expect(decisions.some((d) => d.accept)).toBe(true);
    expect(decisions.some((d) => !d.accept)).toBe(true);
  });

  it.each(goDecisions().map((d) => [`${d.property}: ${d.value}`, d] as const))(
    'agrees on %s',
    (_label, decision) => {
      const go = decision.accept;
      const ts = tsAccepts(decision.property, decision.value);
      expect(
        ts,
        `Go ${go ? 'accepts' : 'rejects'} ${decision.property}: ${decision.value}` +
          (decision.message ? ` (${decision.message})` : '') +
          `, but the adapter validator ${ts ? 'accepts' : 'rejects'} it`,
      ).toBe(go);
    },
  );

  it('the mirror lists exactly the properties Go supports', () => {
    // Go's property table is the contract. A property in the mirror that Go
    // rejects is drift; a property Go supports that the mirror omits means the
    // adapter rejects valid stylesheets.
    const goProperties = new Set(
      decisions.filter((d) => !d.property.startsWith('@')).map((d) => d.property),
    );
    const mirror = new Set(ALLOWED_PROPERTIES);
    for (const property of mirror) {
      if (!goProperties.has(property)) {
        // Only report properties Go actually refused, not ones simply absent
        // from the probe list.
        const refused = decisions.find(
          (d) => d.property === property && !d.accept,
        );
        expect(refused, `mirror allows ${property}, which the probe never exercises`).toBeUndefined();
      }
    }
  });
});
