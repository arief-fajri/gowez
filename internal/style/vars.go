package style

import (
	"fmt"
	"strings"
)

// Custom property support (docs/CSS-SUBSET.md §custom properties).
//
// A custom property's value is a **raw token stream**, deliberately not parsed.
// `--x: 1px solid #f00` has to stay legal so that a later shorthand can consume
// it; parsing it here would tie every custom property to the grammar of the
// property that happens to use it first, which is not how CSS works.
//
// Substitution happens at **computed-value time** — after the cascade has
// decided which declaration wins, and before the substituted text is validated.
// That order is the only one under which this works:
//
//	.app  { --brand: #f00 }
//	.card { border-color: var(--brand) }
//
// Substituting at parse time would bake `#f00` into `.card` at build time and
// then ignore a narrower override on a descendant.

// isCustomProperty reports whether name is a `--*` custom property.
func isCustomProperty(name string) bool {
	return strings.HasPrefix(name, "--") && len(name) > 2
}

// isCustomPropertyName reports whether a scanned token is a `--*` name.
//
// A property name normally has to start with a letter or `_`, which `--bg`
// does not. Custom properties are the one exception, and they are recognised
// here rather than by loosening the identifier rule, so that `-2px` can still
// never be mistaken for a property.
func isCustomPropertyName(tok string) bool {
	return strings.HasPrefix(tok, "--") && len(tok) > 2
}

// parseCustomProperty validates the shape of a custom property's value.
//
// The value is kept verbatim, so the checks are structural rather than
// grammatical: it must be non-empty, must not carry `!important`, and its
// parentheses must balance — otherwise `var(--x)` would silently disappear
// during substitution and the failure would surface far from its cause.
func parseCustomProperty(name, value string) ([]Declaration, error) {
	if strings.Contains(value, "!") {
		return nil, fmt.Errorf("%q: !important is not supported", value)
	}
	v := strings.TrimSpace(value)
	if err := balancedParens(v); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return []Declaration{{Property: name, Value: v}}, nil
}

// balancedParens checks that every ( has a matching ) in that order.
func balancedParens(s string) error {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return fmt.Errorf("unmatched ) in %q", s)
			}
		}
	}
	if depth != 0 {
		return fmt.Errorf("unmatched ( in %q", s)
	}
	return nil
}

// substituteVars replaces every var() reference in value using custom.
//
// stack carries the names currently being resolved, which is what makes a
// reference cycle an error instead of an infinite loop: `--a: var(--b)` and
// `--b: var(--a)` has no least fixed point, so there is nothing sensible to
// substitute and reporting "empty" would be a silent misrender.
func substituteVars(value string, custom map[string]string, stack []string) (string, error) {
	if !strings.Contains(value, "var(") {
		return value, nil
	}
	var out strings.Builder
	out.Grow(len(value))
	for i := 0; i < len(value); {
		start := strings.Index(value[i:], "var(")
		if start < 0 {
			out.WriteString(value[i:])
			break
		}
		start += i
		out.WriteString(value[i:start])

		// Find the matching close paren, tracking nesting so a fallback may
		// itself contain var(): var(--a, var(--b, red)). The scan starts *after*
		// the `var(` that opened it, so the closing paren of this same call is
		// seen at depth 0.
		depth := 0
		end := -1
		for j := start + 4; j < len(value); j++ {
			switch value[j] {
			case '(':
				depth++
			case ')':
				if depth == 0 {
					end = j
				} else {
					depth--
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			return "", fmt.Errorf("unterminated var( in %q", value)
		}

		inner := value[start+4 : end]
		name := inner
		fallback := ""
		hasFallback := false
		if comma := splitVarFallback(inner); comma >= 0 {
			name = strings.TrimSpace(inner[:comma])
			fallback = strings.TrimSpace(inner[comma+1:])
			hasFallback = true
		}
		if !isCustomProperty(strings.TrimSpace(name)) {
			return "", fmt.Errorf("var(%s): not a custom property name", strings.TrimSpace(name))
		}
		name = strings.TrimSpace(name)

		replacement, ok := custom[name]
		// An empty value counts as unset, matching CSS: `--x: ; color: var(--x)`
		// must use the fallback, not resolve to nothing.
		if !ok || strings.TrimSpace(replacement) == "" {
			if !hasFallback {
				return "", fmt.Errorf("var(%s) is not defined", name)
			}
			replacement = fallback
		} else {
			for _, seen := range stack {
				if seen == name {
					return "", fmt.Errorf("custom property cycle: %s -> %s", strings.Join(append(stack, name), " -> "), name)
				}
			}
			nested, err := substituteVars(replacement, custom, append(stack, name))
			if err != nil {
				return "", err
			}
			replacement = nested
		}

		// A fallback may itself reference var(), so resolve it before splicing.
		if hasFallback && strings.Contains(replacement, "var(") {
			nested, err := substituteVars(replacement, custom, stack)
			if err != nil {
				return "", err
			}
			replacement = nested
		}
		out.WriteString(strings.TrimSpace(replacement))
		i = end + 1
	}
	return out.String(), nil
}

// splitVarFallback returns the index of the comma separating a var() name from
// its fallback, or -1. The scan ignores commas nested inside parentheses so a
// fallback like `var(--a, rgb(1, 2, 3))` is split at the right place.
func splitVarFallback(inner string) int {
	depth := 0
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
