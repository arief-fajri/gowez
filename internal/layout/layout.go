// Package layout computes deterministic geometry for the UI tree.
//
// Layout is a pure function of (tree, computed styles, viewport): the same
// input always yields the same boxes and text lines (Module 6: layout
// produces deterministic geometry). Nodes with display:none produce no
// geometry and no text lines.
//
// Scope: block flow, flex containers, and greedy word-wrapped text.
// Divergences from CSS (no margin collapsing, no inline flow, …) are
// documented in docs/CSS-SUBSET.md.
package layout

import (
	"fmt"
	"strings"

	"github.com/arief-fajri/gowez/internal/style"
	"github.com/arief-fajri/gowez/internal/text"
	"github.com/arief-fajri/gowez/internal/ui"
)

// TextLine is one laid-out line of a text node, in viewport coordinates.
// Paint draws each line at (X, Y+Ascent); lines are the only place text
// geometry lives, so paint never re-measures (layout and pixels cannot
// disagree).
type TextLine struct {
	// Text is the line's (whitespace-collapsed) content.
	Text string
	// X, Y are the line box's top-left corner.
	X, Y float64
	// W, H are the line box dimensions (H = Ascent + Descent).
	W, H float64
	// Ascent is the baseline offset from Y.
	Ascent float64
	// FontSize is the line's font size in logical pixels.
	FontSize float64
}

// Result is the complete layout output for one viewport.
type Result struct {
	// Boxes maps every laid-out node to its border/content boxes.
	// Nodes with display:none are absent.
	Boxes map[ui.NodeID]ui.Geometry
	// Lines maps text nodes to their laid-out lines (absent when the
	// node has no text).
	Lines map[ui.NodeID][]TextLine
}

// layoutCtx carries one layout run's inputs and scratch state.
type layoutCtx struct {
	styles    map[ui.NodeID]style.ComputedStyle
	font      *text.Font
	res       *Result
	intrinsic map[ui.NodeID]float64 // memo for max-content measurement
}

// Layout computes geometry and text lines for roots against a
// viewport.W-wide containing block, returning them keyed by node ID.
//
// Roots are stacked vertically from (0,0) like block children; viewport.H
// is validated but not consumed (no viewport units in the subset).
// Every node must have an entry in styles — call style.Resolve first.
// Failures are explicit: no partial result is returned (invariant I1).
func Layout(roots []*ui.Node, styles map[ui.NodeID]style.ComputedStyle, viewport ui.Size) (*Result, error) {
	if viewport.W <= 0 || viewport.H <= 0 {
		return nil, fmt.Errorf("layout: invalid viewport %gx%g", viewport.W, viewport.H)
	}
	if styles == nil {
		return nil, fmt.Errorf("layout: nil styles (call style.Resolve first)")
	}
	font, err := text.Default()
	if err != nil {
		return nil, fmt.Errorf("layout: font: %w", err)
	}
	ctx := &layoutCtx{
		styles:    styles,
		font:      font,
		res:       &Result{Boxes: map[ui.NodeID]ui.Geometry{}, Lines: map[ui.NodeID][]TextLine{}},
		intrinsic: map[ui.NodeID]float64{},
	}
	for _, root := range roots {
		if root == nil {
			continue
		}
		if err := ctx.validateStyled(root); err != nil {
			return nil, err
		}
		cs := styles[root.ID]
		if cs.Display == style.DisplayNone {
			continue
		}
		if _, _, err := ctx.layoutBox(root, cs.Margin[3], cs.Margin[0], viewport.W, nil, nil); err != nil {
			return nil, err
		}
	}
	return ctx.res, nil
}

// validateStyled walks the tree and rejects nodes without a computed
// style, so a half-resolved tree can never produce half-laid-out boxes.
func (c *layoutCtx) validateStyled(n *ui.Node) error {
	if _, ok := c.styles[n.ID]; !ok {
		return fmt.Errorf("layout: node %d: no computed style (call style.Resolve first)", n.ID)
	}
	for _, child := range n.Children {
		if err := c.validateStyled(child); err != nil {
			return err
		}
	}
	return nil
}

