/**
 * CSS subset validation on the TypeScript side.
 *
 * The authoritative rule set lives in Go (`internal/style`, spec in
 * docs/CSS-SUBSET.md). This module mirrors it so the adapter can fail a build
 * before a bundle exists — and `css-parity.test.ts` compares the two
 * validator-for-validator, because a mirror that silently disagrees is worse
 * than no mirror: it either lets a stylesheet through that Go then rejects at
 * startup, or refuses one Go would have accepted.
 *
 * Every rule below mirrors internal/style/value.go. When that table changes,
 * this file changes in the same commit and the parity test proves it.
 */

/** Properties the subset accepts — mirrors internal/style's `properties` map. */
export const ALLOWED_PROPERTIES = new Set([
  'display',
  'width',
  'height',
  'margin',
  'margin-top',
  'margin-right',
  'margin-bottom',
  'margin-left',
  'padding',
  'padding-top',
  'padding-right',
  'padding-bottom',
  'padding-left',
  'border-width',
  'border-top-width',
  'border-right-width',
  'border-bottom-width',
  'border-left-width',
  'border-color',
  'border-top-color',
  'border-right-color',
  'border-bottom-color',
  'border-left-color',
  'background-color',
  'list-style',
  'outline',
  'color',
  'font-size',
  'flex-direction',
  'justify-content',
  'align-items',
  'gap',
  'flex-grow',
  'flex-shrink',
]);

/** Value grammars, keyed by property — mirrors internal/style's valueKind. */
type Grammar = (value: string) => string | null;

/** px accepts "0" or a non-negative "<n>px" — parsePxValue. */
const px: Grammar = (value) => {
  if (value === '0') return null;
  if (!value.endsWith('px')) return `want a px length (e.g. 8px), got ${value}`;
  const n = Number(value.slice(0, -2));
  if (Number.isNaN(n)) return `invalid length ${value}`;
  if (n < 0) return `negative lengths are not supported`;
  return null;
};

/** number accepts a non-negative unitless number — parseNumberValue. */
const number: Grammar = (value) => {
  const n = Number(value);
  if (Number.isNaN(n)) return `want a number, got ${value}`;
  if (n < 0) return `negative numbers are not supported`;
  return null;
};

/** width accepts auto, px and percentages — parseWidthValue. */
const width: Grammar = (value) => {
  if (value === 'auto') return null;
  if (value.endsWith('%')) {
    const n = Number(value.slice(0, -1));
    if (Number.isNaN(n) || n < 0) return `invalid percentage ${value}`;
    return null;
  }
  return px(value);
};

/** height accepts auto and px; a percentage is refused, not treated as auto. */
const height: Grammar = (value) => {
  if (value.endsWith('%')) return `percentage heights are not supported`;
  return width(value);
};

/** box accepts 1–4 px lengths — kindBoxShorthand. */
const box: Grammar = (value) => {
  const parts = value.trim().split(/\s+/);
  if (parts.length < 1 || parts.length > 4) {
    return `want 1 to 4 lengths, got ${parts.length}`;
  }
  for (const part of parts) {
    const err = px(part);
    if (err) return err;
  }
  return null;
};

/** color accepts hex forms, three keywords, and color-mix(in srgb, …) —
 * parseColorValue. */
const color: Grammar = (value) => {
  const v = value.trim();
  if (v.startsWith('color-mix(')) return colorMix(v);
  if (v.startsWith('#')) {
    const hex = v.slice(1);
    if (![3, 4, 6, 8].includes(hex.length)) {
      return `want #RGB, #RRGGBB or #RRGGBBAA`;
    }
    if (!/^[0-9a-fA-F]+$/.test(hex)) return `invalid hex color`;
    return null;
  }
  if (['black', 'white', 'transparent'].includes(v)) return null;
  return `unsupported color (docs/CSS-SUBSET.md)`;
};

/**
 * colorMix mirrors parseColorMixValue's *verdict* — not its arithmetic, which
 * only Go performs. What matters here is agreeing on which inputs are legal:
 * `in srgb` only, exactly two colours, percentages in range and summing to 100
 * when both are given.
 *
 * A space other than `in srgb` is refused rather than quietly mixed in srgb
 * anyway, which would return a different colour from every browser.
 */
