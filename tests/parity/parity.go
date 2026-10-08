// Package parity exposes the Go CSS subset's accept/reject decisions as data,
// so the TypeScript mirror in packages/adapter can be checked against them
// instead of trusted.
//
// This is the anti-drift half of the M5 CSS contract (docs/CSS-SUBSET.md). The
// authoritative rule set lives in internal/style; the adapter validates the
// same properties on its side to fail a build earlier. Two implementations of
// one rule set is a liability unless a test compares them — that is this
// package.
package parity

import (
	"fmt"

	"github.com/arief-fajri/gowez/internal/style"
)

// Case is one property/value pair probed against the Go subset.
type Case struct {
	Property string `json:"property"`
	Value    string `json:"value"`
	// Accept is what internal/style actually decided, not what was hoped for.
	Accept bool `json:"accept"`
	// Message is the parser's own diagnostic, empty when accepted.
	Message string `json:"message,omitempty"`
}

// Decision is one Case plus the parser verdict.
type Decision struct {
	Property string `json:"property"`
	Value    string `json:"value"`
	Accept   bool   `json:"accept"`
	Message  string `json:"message,omitempty"`
}

// Cases is the probe list: the boundaries and the values that have actually
// drifted between the two validators, plus a sample of what the subset accepts.
func Cases() []Case {
	return []Case{
		// lengths
		{"width", "100px", true, ""},
		{"width", "50%", true, ""},
		{"width", "auto", true, ""},
		{"width", "0", true, ""},
		{"width", "-10px", false, "negative length"},
		{"width", "100", false, "unitless width"},
		{"min-width", "120px", true, ""},
		{"max-width", "100%", true, ""},

		// Percentage heights: the case that motivated the parity test. The Go
		// subset rejects them explicitly rather than treating them as auto, so a
		// mirror that allows them silently changes layout.
		{"height", "640px", true, ""},
		{"height", "auto", true, ""},
		{"height", "0", true, ""},
		{"height", "100%", false, "percentage heights are not supported"},
		{"height", "50%", false, "percentage heights are not supported"},

		// box
		{"padding", "8px", true, ""},
		{"padding", "8px 12px", true, ""},
		{"padding", "0", true, ""},
		{"margin", "auto", true, ""},
		{"margin", "8px auto", true, ""},
		{"margin-top", "4px", true, ""},

		// flex
		{"display", "flex", true, ""},
		{"display", "block", true, ""},
		{"display", "none", true, ""},
		{"display", "grid", false, "no grid"},
		{"display", "inline", false, "no inline flow"},
		{"display", "table", false, "no table layout"},
		{"flex-direction", "column", true, ""},
		{"flex-direction", "row-reverse", true, ""},
		{"flex-direction", "sideways", false, "unknown keyword"},
		{"justify-content", "space-between", true, ""},
		{"align-items", "center", true, ""},
		{"gap", "8px", true, ""},
		{"flex-grow", "1", true, ""},

		// color
		{"color", "#ffffff", true, ""},
		{"color", "#fff", true, ""},
		{"color", "rgb(255, 0, 0)", true, ""},
		{"color", "rgb(255 0 0 / 50%)", true, ""},
		{"background-color", "#14161c", true, ""},
		{"color", "notacolor", false, "unknown color"},

		// borders
		{"border-width", "1px", true, ""},
		{"border-color", "#333945", true, ""},

		// Per-side border colour. `border-bottom: 1px solid red` is a divider,
		// so one colour for the whole box cannot express the common case; these
		// cases pin both the single-value shorthand and the longhands it expands
		// into, including the rejection of a fifth colour.
		{"border-color", "#111 #222 #333 #444", true, ""},
		{"border-color", "#111 #222", true, ""},
		{"border-color", "#111 #222 #333 #444 #555", false, "1-4 colors"},
		{"border-top-color", "#abc", true, ""},
		{"border-right-color", "#abc", true, ""},
		{"border-bottom-color", "rgb(1 2 3 / 50%)", true, ""},
		{"border-left-color", "notacolor", false, "unknown color"},

		// `none`-only properties. The subset paints no list marker and no
		// outline, so `none` is true rather than ignored — and every other value
		// must be refused rather than swallowed, or the stylesheet would claim
		// an effect the runtime does not produce.
		{"list-style", "none", true, ""},
		{"list-style", "disc", false, "only none"},
		{"outline", "none", true, ""},
		{"outline", "2px solid red", false, "only none"},
		{"outline", "0", false, "only none"},

		// typography
		{"font-size", "14px", true, ""},
		{"font-size", "0", true, ""},
		{"font-size", "large", false, "keyword font sizes"},
		{"line-height", "1.4", true, ""},
		{"line-height", "20px", true, ""},
		{"text-align", "center", true, ""},
		{"font-family", "sans-serif", true, ""},
		{"opacity", "0.5", true, ""},

		// custom properties and var(). The mirror cannot substitute, so parity
		// is asserted on *acceptance* of the deferral plus the structural checks
		// it does share: unbalanced parentheses are caught by both.
		{"--gap", "8px", true, ""},
		{"--gap", "var(--other)", true, ""},
		{"--gap", "", true, "empty custom property is legal (set but empty)"},
		{"color", "var(--fg)", true, ""},
		{"padding", "var(--gap) 12px", true, ""},
		{"color", "var(--a, var(--b, #00ff00))", true, ""},
		{"color", "var(--fg", false, "unterminated var("},
		// The deferral must not make an *unknown* property acceptable: the
		// property is rejected before its value is ever looked at. Without
		// these the mirror silently accepted `background: var(--x)`.
		{"background", "var(--accent)", false, "unsupported property"},
		{"grid-template-columns", "var(--cols)", false, "documented gap"},
		{"border-radius", "var(--r)", false, "documented gap"},
		{"--gap", "var(--fg", false, "unbalanced"},

		// color-mix: only `in srgb`, exactly two colours.
		{"color", "color-mix(in srgb, #000000, #ffffff)", true, ""},
		{"color", "color-mix(in srgb, #dc2626 45%, transparent)", true, ""},
		{"color", "color-mix(in srgb, #000 25%, #fff 75%)", true, ""},
		{"color", "color-mix(in oklch, #000, #fff)", false, "only in srgb"},
		{"color", "color-mix(in srgb, #000, #fff, #0f0)", false, "exactly two colours"},
		{"color", "color-mix(in srgb, #000 30%, #fff 30%)", false, "sum to 100"},
		{"color", "color-mix(in srgb, #000 150%, #fff)", false, "0-100%"},
		{"border-color", "color-mix(in srgb, #dc2626 45%, transparent)", true, ""},

		// documented gaps
		{"border-radius", "4px", false, "documented gap"},
		{"box-shadow", "0 0 2px #000", false, "documented gap"},
		{"transition", "all 0.2s", false, "documented gap"},
		{"overflow-y", "auto", false, "documented gap"},
		{"position", "absolute", false, "documented gap"},
		{"grid-template-columns", "1fr 1fr", false, "documented gap"},
		{"z-index", "10", false, "documented gap"},
		{"cursor", "pointer", false, "documented gap"},
	}
}

// AtRuleCases probes at-rules. The subset has none, so every case is a
// rejection — asserted rather than assumed.
func AtRuleCases() []Case {
	return []Case{
		{"@media", "(min-width: 600px)", false, "no media queries"},
		{"@supports", "display: flex", false, "no at-rules"},
		{"@keyframes", "fade", false, "no at-rules"},
	}
}

// Decide probes one property/value pair against the Go subset.
//
// A single declaration is wrapped in a rule so the failure comes from the value
// parser, not from the selector grammar.
func Decide(property, value string) Decision {
	css := fmt.Sprintf("probe { %s: %s; }", property, value)
	_, err := style.Parse(css)
	d := Decision{Property: property, Value: value, Accept: err == nil}
	if err != nil {
		d.Message = err.Error()
	}
	return d
}

// DecideAll probes every case and returns the decisions in order.
func DecideAll() []Decision {
	cases := append(Cases(), AtRuleCases()...)
	out := make([]Decision, 0, len(cases))
	for _, c := range cases {
		out = append(out, Decide(c.Property, c.Value))
	}
	return out
}
