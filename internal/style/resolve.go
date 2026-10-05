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
	for _, m := range matches {
		for _, d := range m.decls {
			if err := apply(&st, d); err != nil {
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
		for _, d := range decls {
			if err := apply(&st, d); err != nil {
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
	case "border-color":
		v, err := parseColorValue(d.Value)
		if err != nil {
			return err
		}
		st.BorderColor = v
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
