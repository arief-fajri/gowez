package style

import (
	"fmt"
	"strconv"
	"strings"
)

// valueKind classifies the values one property accepts. The kind table is
// the single source of truth for what the subset supports; anything not
// listed here fails at parse time with a position (docs/CSS-SUBSET.md).
type valueKind int

const (
	kindColor valueKind = iota
	kindDisplay
	kindFlexDirection
	kindJustify
	kindAlign
	kindWidth        // auto | px | %
	kindHeight       // auto | px
	kindPx           // 0 | <n>px
	kindBoxShorthand // 1–4 px lengths, expanded to longhands
	kindColorSides   // 1–4 colors, expanded to per-side longhands
	kindNumber       // unitless, ≥ 0
	kindFontSize     // px, > 0
	kindNoneOnly     // exactly the keyword `none`
)

type propSpec struct {
	kind valueKind
	// longhands lists the properties a shorthand expands into, in CSS
	// order (top, right, bottom, left).
	longhands []string
}

// properties is the supported property table. Adding a row here is a
// deliberate subset extension: update docs/CSS-SUBSET.md in the same change.
var properties = map[string]propSpec{
	"display":             {kind: kindDisplay},
	"width":               {kind: kindWidth},
	"height":              {kind: kindHeight},
	"margin":              {kind: kindBoxShorthand, longhands: []string{"margin-top", "margin-right", "margin-bottom", "margin-left"}},
	"margin-top":          {kind: kindPx},
	"margin-right":        {kind: kindPx},
	"margin-bottom":       {kind: kindPx},
	"margin-left":         {kind: kindPx},
	"padding":             {kind: kindBoxShorthand, longhands: []string{"padding-top", "padding-right", "padding-bottom", "padding-left"}},
	"padding-top":         {kind: kindPx},
	"padding-right":       {kind: kindPx},
	"padding-bottom":      {kind: kindPx},
	"padding-left":        {kind: kindPx},
	"border-width":        {kind: kindBoxShorthand, longhands: []string{"border-top-width", "border-right-width", "border-bottom-width", "border-left-width"}},
	"border-top-width":    {kind: kindPx},
	"border-right-width":  {kind: kindPx},
	"border-bottom-width": {kind: kindPx},
	"border-left-width":   {kind: kindPx},
	"border-color":        {kind: kindColorSides, longhands: []string{"border-top-color", "border-right-color", "border-bottom-color", "border-left-color"}},
	"border-top-color":    {kind: kindColor},
	"border-right-color":  {kind: kindColor},
	"border-bottom-color": {kind: kindColor},
	"border-left-color":   {kind: kindColor},
	"background-color":    {kind: kindColor},
	"list-style":          {kind: kindNoneOnly},
	"outline":             {kind: kindNoneOnly},
	"color":               {kind: kindColor},
	"font-size":           {kind: kindFontSize},
	"flex-direction":      {kind: kindFlexDirection},
	"justify-content":     {kind: kindJustify},
	"align-items":         {kind: kindAlign},
	"gap":                 {kind: kindPx},
	"flex-grow":           {kind: kindNumber},
	"flex-shrink":         {kind: kindNumber},
}

