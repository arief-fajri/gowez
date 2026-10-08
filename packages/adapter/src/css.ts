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
  'background-color',
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

/** color accepts hex forms and three keywords — parseColorValue. */
const color: Grammar = (value) => {
  if (value.startsWith('#')) {
    const hex = value.slice(1);
    if (![3, 4, 6, 8].includes(hex.length)) {
      return `want #RGB, #RRGGBB or #RRGGBBAA`;
    }
    if (!/^[0-9a-fA-F]+$/.test(hex)) return `invalid hex color`;
    return null;
  }
  if (['black', 'white', 'transparent'].includes(value)) return null;
  return `unsupported color (docs/CSS-SUBSET.md)`;
};

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
  'border-color': color,
  'background-color': color,
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
 * validateCssSubset throws on the first declaration the subset rejects.
 *
 * It is a throwing validator rather than a collector: a strict build wants the
 * failure, and report mode catches it and converts it into a finding.
 */
export function validateCssSubset(
  declarations: ReadonlyArray<{ property: string; value: string }>,
  file = '',
): void {
  const where = file ? `${file}: ` : '';
  for (const { property, value } of declarations) {
    if (property.startsWith('@')) {
      throw new Error(`${where}at-rule ${property} is not in the CSS subset`);
    }
    if (value.includes('!')) {
      throw new Error(`${where}${property}: !important is not supported`);
    }
    const grammar = GRAMMARS[property];
    if (!grammar) {
      const kind = KNOWN_GAPS.has(property) ? 'documented gap' : 'unsupported property';
      throw new Error(`${where}${property} is a ${kind} (docs/CSS-SUBSET.md)`);
    }
    const err = grammar(value.trim());
    if (err) {
      throw new Error(`${where}${property}: "${value}": ${err}`);
    }
  }
}

/** cssGapProperties lists the documented out-of-subset properties. */
export function cssGapProperties(): string[] {
  return [...KNOWN_GAPS].filter((p) => !p.includes(':')).sort();
}
