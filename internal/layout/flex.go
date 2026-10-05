package layout

import "github.com/arief-fajri/gowez/internal/style"

// The flex enums live with the style values that produce them; the
// aliases keep the layout package's vocabulary stable for callers.

// FlexDirection is the main axis direction of a flex container.
type FlexDirection = style.FlexDirection

// JustifyContent distributes leftover space on the main axis.
type JustifyContent = style.JustifyContent

// AlignItems aligns children on the cross axis.
type AlignItems = style.AlignItems

const (
	// Row lays children out left to right.
	Row = style.Row
	// Column lays children out top to bottom.
	Column = style.Column

	// JustifyStart packs children at the main-axis start.
	JustifyStart = style.JustifyStart
	// JustifyCenter packs children at the main-axis center.
	JustifyCenter = style.JustifyCenter
	// JustifyEnd packs children at the main-axis end.
	JustifyEnd = style.JustifyEnd
	// JustifySpaceBetween puts leftover space between children.
	JustifySpaceBetween = style.JustifySpaceBetween

	// AlignStretch stretches children to fill the cross axis.
	AlignStretch = style.AlignStretch
	// AlignStart aligns children to the cross-axis start.
	AlignStart = style.AlignStart
	// AlignCenter centers children on the cross axis.
	AlignCenter = style.AlignCenter
	// AlignEnd aligns children to the cross-axis end.
	AlignEnd = style.AlignEnd
)