const colorMix: Grammar = (value) => {
  if (!value.endsWith(')')) return `unterminated color-mix()`;
  const inner = value.slice('color-mix('.length, -1).trim();
  const commas = splitTopLevelCommas(inner);
  if (commas.length < 1) return `want color-mix(in srgb, <color>, <color>)`;
  const space = commas[0]!.text;
  if (space.toLowerCase() !== 'in srgb') {
    return `only 'in srgb' is supported; "${space}" would mix in a different space and give a different colour`;
  }
  const clauses = commas.slice(1).map((c) => c.text);
  if (clauses.length !== 2) return `want exactly two colours, got ${clauses.length}`;
  let sum = 0;
  let seen = 0;
  for (const clause of clauses) {
    const fields = clause.split(/\s+/).filter(Boolean);
    if (fields.length === 0 || fields.length > 2) {
      return `want a colour and an optional percentage, got "${clause}"`;
    }
    const err = color(fields[0]!);
    if (err) return err;
    if (fields.length === 2) {
      const pct = fields[1]!;
      if (!pct.endsWith('%')) return `percentage must end in %, got "${pct}"`;
      const n = Number(pct.slice(0, -1));
      if (!Number.isFinite(n) || n < 0 || n > 100) {
        return `percentage ${pct} is outside 0%-100%`;
      }
      sum += n;
      seen++;
    }
  }
  if (seen === 2 && Math.abs(sum - 100) > 1e-9) {
    return `percentages must sum to 100%, got ${sum}%`;
  }
  return null;
};

/** splitTopLevelCommas splits on commas outside parentheses, keeping each
 * piece, so the space clause can be read off the front. */
function splitTopLevelCommas(s: string): Array<{ text: string }> {
  const out: Array<{ text: string }> = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < s.length; i++) {
    const ch = s[i];
    if (ch === '(') depth++;
    else if (ch === ')') depth = Math.max(0, depth - 1);
    else if (ch === ',' && depth === 0) {
      out.push({ text: s.slice(start, i).trim() });
      start = i + 1;
    }
  }
  out.push({ text: s.slice(start).trim() });
  return out;
}

/** splitTopLevelFields splits on whitespace outside parentheses, so
 * `color-mix(in srgb, #f00, #fff)` stays one field instead of becoming five. */
function splitTopLevelFields(s: string): string[] {
  const out: string[] = [];
  let depth = 0;
  let start = -1;
  for (let i = 0; i < s.length; i++) {
    const ch = s[i];
    if (ch === '(') depth++;
    else if (ch === ')') depth = Math.max(0, depth - 1);
    if (depth === 0 && ch !== undefined && /\s/.test(ch)) {
      if (start >= 0) {
        out.push(s.slice(start, i));
        start = -1;
      }
      continue;
    }
    if (start < 0) start = i;
  }
  if (start >= 0) out.push(s.slice(start));
  return out;
}

/** fontSize accepts a positive px length — parseFontSizeValue. */
const fontSize: Grammar = (value) => {
  const err = px(value);
  if (err) return err;
  if (Number(value === '0' ? 0 : value.slice(0, -2)) <= 0) return 'font-size must be positive';
  return null;
};

/** keywords builds a grammar that accepts only a fixed keyword set. */
function keywords(allowed: string[], note = ''): Grammar {
  return (value) =>
    allowed.includes(value)
      ? null
      : `unsupported value${note ? ` (${note})` : ''} (docs/CSS-SUBSET.md)`;
}

/**
 * noneOnly accepts exactly `none`.
 *
 * Used for `list-style` and `outline`. The subset honours `none` because the
 * outcome is already true — there is no list marker and no outline in the render
 * pipeline — not because the declaration is ignored. Every other value is
 * refused, because accepting `outline: 2px solid red` would claim an effect the
 * runtime does not produce.
 */
const noneOnly: Grammar = (value) =>
  value === 'none'
    ? null
    : 'only none is supported (the subset paints no list marker and no outline)';

/** colors1to4 mirrors kindColorSides: 1–4 colours, expanded per side. */
function colors1to4(what: string): Grammar {
  return (value) => {
    const parts = splitTopLevelFields(value.trim());
    if (parts.length < 1 || parts.length > 4) return `want 1-4 ${what}, got "${value}"`;
    for (const p of parts) {
      const err = color(p);
      if (err) return err;
    }
    return null;
  };
}

const GRAMMARS: Record<string, Grammar> = {
  display: keywords(['block', 'flex', 'none']),
  width,
  height,
  margin: box,
  'margin-top': px,
  'margin-right': px,
  'margin-bottom': px,
  'margin-left': px,
  padding: box,
  'padding-top': px,
  'padding-right': px,
  'padding-bottom': px,
  'padding-left': px,
  'border-width': box,
  'border-top-width': px,
  'border-right-width': px,
  'border-bottom-width': px,
  'border-left-width': px,
  'border-color': colors1to4('colors'),
  'border-top-color': color,
  'border-right-color': color,
  'border-bottom-color': color,
  'border-left-color': color,
  'background-color': color,
  'list-style': noneOnly,
  outline: noneOnly,
  color,
  'font-size': fontSize,
  'flex-direction': keywords(['row', 'column']),
  'justify-content': keywords(['start', 'center', 'end', 'space-between']),
  'align-items': keywords(['stretch', 'start', 'center', 'end']),
  gap: px,
  'flex-grow': number,
  'flex-shrink': number,
};