// parseProperty validates value against spec and returns the declarations
// to record — shorthands expand into their longhands here, so the cascade
// only ever sees longhands.
func parseProperty(name string, spec propSpec, value string) ([]Declaration, error) {
	if strings.Contains(value, "!") {
		return nil, fmt.Errorf("%q: !important is not supported", value)
	}
	// A custom property is not in the typed table by design: its value is an
	// untyped token stream that only the consuming property interprets.
	if isCustomProperty(name) {
		return parseCustomProperty(name, value)
	}
	// A value containing var() cannot be validated here — the substitution has
	// not happened yet, and it happens per node at resolve time, after the
	// cascade. Resolve re-parses the substituted text with the same typed parser,
	// so deferring does not weaken the check, only relocates it.
	if strings.Contains(value, "var(") {
		if err := balancedParens(value); err != nil {
			return nil, err
		}
		return []Declaration{{Property: name, Value: value}}, nil
	}
	if spec.kind == kindBoxShorthand {
		parts := strings.Fields(value)
		if len(parts) == 0 || len(parts) > 4 {
			return nil, fmt.Errorf("want 1-4 px lengths, got %q", value)
		}
		var v [4]float64
		for i, p := range parts {
			n, err := parsePxValue(p)
			if err != nil {
				return nil, err
			}
			v[i] = n
		}
		// CSS shorthand expansion: 1 → all, 2 → vertical/horizontal,
		// 3 → top, horizontal, bottom, 4 → all four.
		full := [4]float64{v[0], v[0], v[0], v[0]}
		switch len(parts) {
		case 2:
			full = [4]float64{v[0], v[1], v[0], v[1]}
		case 3:
			full = [4]float64{v[0], v[1], v[2], v[1]}
		case 4:
			full = v
		}
		out := make([]Declaration, 0, 4)
		for i, lh := range spec.longhands {
			out = append(out, Declaration{Property: lh, Value: formatPx(full[i])})
		}
		return out, nil
	}
	if spec.kind == kindColorSides {
		// Split on top-level whitespace: a value may itself be a function with
		// spaces in it — `color-mix(in srgb, #f00, #fff)` is one colour, not five.
		parts := splitTopLevelFields(value)
		if len(parts) == 0 || len(parts) > 4 {
			return nil, fmt.Errorf("want 1-4 colors, got %q", value)
		}
		// Each token is validated as a colour, then redistributed to the sides
		// as its *original text*: a longhand must carry the author's spelling,
		// and a position-carrying error message has to name what was written.
		for _, p := range parts {
			if _, err := parseColorValue(p); err != nil {
				return nil, err
			}
		}
		var full [4]string
		for i := range full {
			full[i] = parts[0]
		}
		switch len(parts) {
		case 2:
			full = [4]string{parts[0], parts[1], parts[0], parts[1]}
		case 3:
			full = [4]string{parts[0], parts[1], parts[2], parts[1]}
		case 4:
			full = [4]string{parts[0], parts[1], parts[2], parts[3]}
		}
		out := make([]Declaration, 0, 4)
		for i, lh := range spec.longhands {
			out = append(out, Declaration{Property: lh, Value: full[i]})
		}
		return out, nil
	}
	if err := validateValue(spec.kind, value); err != nil {
		return nil, err
	}
	return []Declaration{{Property: name, Value: value}}, nil
}

// validateValue runs the typed parser for kind and discards the result.
func validateValue(kind valueKind, value string) error {
	_, err := parseTyped(kind, value)
	return err
}

// parseTyped dispatches to the typed parser for kind.
func parseTyped(kind valueKind, value string) (any, error) {
	switch kind {
	case kindColor:
		return parseColorValue(value)
	case kindDisplay:
		return parseDisplayValue(value)
	case kindFlexDirection:
		return parseFlexDirectionValue(value)
	case kindJustify:
		return parseJustifyValue(value)
	case kindAlign:
		return parseAlignValue(value)
	case kindWidth:
		return parseWidthValue(value)
	case kindHeight:
		return parseHeightValue(value)
	case kindPx:
		return parsePxValue(value)
	case kindNumber:
		return parseNumberValue(value)
	case kindFontSize:
		return parseFontSizeValue(value)
	case kindNoneOnly:
		return parseNoneValue(value)
	}
	return nil, fmt.Errorf("internal: no parser for value kind %d", kind)
}

// parseNoneValue accepts only `none`.
//
// It exists for `list-style` and `outline`, whose only value the subset honours
// is `none` — not because nothing is drawn, but because nothing is drawn
// *already*: there is no list marker and no outline in the render pipeline. So
// `none` is true rather than ignored, and every other value is refused rather
// than silently swallowed. Accepting `outline: 2px solid red` would claim an
// effect the runtime does not produce (G-UPG-04).
func parseNoneValue(s string) (any, error) {
	if s == "none" {
		return nil, nil
	}
	return nil, fmt.Errorf("only none is supported (the subset paints no list marker and no outline), got %q", s)
}

// parsePxValue accepts "0" or "<n>px"; negative lengths are rejected.
func parsePxValue(s string) (float64, error) {
	if s == "0" {
		return 0, nil
	}
	if !strings.HasSuffix(s, "px") {
		return 0, fmt.Errorf("%q: want a px length (e.g. 8px)", s)
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 64)
	if err != nil {
		return 0, fmt.Errorf("%q: invalid length", s)
	}
	if v < 0 {
		return 0, fmt.Errorf("%q: negative lengths are not supported", s)
	}
	return v, nil
}

