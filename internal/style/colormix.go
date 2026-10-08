package style

import (
	"fmt"
	"math"
	"strings"
)

// color-mix() support (CSS Color 4), restricted to what the subset needs.
//
// Only `color-mix(in srgb, <color> <pct>%, <color> <pct>?)` is accepted. The
// restriction is deliberate rather than provisional: every other color space in
// CSS Color 4 mixes in a different space with different edge behaviour (and some
// with hue-interpolation rules), so a half-implemented `color-mix` that accepted
// an unsupported space would return a *wrong colour* rather than an error. A
// named rejection is the honest outcome.
//
// Interpolation follows the spec for srgb: percentages are of the total, mixed
// linearly, and **alpha is premultiplied** — mixing `#ff0000` with
// `transparent` at 50% must yield `rgba(255,0,0,0.5)` rather than
// `rgba(128,0,0,0.5)`, which is the classic result of interpolating alpha
// independently.

func parseColorMixValue(s string) (uint32, error) {
	if !strings.HasSuffix(s, ")") {
		return 0, fmt.Errorf("%q: unterminated color-mix()", s)
	}
	inner := strings.TrimSpace(s[len("color-mix(") : len(s)-1])

	// Split the space clause off the two color clauses.
	comma := strings.Index(inner, ",")
	if comma < 0 {
		return 0, fmt.Errorf("%q: want color-mix(in srgb, <color>, <color>)", s)
	}
	space := strings.TrimSpace(inner[:comma])
	if !strings.EqualFold(space, "in srgb") {
		// Explicitly named so an author knows the value was understood and
		// refused, rather than quietly mixed in the wrong space.
		return 0, fmt.Errorf("%q: only 'in srgb' is supported; %q would mix in a different space and give a different colour", s, space)
	}

	parts := splitTopLevel(inner[comma+1:])
	if len(parts) != 2 {
		return 0, fmt.Errorf("%q: want exactly two colours, got %d", s, len(parts))
	}

	c1, p1, hasP1, err := parseColorMixClause(parts[0])
	if err != nil {
		return 0, fmt.Errorf("%s: first colour: %w", s, err)
	}
	c2, p2, hasP2, err := parseColorMixClause(parts[1])
	if err != nil {
		return 0, fmt.Errorf("%s: second colour: %w", s, err)
	}

	// One percentage is enough; the other is 100% minus it.
	switch {
	case hasP1 && hasP2:
		sum := p1 + p2
		if math.Abs(sum-100) > 1e-6 {
			return 0, fmt.Errorf("%s: percentages must sum to 100%%, got %g%% and %g%%", s, p1, p2)
		}
	case hasP1:
		p2 = 100 - p1
	case hasP2:
		p1 = 100 - p2
	default:
		p1, p2 = 50, 50
	}
	w1, w2 := p1/100, p2/100

	// Premultiplied interpolation, per CSS Color 4: premultiply each colour by
	// its alpha, mix the channels by weight, then divide by the mixed alpha.
	// Interpolating the channels independently would turn
	// `#ff0000 50% + transparent` into dark red instead of half-transparent red.
	a1 := float64(c1&0xff) / 255
	a2 := float64(c2&0xff) / 255
	alpha := a1*w1 + a2*w2
	var r, g, b float64
	if alpha > 0 {
		r = mixChannel(c1>>24, a1, c2>>24, a2, w1, w2, alpha)
		g = mixChannel((c1>>16)&0xff, a1, (c2>>16)&0xff, a2, w1, w2, alpha)
		b = mixChannel((c1>>8)&0xff, a1, (c2>>8)&0xff, a2, w1, w2, alpha)
	}
	return packColor(r, g, b, alpha), nil
}

// mixChannel premultiplies, mixes and un-premultiplies one colour channel.
func mixChannel(c1 uint32, a1 float64, c2 uint32, a2 float64, w1, w2, alpha float64) float64 {
	p1 := float64(c1) * a1
	p2 := float64(c2) * a2
	return (p1*w1 + p2*w2) / alpha
}

// parseColorMixClause parses "<color> [pct]%" into a colour and its percentage.
func parseColorMixClause(s string) (uint32, float64, bool, error) {
	t := strings.TrimSpace(s)
	fields := strings.Fields(t)
	if len(fields) == 0 || len(fields) > 2 {
		return 0, 0, false, fmt.Errorf("want a colour and an optional percentage, got %q", t)
	}
	color, err := parseColorValue(fields[0])
	if err != nil {
		return 0, 0, false, err
	}
	if len(fields) == 1 {
		return color, 0, false, nil
	}
	pct := fields[1]
	if !strings.HasSuffix(pct, "%") {
		return 0, 0, false, fmt.Errorf("percentage must end in %%, got %q", pct)
	}
	v, err := parseNumberValue(strings.TrimSuffix(pct, "%"))
	if err != nil {
		return 0, 0, false, fmt.Errorf("percentage %q: %w", pct, err)
	}
	if v < 0 || v > 100 {
		return 0, 0, false, fmt.Errorf("percentage %g%% is outside 0%%-100%%", v)
	}
	return color, v, true, nil
}

// splitTopLevel splits on commas that are not inside parentheses, so
// `color-mix(in srgb, rgb(1, 2, 3), #fff)` splits into two clauses.
func splitTopLevel(s string) []string {
	var out []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

// packColor clamps channels into 0-255 and packs as 0xRRGGBBAA.
func packColor(r, g, b, a float64) uint32 {
	return uint32(clamp255(r))<<24 |
		uint32(clamp255(g))<<16 |
		uint32(clamp255(b))<<8 |
		uint32(clamp255(a*255))
}

func clamp255(v float64) int {
	r := math.Round(v)
	switch {
	case r < 0:
		return 0
	case r > 255:
		return 255
	default:
		return int(r)
	}
}

// splitTopLevelFields splits s on whitespace that is not inside parentheses, so
// a function call with spaces in it stays one field.
func splitTopLevelFields(s string) []string {
	var out []string
	depth := 0
	start := -1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}