/**
 * KNOWN_GAPS is the published list of properties applications ask for that the
 * subset deliberately lacks. It is documentation, not validation: the property
 * table already rejects them. docs/SVELTE.md §Gap register is generated from it,
 * so a new entry is a documented contract change.
 */
export const KNOWN_GAPS = new Set([
  'border-radius',
  'box-shadow',
  'transition',
  'transform',
  'position',
  'top',
  'right',
  'bottom',
  'left',
  'z-index',
  'cursor',
  'overflow',
  'overflow-x',
  'overflow-y',
  'grid-template-columns',
  'grid-template-rows',
  'grid-column',
  'grid-row',
  'grid-area',
  'display: grid',
  'min-width',
  'max-width',
  'min-height',
  'max-height',
  'line-height',
  'font-family',
  'text-align',
  'opacity',
  'flex-wrap',
  'flex-basis',
  'align-self',
  'row-gap',
  'column-gap',
]);

/**
 * cssSubsetErrors returns one message per rejected declaration, in source
 * order, and an empty array when the whole rule is in the subset.
 *
 * The register needs *every* rejection, not the first. An earlier version threw
 * on the first bad declaration, so report mode recorded one finding per rule and
 * the register counted rules rather than gaps: `.app { display: grid;
 * grid-template-columns: 240px 1fr; height: 100dvh }` reported only
 * `display: grid`, hiding two more problems behind it. That made the number a
 * lower bound that could *stay flat* while a milestone closed a property — or
 * rise as earlier failures unmasked later ones — which is precisely the shape of
 * a gate that cannot fail honestly.
 */
export function cssSubsetErrors(
  declarations: ReadonlyArray<{ property: string; value: string }>,
  file = '',
): string[] {
  const where = file ? `${file}: ` : '';
  const errors: string[] = [];
  for (const { property, value } of declarations) {
    if (property.startsWith('@')) {
      errors.push(`${where}at-rule ${property} is not in the CSS subset`);
      continue;
    }
    if (value.includes('!')) {
      errors.push(`${where}${property}: !important is not supported`);
      continue;
    }
    // A custom property holds an untyped token stream; only the property that
    // consumes it interprets the value, so nothing is checked here.
    if (isCustomProperty(property)) {
      if (!balancedParens(value)) {
        errors.push(`${where}${property}: unbalanced parentheses in the custom property value`);
      }
      continue;
    }
    // Property existence is checked FIRST, before any var() deferral. A
    // deferral that ran first would accept `background: var(--x)` and
    // `grid-template-columns: var(--x)` — the property is unknown regardless of
    // what its value turns out to be, and Go rejects it at that point too.
    const grammar = GRAMMARS[property];
    if (!grammar) {
      const kind = KNOWN_GAPS.has(property) ? 'documented gap' : 'unsupported property';
      errors.push(`${where}${property} is a ${kind} (docs/CSS-SUBSET.md)`);
      continue;
    }
    // A value containing var() cannot be validated yet: substitution happens
    // per node at resolve time, after the cascade. Deferring does not weaken
    // the check — the Go resolve re-parses the substituted text — but this
    // mirror genuinely cannot do it, which is the one place the two
    // implementations differ in reach rather than in verdict.
    if (value.includes('var(')) {
      if (!balancedParens(value)) {
        errors.push(`${where}${property}: unterminated var( in "${value}"`);
      }
      continue;
    }
    const err = grammar(value.trim());
    if (err) {
      errors.push(`${where}${property}: "${value}": ${err}`);
    }
  }
  return errors;
}

/** isCustomProperty mirrors the Go isCustomProperty. */
function isCustomProperty(name: string): boolean {
  return name.startsWith('--') && name.length > 2;
}

/** balancedParens mirrors the Go balancedParens: every ( has a matching ). */
function balancedParens(value: string): boolean {
  let depth = 0;
  for (const ch of value) {
    if (ch === '(') depth++;
    else if (ch === ')') {
      depth--;
      if (depth < 0) return false;
    }
  }
  return depth === 0;
}

/**
 * validateCssSubset throws on the first declaration the subset rejects.
 *
 * It is a throwing validator rather than a collector: a strict build wants the
 * failure and the first one is the actionable one. Report mode uses
 * cssSubsetErrors instead, because a gap register has to be complete.
 */
export function validateCssSubset(
  declarations: ReadonlyArray<{ property: string; value: string }>,
  file = '',
): void {
  const errors = cssSubsetErrors(declarations, file);
  if (errors.length > 0) throw new Error(errors[0]!);
}

/** cssGapProperties lists the documented out-of-subset properties. */
export function cssGapProperties(): string[] {
  return [...KNOWN_GAPS].filter((p) => !p.includes(':')).sort();
}
