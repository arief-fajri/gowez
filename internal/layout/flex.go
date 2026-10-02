package layout

// FlexDirection is the main axis direction of a flex container (Milestone 2).
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