// parseWidthValue accepts auto, px, and percentages of the containing block.
func parseWidthValue(s string) (Length, error) {
	if s == "auto" {
		return Length{Kind: LengthAuto}, nil
	}
	if strings.HasSuffix(s, "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err != nil || v < 0 {
			return Length{}, fmt.Errorf("%q: invalid percentage", s)
		}
		return Length{Kind: LengthPercent, Value: v}, nil
	}
	v, err := parsePxValue(s)
	if err != nil {
		return Length{}, err
	}
	return Length{Kind: LengthPx, Value: v}, nil
}

// parseHeightValue accepts auto and px only. Percentage heights are
// rejected rather than silently treated as auto (docs/CSS-SUBSET.md).
func parseHeightValue(s string) (Length, error) {
	l, err := parseWidthValue(s)
	if err != nil {
		return Length{}, err
	}
	if l.Kind == LengthPercent {
		return Length{}, fmt.Errorf("%q: percentage heights are not supported (docs/CSS-SUBSET.md)", s)
	}
	return l, nil
}

// parseNumberValue accepts a unitless number ≥ 0.
func parseNumberValue(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%q: want a number", s)
	}
	if v < 0 {
		return 0, fmt.Errorf("%q: negative numbers are not supported", s)
	}
	return v, nil
}

// parseFontSizeValue accepts a positive px length.
func parseFontSizeValue(s string) (float64, error) {
	v, err := parsePxValue(s)
	if err != nil {
		return 0, err
	}
	if v <= 0 {
		return 0, fmt.Errorf("%q: font-size must be positive", s)
	}
	return v, nil
}

// parseColorValue accepts #RGB, #RRGGBB, #RRGGBBAA, and the keywords
// black, white, transparent. The result is 0xRRGGBBAA.
func parseColorValue(s string) (uint32, error) {
	if strings.HasPrefix(s, "color-mix(") {
		return parseColorMixValue(s)
	}
	if strings.HasPrefix(s, "#") {
		h := s[1:]
		switch len(h) {
		case 3, 4:
			var expanded strings.Builder
			expanded.Grow(len(h) * 2)
			for i := 0; i < len(h); i++ {
				expanded.WriteByte(h[i])
				expanded.WriteByte(h[i])
			}
			h = expanded.String()
		case 6:
			h += "ff"
		case 8:
		default:
			return 0, fmt.Errorf("%q: want #RGB, #RRGGBB or #RRGGBBAA", s)
		}
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return 0, fmt.Errorf("%q: invalid hex color", s)
		}
		return uint32(v), nil
	}
	switch s {
	case "black":
		return 0x000000ff, nil
	case "white":
		return 0xffffffff, nil
	case "transparent":
		return 0x00000000, nil
	}
	return 0, fmt.Errorf("%q: unsupported color (docs/CSS-SUBSET.md)", s)
}

// parseDisplayValue maps the display keywords.
func parseDisplayValue(s string) (Display, error) {
	switch s {
	case "block":
		return DisplayBlock, nil
	case "flex":
		return DisplayFlex, nil
	case "none":
		return DisplayNone, nil
	}
	return 0, fmt.Errorf("%q: unsupported value (docs/CSS-SUBSET.md)", s)
}

// parseFlexDirectionValue maps flex-direction keywords.
func parseFlexDirectionValue(s string) (FlexDirection, error) {
	switch s {
	case "row":
		return Row, nil
	case "column":
		return Column, nil
	}
	return 0, fmt.Errorf("%q: unsupported value (docs/CSS-SUBSET.md)", s)
}

// parseJustifyValue maps justify-content keywords.
func parseJustifyValue(s string) (JustifyContent, error) {
	switch s {
	case "start":
		return JustifyStart, nil
	case "center":
		return JustifyCenter, nil
	case "end":
		return JustifyEnd, nil
	case "space-between":
		return JustifySpaceBetween, nil
	}
	return 0, fmt.Errorf("%q: unsupported value (docs/CSS-SUBSET.md)", s)
}

// parseAlignValue maps align-items keywords.
func parseAlignValue(s string) (AlignItems, error) {
	switch s {
	case "stretch":
		return AlignStretch, nil
	case "start":
		return AlignStart, nil
	case "center":
		return AlignCenter, nil
	case "end":
		return AlignEnd, nil
	}
	return 0, fmt.Errorf("%q: unsupported value (docs/CSS-SUBSET.md)", s)
}

// formatPx renders a non-negative length in canonical px form.
func formatPx(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64) + "px"
}