// layoutBox places n's border box with its top-left at (x, y), resolving
// sizes against a containing block of containW. forceW/forceH, when non
// nil, override the resolved *content* sizes (flex items and stretch).
// It returns the resulting border box size.
func (c *layoutCtx) layoutBox(n *ui.Node, x, y, containW float64, forceW, forceH *float64) (float64, float64, error) {
	cs, ok := c.styles[n.ID]
	if !ok {
		return 0, 0, fmt.Errorf("layout: node %d: no computed style", n.ID)
	}
	if cs.Display == style.DisplayNone {
		return 0, 0, nil // caller must skip display:none before positioning
	}
	if cs.Display == style.DisplayInline {
		return 0, 0, fmt.Errorf("layout: node %d: display:inline is not implemented (docs/CSS-SUBSET.md)", n.ID)
	}

	innerW := cs.Padding[1] + cs.Padding[3] + cs.BorderWidth[1] + cs.BorderWidth[3]
	innerH := cs.Padding[0] + cs.Padding[2] + cs.BorderWidth[0] + cs.BorderWidth[2]

	// Content width: forced (flex), else resolved from the property.
	var contentW float64
	switch {
	case forceW != nil:
		contentW = *forceW
	default:
		switch cs.Width.Kind {
		case style.LengthAuto:
			contentW = containW - cs.Margin[1] - cs.Margin[3] - innerW
		case style.LengthPx:
			contentW = cs.Width.Value
		case style.LengthPercent:
			contentW = cs.Width.Value / 100 * containW
		}
	}
	if contentW < 0 {
		contentW = 0
	}

	contentX := x + cs.BorderWidth[3] + cs.Padding[3]
	contentY := y + cs.BorderWidth[0] + cs.Padding[0]

	// Children (or the node's own text) determine the natural height.
	var naturalH float64
	switch {
	case n.Kind == ui.TextNode:
		lines, h, err := c.layoutText(n, cs, contentX, contentY, contentW)
		if err != nil {
			return 0, 0, err
		}
		if len(lines) > 0 {
			c.res.Lines[n.ID] = lines
		}
		naturalH = h
	case cs.Display == style.DisplayFlex:
		h, err := c.layoutFlex(n, cs, contentX, contentY, contentW)
		if err != nil {
			return 0, 0, err
		}
		naturalH = h
	default: // DisplayBlock
		h, err := c.layoutBlockChildren(n, contentX, contentY, contentW)
		if err != nil {
			return 0, 0, err
		}
		naturalH = h
	}

	contentH := naturalH
	switch {
	case forceH != nil:
		contentH = *forceH
	case cs.Height.Kind == style.LengthPx:
		contentH = cs.Height.Value
	}
	if contentH < 0 {
		contentH = 0
	}

	borderW := contentW + innerW
	borderH := contentH + innerH
	c.res.Boxes[n.ID] = ui.Geometry{
		Border:  ui.Box{X: x, Y: y, W: borderW, H: borderH},
		Content: ui.Box{X: contentX, Y: contentY, W: contentW, H: contentH},
	}
	return borderW, borderH, nil
}

// layoutBlockChildren stacks children vertically; margins add up and never
// collapse (docs/CSS-SUBSET.md). It returns the content height consumed.
func (c *layoutCtx) layoutBlockChildren(n *ui.Node, x, y, contentW float64) (float64, error) {
	cursor := y
	for _, child := range n.Children {
		cs, ok := c.styles[child.ID]
		if !ok {
			return 0, fmt.Errorf("layout: node %d: no computed style", child.ID)
		}
		if cs.Display == style.DisplayNone {
			continue
		}
		_, h, err := c.layoutBox(child, x+cs.Margin[3], cursor+cs.Margin[0], contentW, nil, nil)
		if err != nil {
			return 0, err
		}
		cursor += cs.Margin[0] + h + cs.Margin[2]
	}
	return cursor - y, nil
}

// layoutText wraps n's text greedily into lines of at most contentW
// logical pixels and returns them with the total height consumed.
func (c *layoutCtx) layoutText(n *ui.Node, cs style.ComputedStyle, x, y, contentW float64) ([]TextLine, float64, error) {
	words := strings.Fields(n.Text)
	if len(words) == 0 {
		return nil, 0, nil
	}
	font := c.font.WithSize(cs.FontSize)
	widths := make([]float64, len(words))
	for i, w := range words {
		sh, err := text.Shape(w, font)
		if err != nil {
			return nil, 0, fmt.Errorf("layout: node %d: shape %q: %w", n.ID, w, err)
		}
		widths[i] = sh.Width
	}
	space, err := text.Shape(" ", font)
	if err != nil {
		return nil, 0, fmt.Errorf("layout: node %d: shape space: %w", n.ID, err)
	}

	var (
		lines  []TextLine
		cursor int // first word index of the current line
		curW   float64
		total  float64
	)
	flush := func(end int) error {
		lineText := strings.Join(words[cursor:end], " ")
		sh, err := text.Shape(lineText, font)
		if err != nil {
			return fmt.Errorf("layout: node %d: shape %q: %w", n.ID, lineText, err)
		}
		h := sh.Ascent + sh.Descent
		lines = append(lines, TextLine{
			Text: lineText, X: x, Y: y + total,
			W: sh.Width, H: h, Ascent: sh.Ascent, FontSize: cs.FontSize,
		})
		total += h
		return nil
	}

	for i := range words {
		add := widths[i]
		if i > cursor {
			add += space.Width
		}
		if i > cursor && curW+add > contentW && contentW > 0 {
			if err := flush(i); err != nil {
				return nil, 0, err
			}
			cursor = i
			curW = widths[i]
			continue
		}
		curW += add
	}
	if err := flush(len(words)); err != nil {
		return nil, 0, err
	}
	return lines, total, nil
}

// shift moves a node's stored geometry (and text lines) by (dx, dy) —
// used for cross-axis alignment after a box has been measured. The whole
// subtree moves with the node: align-items relocates a laid-out item, and
// leaving descendants (or their text lines) behind would draw children
// outside the shifted parent (regression: TestLayoutFlexAlignShiftSubtree).
func (c *layoutCtx) shift(n *ui.Node, dx, dy float64) {
	if g, ok := c.res.Boxes[n.ID]; ok {
		g.Border.X += dx
		g.Border.Y += dy
		g.Content.X += dx
		g.Content.Y += dy
		c.res.Boxes[n.ID] = g
	}
	if lines, ok := c.res.Lines[n.ID]; ok {
		for i := range lines {
			lines[i].X += dx
			lines[i].Y += dy
		}
		c.res.Lines[n.ID] = lines
	}
	for _, child := range n.Children {
		c.shift(child, dx, dy)
	}
}
