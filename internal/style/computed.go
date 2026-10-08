package style

// LengthKind distinguishes the three width/height value forms.
type LengthKind int

const (
	// LengthAuto means the size is content-derived.
	LengthAuto LengthKind = iota
	// LengthPx means a fixed size in logical pixels.
	LengthPx
	// LengthPercent means a percentage of the containing block content
	// width (width only — percentage heights are rejected).
	LengthPercent
)

// Length is one resolved size value. The zero value is auto, matching the
// documented initial value of width/height.
type Length struct {
	Kind  LengthKind
	Value float64
}

// Display is the layout participation mode of a node.
type Display int

const (
	// DisplayBlock participates as a block box.
	DisplayBlock Display = iota
	// DisplayInline participates as an inline box. The value exists for
	// forward compatibility; the subset rejects `display: inline` until
	// inline layout is implemented (docs/CSS-SUBSET.md).
	DisplayInline
	// DisplayFlex lays out children with the flex algorithm.
	DisplayFlex
	// DisplayNone removes the node from layout and rendering.
	DisplayNone
)

// FlexDirection is the main axis direction of a flex container.
type FlexDirection int

const (
	// Row lays children out left to right.
	Row FlexDirection = iota
	// Column lays children out top to bottom.
	Column
)

// JustifyContent distributes leftover space on the main axis.
type JustifyContent int

const (
	// JustifyStart packs children at the main-axis start.
	JustifyStart JustifyContent = iota
	// JustifyCenter packs children at the main-axis center.
	JustifyCenter
	// JustifyEnd packs children at the main-axis end.
	JustifyEnd
	// JustifySpaceBetween puts leftover space between children.
	JustifySpaceBetween
)

// AlignItems aligns children on the cross axis.
type AlignItems int

const (
	// AlignStretch stretches children to fill the cross axis.
	AlignStretch AlignItems = iota
	// AlignStart aligns children to the cross-axis start.
	AlignStart
	// AlignCenter centers children on the cross axis.
	AlignCenter
	// AlignEnd aligns children to the cross-axis end.
	AlignEnd
)

// ComputedStyle is the resolved style of one node after cascade and value
// computation. Sides are ordered top, right, bottom, left.
type ComputedStyle struct {
	// Display selects the layout mode.
	Display Display
	// Width and Height are content-box sizes; zero value is auto.
	Width, Height Length
	// Margin, Padding, BorderWidth are px lengths per side.
	Margin, Padding, BorderWidth [4]float64
	// BorderColor is the border fill per side as 0xRRGGBBAA, in the same
	// order as BorderWidth.
	//
	// Per side, not one colour, because `border-bottom: 1px solid red` is a
	// divider and a single value cannot express it: the common case for a
	// border is "one edge, one colour, the rest absent".
	BorderColor [4]uint32
	// BackgroundColor is the box fill as 0xRRGGBBAA.
	BackgroundColor uint32
	// Color is the text color as 0xRRGGBBAA.
	Color uint32
	// FontSize is the resolved font size in logical pixels.
	FontSize float64
	// FlexDirection, JustifyContent, AlignItems, Gap apply when the
	// element's display is flex.
	FlexDirection  FlexDirection
	JustifyContent JustifyContent
	AlignItems     AlignItems
	Gap            float64
	// FlexGrow and FlexShrink apply to flex items on the main axis.
	FlexGrow, FlexShrink float64
	// Custom holds the element's resolved `--*` properties, after var()
	// substitution.
	//
	// Resolved rather than raw so a descendant can inherit a flat value: under
	// inheritance CSS re-evaluates a custom property in the child's context, so
	// `--a: var(--b)` where `--b` is overridden below must re-resolve. Carrying
	// the substituted text is what makes that work without re-walking the
	// parent's declarations.
	Custom map[string]string
}

// Initial returns the documented initial values (docs/CSS-SUBSET.md). Every
// resolved style starts here, so a node with no matching rule still has a
// complete ComputedStyle.
func Initial() ComputedStyle {
	return ComputedStyle{
		Display:         DisplayBlock,
		Width:           Length{Kind: LengthAuto},
		Height:          Length{Kind: LengthAuto},
		Color:           0x000000ff,
		BackgroundColor: 0x00000000,
		BorderColor:     [4]uint32{0x00000000, 0x00000000, 0x00000000, 0x00000000},
		FontSize:        16,
		FlexDirection:   Row,
		JustifyContent:  JustifyStart,
		AlignItems:      AlignStretch,
		FlexGrow:        0,
		FlexShrink:      1,
	}
}
