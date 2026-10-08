package style

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arief-fajri/gowez/internal/ui"
)

// ruleRef is one stylesheet rule plus its global source order, used by
// the cascade tie-break.
type ruleRef struct {
	sel   Selector
	order int
	decls []Declaration
}

// Resolve computes the style of every node reachable from roots and
// returns it keyed by node ID.
//
// Cascade: initial values, then matching rules ordered by specificity
// (id, class, type) and source order, then the node's inline style
// attribute last. Errors name the node and the declaration — a failed
// resolve never returns a partial map (invariant I1).
func Resolve(roots []*ui.Node, sheets ...*Stylesheet) (map[ui.NodeID]ComputedStyle, error) {
	var rules []ruleRef
	order := 0
	for _, sh := range sheets {
		if sh == nil {
			continue
		}
		for _, r := range sh.Rules {
			rules = append(rules, ruleRef{sel: r.Selector, order: order, decls: r.Declarations})
			order++
		}
	}
	out := make(map[ui.NodeID]ComputedStyle)
	for _, root := range roots {
		if root == nil {
			continue
		}
		if err := resolveNode(root, nil, rules, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// cascadeMatch is one rule matched against the node being resolved.
type cascadeMatch struct {
	spec  [3]int
	order int
	decls []Declaration
}

// resolveNode resolves one node and its subtree in document order.
// parent supplies inherited values (color, font-size) for nodes whose
// rules do not set them explicitly.
func resolveNode(n *ui.Node, parent *ComputedStyle, rules []ruleRef, out map[ui.NodeID]ComputedStyle) error {
	st := Initial()
	if parent != nil {
		st.Color = parent.Color
		st.FontSize = parent.FontSize
		// Custom properties inherit, so start from the parent's resolved set.
		if len(parent.Custom) > 0 {
			st.Custom = make(map[string]string, len(parent.Custom))
			for k, v := range parent.Custom {
				st.Custom[k] = v
			}
		}
	}

	var matches []cascadeMatch
	for _, r := range rules {
		if r.sel.Matches(n) {
			matches = append(matches, cascadeMatch{spec: r.sel.spec, order: r.order, decls: r.decls})
		}
	}
	sort.SliceStable(matches, func(a, b int) bool {
		sa, sb := matches[a].spec, matches[b].spec
		for i := 0; i < 3; i++ { // (id, class, type), highest tuple first
			if sa[i] != sb[i] {
				return sa[i] < sb[i]
			}
		}
		return matches[a].order < matches[b].order
	})

	// Custom properties first, in cascade order, each substituted against the
	// set built so far. Every other declaration then substitutes against the
	// complete set. Doing it in this order is what makes a custom property able
	// to reference one declared beside it.
	if err := resolveCustomProperties(&st, matches, n); err != nil {
		return err
	}

	for _, m := range matches {
		for _, d := range m.decls {
			if isCustomProperty(d.Property) {
				continue
			}
			if err := applyResolved(&st, d); err != nil {
				return fmt.Errorf("style: node %d: %s: %w", n.ID, d.Property, err)
			}
		}
	}

	// Inline style has the highest priority: applied last, it beats
	// every selector regardless of specificity.
	if inline := n.GetAttribute("style"); inline != "" {
		decls, err := ParseInline(inline)
		if err != nil {
			return fmt.Errorf("style: node %d: inline style: %w", n.ID, err)
		}
		// Inline custom properties join the set before any inline var() is
		// substituted, so `style="--ink: red; color: var(--ink)"` works.
		if err := mergeCustomProperties(&st, decls, n, "inline"); err != nil {
			return err
		}
		for _, d := range decls {
			if isCustomProperty(d.Property) {
				continue
			}
			if err := applyResolved(&st, d); err != nil {
				return fmt.Errorf("style: node %d: inline %s: %w", n.ID, d.Property, err)
			}
		}
	}

	out[n.ID] = st
	for _, child := range n.Children {
		if err := resolveNode(child, &st, rules, out); err != nil {
			return err
		}
	}
	return nil
}

// applyResolved substitutes var() in d and applies it.
//
// Substitution happens here rather than at parse time because a custom property
// may be overridden on the element that uses it, so the only point at which the
// value is knowable is after the cascade. The substituted text goes through the
// same typed parser as at parse time, so deferring validation relocates it
// rather than weakening it.
func applyResolved(st *ComputedStyle, d Declaration) error {
	if strings.Contains(d.Value, "var(") {
		v, err := substituteVars(d.Value, st.Custom, nil)
		if err != nil {
			return err
		}
		d.Value = v
		// A shorthand that contained var() was recorded un-expanded at parse
		// time (its value was not yet a known token list), so expansion happens
		// here, after substitution. Longhands that were already expanded at
		// parse time fall through unchanged.
		if spec, ok := properties[d.Property]; ok && len(spec.longhands) > 0 {
			expanded, err := parseProperty(d.Property, spec, d.Value)
			if err != nil {
				return err
			}
			for _, lh := range expanded {
				if err := apply(st, lh); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return apply(st, d)
}

// resolveCustomProperties builds the element's custom property set.
//
// Declaration order does not matter: every custom property a matching rule sets
// is collected first, and only then substituted. Resolving them as they are
// encountered would make `--a: var(--b)` fail with "not defined" whenever `--b`
// happens to be declared later in the same rule — order-dependence that CSS does
// not have, and that would report a cycle as a missing name.
func resolveCustomProperties(st *ComputedStyle, matches []cascadeMatch, n *ui.Node) error {
	raw := make(map[string]string, 4)
	for _, m := range matches {
		for _, d := range m.decls {
			if isCustomProperty(d.Property) {
				raw[d.Property] = d.Value
			}
		}
	}
	if len(raw) == 0 {
		return nil
	}
	// Resolution sees the inherited set *and* this element's declarations, so a
	// descendant may define `--ink: var(--fg)` in terms of a `--fg` it inherits.
	lookup := make(map[string]string, len(st.Custom)+len(raw))
	for k, v := range st.Custom {
		lookup[k] = v
	}
	for k, v := range raw {
		lookup[k] = v
	}
	resolved := make(map[string]string, len(lookup))
	for name := range lookup {
		v, err := resolveCustomProp(name, lookup, resolved, nil)
		if err != nil {
			return fmt.Errorf("style: node %d: %s: %w", n.ID, name, err)
		}
		resolved[name] = v
	}
	if st.Custom == nil {
		st.Custom = make(map[string]string, len(resolved))
	}
	for name, v := range resolved {
		st.Custom[name] = v
	}
	return nil
}

// mergeCustomProperties folds decls' custom properties into st.Custom, resolving
// them against the set as it stands. Used for the inline declaration block,
// which beats every selector and therefore has to be merged last.
func mergeCustomProperties(st *ComputedStyle, decls []Declaration, n *ui.Node, where string) error {
	raw := make(map[string]string, 4)
	for _, d := range decls {
		if isCustomProperty(d.Property) {
			raw[d.Property] = d.Value
		}
	}
	if len(raw) == 0 {
		return nil
	}
	if st.Custom == nil {
		st.Custom = make(map[string]string, len(raw))
	}
	resolved := make(map[string]string, len(raw))
	for name := range raw {
		v, err := resolveCustomProp(name, raw, resolved, nil)
		if err != nil {
			return fmt.Errorf("style: node %d: %s %s: %w", n.ID, where, name, err)
		}
		resolved[name] = v
	}
	for name, v := range resolved {
		st.Custom[name] = v
	}
	return nil
}

// resolveCustomProp resolves one custom property's value, memoising as it goes.
//
// visiting is the chain currently being resolved, which is what turns
// `--a: var(--b); --b: var(--a)` into a reported cycle instead of a hang: there
// is no least fixed point, so there is no value to substitute.
//
// The recursion resolves every reference *before* substituting, and substitutes
// against a view in which resolved entries are already final. Resolving the
// reference as it is encountered instead makes the result depend on map
// iteration order: `--ink: var(--fg)` fails with "not defined" whenever `--ink`
// happens to be visited before `--fg`. That bug is invisible in a single run and
// appears intermittently under -count, which is why the order is fixed here
// rather than left to the map.
func resolveCustomProp(name string, lookup, resolved map[string]string, visiting []string) (string, error) {
	if v, ok := resolved[name]; ok {
		return v, nil
	}
	for _, seen := range visiting {
		if seen == name {
			return "", fmt.Errorf("custom property cycle: %s",
				strings.Join(append(visiting, name), " -> "))
		}
	}
	value, ok := lookup[name]
	if !ok {
		return "", fmt.Errorf("is not defined")
	}
	if !strings.Contains(value, "var(") {
		v := strings.TrimSpace(value)
		resolved[name] = v
		return v, nil
	}
	inner := append(append([]string{}, visiting...), name)
	for _, ref := range referencedNames(value) {
		if _, done := resolved[ref]; done {
			continue
		}
		if _, declared := lookup[ref]; !declared {
			continue // undefined: substituteVars reports it, or the fallback wins
		}
		if _, err := resolveCustomProp(ref, lookup, resolved, inner); err != nil {
			return "", err
		}
	}
	view := make(map[string]string, len(lookup))
	for k, v := range lookup {
		view[k] = v
	}
	for k, v := range resolved {
		view[k] = v
	}
	out, err := substituteVars(value, view, visiting)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	resolved[name] = out
	return out, nil
}

// referencedNames lists the custom properties a value refers to.
func referencedNames(value string) []string {
	var out []string
	for i := 0; i+4 <= len(value); i++ {
		if value[i:i+4] != "var(" {
			continue
		}
		depth := 0
		end := len(value)
		for j := i + 4; j < len(value); j++ {
			if value[j] == '(' {
				depth++
			} else if value[j] == ')' {
				if depth == 0 {
					break
				}
				depth--
			} else if depth == 0 && (value[j] == ',' || value[j] == ' ' || value[j] == '\t') {
				end = j
				break
			}
		}
		inner := value[i+4 : min(end, len(value))]
		name := inner
		if c := splitVarFallback(inner); c >= 0 {
			name = inner[:c]
		}
		if name = strings.TrimSpace(name); isCustomProperty(name) {
			out = append(out, name)
		}
		i += 3
	}
	return out
}

// apply sets one already-validated declaration on st. The typed parsers
// are the same ones used at parse time, so validation and computation can
// never disagree.
func apply(st *ComputedStyle, d Declaration) error {
	switch d.Property {
	case "display":
		v, err := parseDisplayValue(d.Value)
		if err != nil {
			return err
		}
		st.Display = v
	case "width":
		v, err := parseWidthValue(d.Value)
		if err != nil {
			return err
		}
		st.Width = v
	case "height":
		v, err := parseHeightValue(d.Value)
		if err != nil {
			return err
		}
		st.Height = v
	case "margin-top", "margin-right", "margin-bottom", "margin-left":
		v, err := parsePxValue(d.Value)
		if err != nil {
			return err
		}
		st.Margin[sideIndex(d.Property)] = v
	case "padding-top", "padding-right", "padding-bottom", "padding-left":
		v, err := parsePxValue(d.Value)
		if err != nil {
			return err
		}
		st.Padding[sideIndex(d.Property)] = v
	case "border-top-width", "border-right-width", "border-bottom-width", "border-left-width":
		v, err := parsePxValue(d.Value)
		if err != nil {
			return err
		}
		st.BorderWidth[sideIndex(d.Property)] = v
	case "border-top-color", "border-right-color", "border-bottom-color", "border-left-color":
		v, err := parseColorValue(d.Value)
		if err != nil {
			return err
		}
		st.BorderColor[sideIndex(d.Property)] = v
	case "background-color":
		v, err := parseColorValue(d.Value)
		if err != nil {
			return err
		}
		st.BackgroundColor = v
	case "color":
		v, err := parseColorValue(d.Value)
		if err != nil {
			return err
		}
		st.Color = v
	case "font-size":
		v, err := parseFontSizeValue(d.Value)
		if err != nil {
			return err
		}
		st.FontSize = v
	case "flex-direction":
		v, err := parseFlexDirectionValue(d.Value)
		if err != nil {
			return err
		}
		st.FlexDirection = v
	case "justify-content":
		v, err := parseJustifyValue(d.Value)
		if err != nil {
			return err
		}
		st.JustifyContent = v
	case "align-items":
		v, err := parseAlignValue(d.Value)
		if err != nil {
			return err
		}
		st.AlignItems = v
	case "gap":
		v, err := parsePxValue(d.Value)
		if err != nil {
			return err
		}
		st.Gap = v
	case "flex-grow":
		v, err := parseNumberValue(d.Value)
		if err != nil {
			return err
		}
		st.FlexGrow = v
	case "flex-shrink":
		v, err := parseNumberValue(d.Value)
		if err != nil {
			return err
		}
		st.FlexShrink = v
	default:
		return fmt.Errorf("unknown property %q", d.Property)
	}
	return nil
}

// sideIndex maps a side longhand (`margin-top`, `border-left-width`, …)
// to its [4] slot (top, right, bottom, left).
func sideIndex(property string) int {
	switch {
	case strings.Contains(property, "top"):
		return 0
	case strings.Contains(property, "right"):
		return 1
	case strings.Contains(property, "bottom"):
		return 2
	default: // left
		return 3
	}
}
