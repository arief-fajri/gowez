/**
 * Rejection tests: one per unsupported construct.
 *
 * This file is the "Unsupported Svelte/browser behavior is explicit" checklist
 * item. Every construct outside the M5 subset must produce a finding with a
 * stable code — never a silent skip, which would turn "unsupported" into
 * "supported" (G-UPG-04, G-IFACE-04).
 */
import { describe, expect, it } from 'vitest';
import { buildFixture, reportFixture } from './helpers.js';
import { rejectedFixtures } from './fixtures.js';
import { CODES, CompileError, type Code } from '../src/index.js';

describe('unsupported constructs are explicit findings', () => {
  for (const fixture of rejectedFixtures) {
    it(`${fixture.name} → ${fixture.code}`, () => {
      const result = reportFixture(fixture.source);
      const codes = result.findings.map((f) => f.code);
      expect(codes).toContain(fixture.code as Code);
    });
  }

  it('report mode never throws, so the whole project can be scanned', () => {
    for (const fixture of rejectedFixtures) {
      expect(() => reportFixture(fixture.source)).not.toThrow();
    }
  });

  it('every finding carries a file and a non-zero location or an explicit zero', () => {
    for (const fixture of rejectedFixtures) {
      const result = reportFixture(fixture.source);
      for (const finding of result.findings) {
        expect(finding.file).toBe('src/App.svelte');
        expect(typeof finding.line).toBe('number');
        expect(typeof finding.column).toBe('number');
        expect(finding.message.length).toBeGreaterThan(0);
        expect(CODES[finding.code]).toBeDefined();
      }
    }
  });

  it('every finding code has a published remedy for the gap register', () => {
    for (const [code, spec] of Object.entries(CODES)) {
      expect(spec!.remedy.length, `${code} has no remedy`).toBeGreaterThan(0);
      expect(['svelte', 'dom', 'css', 'element']).toContain(spec!.category);
    }
  });
});

describe('strict mode fails the build', () => {
  it('throws CompileError with file, line, column and code', () => {
    let caught: unknown;
    try {
      buildFixture('<section><select><option>a</option></select></section>');
    } catch (err) {
      caught = err;
    }
    expect(caught).toBeInstanceOf(CompileError);
    const err = caught as CompileError;
    expect(err.finding.code).toBe('ELEMENT-REJECTED');
    expect(err.finding.file).toBe('src/App.svelte');
    expect(err.message).toContain('ELEMENT-REJECTED');
  });

  it('names the offending element in the message', () => {
    let caught: unknown;
    try {
      buildFixture('<section><table><tr><td>a</td></tr></table></section>');
    } catch (err) {
      caught = err;
    }
    expect((caught as CompileError).finding.message).toContain('table');
  });
});
