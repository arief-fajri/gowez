package style

import (
	"strings"

	"github.com/arief-fajri/gowez/internal/ui"
)

// Compound is one type/.class/#id/:pseudo selector without combinators.
type Compound struct {
	// Type is the element name; "" when the compound is class/id only.
	// The pseudo-type "text" matches text nodes.
	Type string
	// Classes are the .class tokens; all must match.
	Classes []string
	// ID is the #id token; "" when absent.
	ID string
	// Pseudo holds the supported pseudo-class tokens (:hover, :active,
	// :focus); all must match the node's interaction state.
	Pseudo []string
}

// specificity returns the (id, class, type) tuple for this compound.
// Each pseudo-class counts like a class, matching CSS (docs/CSS-SUBSET.md).
func (c Compound) specificity() (int, int, int) {
	id, cls, typ := 0, 0, 0
	if c.ID != "" {
		id++
	}
	cls += len(c.Classes) + len(c.Pseudo)
	if c.Type != "" {
		typ++
	}
	return id, cls, typ
}

// Selector is a parsed selector: a descendant chain of compounds. Steps
// are in document order — Steps[0] is the leftmost (ancestor) compound
// and the last step is the element the selector targets.
type Selector struct {
	// Raw is the original selector text, kept for diagnostics.
	Raw string
	// Steps is the descendant chain; empty selectors never match.
	Steps []Compound

	spec [3]int
}

// specificity returns the (id, class, type) tuple of the whole selector.
func (s Selector) specificity() (int, int, int) {
	return s.spec[0], s.spec[1], s.spec[2]
}

// Matches reports whether the selector applies to n. Only the supported
// subset matches; anything outside it was rejected at parse time — a
// selector never silently ignores a construct (docs/CSS-SUBSET.md).
func (s Selector) Matches(n *ui.Node) bool {
	if n == nil || len(s.Steps) == 0 {
		return false
	}
	target := s.Steps[len(s.Steps)-1]
	if !matchCompound(target, n) {
		return false
	}
	cur := n.Parent
	for i := len(s.Steps) - 2; i >= 0; i-- {
		for cur != nil && !matchCompound(s.Steps[i], cur) {
			cur = cur.Parent
		}
		if cur == nil {
			return false
		}
		cur = cur.Parent
	}
	return true
}

// matchCompound checks one compound against one node.
func matchCompound(c Compound, n *ui.Node) bool {
	if c.Type != "" {
		switch n.Kind {
		case ui.TextNode:
			if !strings.EqualFold(c.Type, "text") {
				return false
			}
		default:
			if !strings.EqualFold(c.Type, n.Tag) {
				return false
			}
		}
	}
	if c.ID != "" && n.GetAttribute("id") != c.ID {
		return false
	}
	if len(c.Classes) > 0 {
		have := strings.Fields(n.GetAttribute("class"))
		for _, want := range c.Classes {
			found := false
			for _, h := range have {
				if h == want {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	for _, p := range c.Pseudo {
		if !matchPseudo(p, n) {
			return false
		}
	}
	return true
}

// matchPseudo checks one supported pseudo-class against the node's
// interaction state. Unknown names never reach here — parseCompound
// rejects them at parse time (a selector never silently ignores a
// construct, docs/CSS-SUBSET.md).
func matchPseudo(name string, n *ui.Node) bool {
	switch name {
	case "hover":
		return n.State&ui.StateHovered != 0
	case "active":
		return n.State&ui.StatePressed != 0
	case "focus":
		return n.State&ui.StateFocused != 0
	}
	return false
}

// parseSelector parses one complex selector (descendant chain) from raw.
// base is the byte offset of raw within the stylesheet source, used only
// for error positions.
func parseSelector(src, raw string, base int) (Selector, error) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return Selector{}, errAt(src, base, "empty selector")
	}
	sel := Selector{Raw: strings.TrimSpace(raw)}
	for i, f := range fields {
		// Offset of this compound inside src (approximate when the
		// selector contained comments; exact otherwise).
		off := base + strings.Index(raw, f)
		if i > 0 && off < 0 {
			off = base
		}
		comp, err := parseCompound(src, f, off)
		if err != nil {
			return Selector{}, err
		}
		sel.Steps = append(sel.Steps, comp)
	}
	var id, cls, typ int
	for _, c := range sel.Steps {
		i, cl, t := c.specificity()
		id += i
		cls += cl
		typ += t
	}
	sel.spec = [3]int{id, cls, typ}
	return sel, nil
}

// parseCompound parses `type? (.class | #id)*`.
func parseCompound(src, s string, base int) (Compound, error) {
	var c Compound
	i := 0
	for i < len(s) && isNameChar(s[i]) {
		i++
	}
	if i > 0 {
		if !isIdentStart(s[0]) {
			return c, errAt(src, base, "invalid selector %q: must start with a letter", s)
		}
		c.Type = s[:i]
	}
	for i < len(s) {
		switch s[i] {
		case '.', '#':
			suffix := s[i]
			j := i + 1
			for j < len(s) && isNameChar(s[j]) {
				j++
			}
			if j == i+1 || !isIdentStart(s[i+1]) {
				return c, errAt(src, base+i, "invalid selector %q: expected a name after %q", s, string(suffix))
			}
			name := s[i+1 : j]
			if suffix == '.' {
				c.Classes = append(c.Classes, name)
			} else {
				if c.ID != "" {
					return c, errAt(src, base+i, "invalid selector %q: multiple #id in one compound", s)
				}
				c.ID = name
			}
			i = j
		case ':':
			if i+1 < len(s) && s[i+1] == ':' {
				return c, errAt(src, base+i,
					"unsupported pseudo-element in %q: :: constructs are not supported (docs/CSS-SUBSET.md)", s)
			}
			j := i + 1
			for j < len(s) && isNameChar(s[j]) {
				j++
			}
			if j == i+1 || !isIdentStart(s[i+1]) {
				return c, errAt(src, base+i, "invalid selector %q: expected a name after \":\"", s)
			}
			name := s[i+1 : j]
			switch name {
			case "hover", "active", "focus":
				c.Pseudo = append(c.Pseudo, name)
			default:
				return c, errAt(src, base+i,
					"unsupported pseudo-class %q in %q: only :hover, :active and :focus are supported (docs/CSS-SUBSET.md)",
					name, s)
			}
			i = j
		default:
			return c, errAt(src, base+i,
				"unsupported selector syntax %q in %q: only type, .class, #id, :pseudo-class and descendant combinators are supported (docs/CSS-SUBSET.md)",
				string(s[i]), s)
		}
	}
	if c.Type == "" && len(c.Classes) == 0 && c.ID == "" && len(c.Pseudo) == 0 {
		return c, errAt(src, base, "empty compound selector")
	}
	return c, nil
}

// isNameChar reports whether c may appear inside an identifier.
func isNameChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

// isIdentStart reports whether c may start an identifier.
func isIdentStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}
