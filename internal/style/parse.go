package style

import "errors"

// ErrNotImplemented is returned until the CSS subset parser lands (Milestone 2).
var ErrNotImplemented = errors.New("style: CSS subset parser not implemented yet (Milestone 2)")

// Stylesheet is a parsed collection of rules.
type Stylesheet struct {
	// Rules are applied in source order.
	Rules []Rule
}

// Rule pairs a selector with its declarations.
type Rule struct {
	// Selector matches target nodes.
	Selector Selector
	// Declarations are the property/value pairs of the rule body.
	Declarations []Declaration
}

// Declaration is a single property: value pair.
type Declaration struct {
	// Property is the CSS property name, e.g. "display".
	Property string
	// Value is the raw value text, validated during resolution.
	Value string
}

// Parse compiles a CSS subset source into a stylesheet.
//
// Milestone 2: start with type/class/id selectors and box properties, then
// extend deliberately (Module 9 Open Question 4).
func Parse(src string) (*Stylesheet, error) {
	_ = src
	return nil, ErrNotImplemented
}
