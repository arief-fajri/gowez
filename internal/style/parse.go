package style

import (
	"fmt"
	"strings"
)

// Stylesheet is a parsed collection of rules.
type Stylesheet struct {
	// Rules are applied in source order.
	Rules []Rule
}

// Rule pairs a selector with its declarations.
type Rule struct {
	// Selector matches target nodes.
	Selector Selector
	// Declarations are the property/value pairs of the rule body,
	// already validated and expanded to longhands.
	Declarations []Declaration
}

// Declaration is a single property: value pair.
type Declaration struct {
	// Property is the CSS property name, e.g. "display".
	Property string
	// Value is the validated raw value text.
	Value string
}

// Parse compiles a CSS subset source into a stylesheet.
//
// Every construct outside the subset — unknown properties, unsupported
// values, other combinators, pseudo-classes, @-rules, !important — fails
// with a line/column error. Nothing is silently ignored
// (docs/CSS-SUBSET.md, G-UPG-04).
func Parse(src string) (*Stylesheet, error) {
	sheet := &Stylesheet{}
	i := 0
	for {
		next, err := skipWS(src, i)
		if err != nil {
			return nil, err
		}
		i = next
		if i >= len(src) {
			return sheet, nil
		}
		sels, brace, err := parseSelectorList(src, i)
		if err != nil {
			return nil, err
		}
		decls, end, err := parseDeclarationBlock(src, brace+1, '}')
		if err != nil {
			return nil, err
		}
		for _, sel := range sels {
			sheet.Rules = append(sheet.Rules, Rule{Selector: sel, Declarations: decls})
		}
		i = end
	}
}

// ParseInline compiles the value of a node's style attribute into
// declarations. It follows the same validation rules as Parse.
func ParseInline(src string) ([]Declaration, error) {
	decls, end, err := parseDeclarationBlock(src, 0, 0)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(src[end:]) != "" {
		return nil, errAt(src, end, "unexpected content after declarations")
	}
	return decls, nil
}

// parseSelectorList reads `sel (',' sel)* '{'` starting at i and returns
// the parsed selectors plus the index of the '{'.
func parseSelectorList(src string, i int) ([]Selector, int, error) {
	var (
		raws   []string
		starts []int
		cur    strings.Builder
		part   = i
	)
	for {
		if i >= len(src) {
			return nil, 0, errAt(src, len(src), "unterminated rule: missing '{'")
		}
		if strings.HasPrefix(src[i:], "/*") {
			cur.WriteString(src[part:i])
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, 0, errAt(src, i, "unterminated comment")
			}
			i += 2 + end + 2
			part = i
			continue
		}
		switch src[i] {
		case '{', ',':
			cur.WriteString(src[part:i])
			raws = append(raws, cur.String())
			starts = append(starts, part)
			cur.Reset()
			if src[i] == ',' {
				i++
				// Skip whitespace after the comma so the next part
				// starts at its first character.
				next, err := skipWS(src, i)
				if err != nil {
					return nil, 0, err
				}
				i = next
				part = i
				if i < len(src) && src[i] == '{' {
					return nil, 0, errAt(src, i, "empty selector in group")
				}
				continue
			}
			sels := make([]Selector, 0, len(raws))
			for k, raw := range raws {
				sel, err := parseSelector(src, raw, starts[k])
				if err != nil {
					return nil, 0, err
				}
				sels = append(sels, sel)
			}
			return sels, i, nil
		default:
			i++
		}
	}
}

// parseDeclarationBlock reads `ident : value (; …)*` until term (or EOF
// when term is 0) and returns the declarations plus the index after term.
func parseDeclarationBlock(src string, i int, term byte) ([]Declaration, int, error) {
	var decls []Declaration
	for {
		next, err := skipWS(src, i)
		if err != nil {
			return nil, i, err
		}
		i = next
		if i >= len(src) {
			if term != 0 {
				return nil, i, errAt(src, len(src), "unterminated block: missing %q", string(term))
			}
			return decls, i, nil
		}
		if src[i] == term {
			return decls, i + 1, nil
		}
		nameStart := i
		for i < len(src) && isNameChar(src[i]) {
			i++
		}
		if i == nameStart {
			return nil, i, errAt(src, nameStart, "unexpected %q where a property name was expected", string(src[nameStart]))
		}
		if !isIdentStart(src[nameStart]) && !isCustomPropertyName(src[nameStart:i]) {
			return nil, i, errAt(src, nameStart, "invalid property name %q", src[nameStart:i])
		}
		name := src[nameStart:i]

		next, err = skipWS(src, i)
		if err != nil {
			return nil, i, err
		}
		i = next
		if i >= len(src) || src[i] != ':' {
			return nil, i, errAt(src, nameStart, "property %q: expected ':'", name)
		}
		i++ // ':'

		valStart := i
		for i < len(src) && src[i] != ';' && src[i] != term {
			if strings.HasPrefix(src[i:], "/*") {
				end := strings.Index(src[i+2:], "*/")
				if end < 0 {
					return nil, i, errAt(src, i, "unterminated comment")
				}
				i += 2 + end + 2
				continue
			}
			i++
		}
		value := strings.TrimSpace(src[valStart:i])
		// A custom property may be declared empty (`--ink: ;`). CSS treats that
		// as "set but empty", and var(--ink) must then fall back rather than
		// resolve to nothing — so the value cannot be rejected here.
		if value == "" && !isCustomProperty(name) {
			return nil, i, errAt(src, nameStart, "property %q: missing value", name)
		}

		// A custom property is not in the typed table: its value is an untyped
		// token stream that only the consuming property interprets.
		if isCustomProperty(name) {
			expanded, err := parseCustomProperty(name, value)
			if err != nil {
				return nil, i, errAt(src, valStart, "%s: %v", name, err)
			}
			decls = append(decls, expanded...)
			next, err = skipWS(src, i)
			if err != nil {
				return nil, i, err
			}
			i = next
			if i < len(src) && src[i] == ';' {
				i++
			}
			continue
		}

		spec, ok := properties[name]
		if !ok {
			return nil, i, errAt(src, nameStart, "unsupported property %q (docs/CSS-SUBSET.md)", name)
		}
		expanded, err := parseProperty(name, spec, value)
		if err != nil {
			return nil, i, errAt(src, valStart, "%s: %v", name, err)
		}
		decls = append(decls, expanded...)

		next, err = skipWS(src, i)
		if err != nil {
			return nil, i, err
		}
		i = next
		if i < len(src) && src[i] == ';' {
			i++
		}
	}
}

// skipWS advances past whitespace and comments.
func skipWS(src string, i int) (int, error) {
	for i < len(src) {
		switch {
		case src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r':
			i++
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return i, errAt(src, i, "unterminated comment")
			}
			i += 2 + end + 2
		default:
			return i, nil
		}
	}
	return i, nil
}

// errAt formats an error carrying the 1-based line/column of off.
func errAt(src string, off int, format string, args ...any) error {
	if off < 0 {
		off = 0
	}
	if off > len(src) {
		off = len(src)
	}
	line, col := 1, 1
	for j := 0; j < off; j++ {
		if src[j] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return fmt.Errorf("style: line %d:%d: %s", line, col, fmt.Sprintf(format, args...))
}
