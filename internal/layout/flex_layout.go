package layout

import (
	"fmt"

	"github.com/arief-fajri/gowez/internal/style"
	"github.com/arief-fajri/gowez/internal/ui"
)

// flexItem is one child's bookkeeping during a flex pass.
type flexItem struct {
	child *ui.Node
	cs    style.ComputedStyle

	// mainC is the main-axis *content* size after grow/shrink; innerM is
	// the main-axis padding+border, so the border box is mainC+innerM.
	mainC, innerM float64
	// m0/m1 are main-axis margins, c0/c1 cross-axis margins (start/end).
	m0, m1, c0, c1 float64
	// bw/bh are the border box after layout.
	bw, bh float64
	// x, y are the placement used for layout.
	x, y float64
	// forceW/forceH are the content-size overrides passed to layoutBox.
	forceW, forceH *float64
}

// layoutFlex dispatches to the row or column algorithm. contentX/contentY
// are the container's content-box origin; the return value is the natural
// content height.
func (c *layoutCtx) layoutFlex(n *ui.Node, cs style.ComputedStyle, contentX, contentY, contentW float64) (float64, error) {
	items, err := c.collectFlexItems(n, cs, contentW)
	if err != nil || len(items) == 0 {
		return 0, err
	}
	if cs.FlexDirection == style.Row {
		return c.layoutFlexRow(cs, items, contentX, contentY, contentW)
	}
	return c.layoutFlexColumn(cs, items, contentX, contentY, contentW)
}

// collectFlexItems gathers visible children with their margins and base
// main sizes (before grow/shrink).
func (c *layoutCtx) collectFlexItems(n *ui.Node, cs style.ComputedStyle, contentW float64) ([]*flexItem, error) {
	var items []*flexItem
	row := cs.FlexDirection == style.Row
	for _, child := range n.Children {
		childCS, ok := c.styles[child.ID]
		if !ok {
			return nil, fmt.Errorf("layout: node %d: no computed style", child.ID)
		}
		if childCS.Display == style.DisplayNone {
			continue
		}
		it := &flexItem{
			child: child,
			cs:    childCS,
		}
		if row {
			it.m0, it.m1 = childCS.Margin[3], childCS.Margin[1]
			it.c0, it.c1 = childCS.Margin[0], childCS.Margin[2]
			it.innerM = childCS.Padding[1] + childCS.Padding[3] + childCS.BorderWidth[1] + childCS.BorderWidth[3]
			switch childCS.Width.Kind {
			case style.LengthPx:
				it.mainC = childCS.Width.Value
			case style.LengthPercent:
				it.mainC = childCS.Width.Value / 100 * contentW
			default:
				w, err := c.intrinsicContentWidth(child)
				if err != nil {
					return nil, err
				}
				it.mainC = w
			}
		} else {
			it.m0, it.m1 = childCS.Margin[0], childCS.Margin[2]
			it.c0, it.c1 = childCS.Margin[3], childCS.Margin[1]
			it.innerM = childCS.Padding[0] + childCS.Padding[2] + childCS.BorderWidth[0] + childCS.BorderWidth[2]
			if childCS.Height.Kind == style.LengthPx {
				it.mainC = childCS.Height.Value
			}
			// Auto main size: resolved from the first layout pass.
		}
		items = append(items, it)
	}
	return items, nil
}

// adjustMain distributes free space on the main axis: positive free space
// by flex-grow, negative by flex-shrink scaled with base size. Items never
// shrink below zero content size.
func adjustMain(items []*flexItem, free float64) {
	if free > 0 {
		var sum float64
		for _, it := range items {
			if it.cs.FlexGrow > 0 {
				sum += it.cs.FlexGrow
			}
		}
		if sum > 0 {
			for _, it := range items {
				if it.cs.FlexGrow > 0 {
					it.mainC += free * it.cs.FlexGrow / sum
				}
			}
		}
		return
	}
	if free < 0 {
		var scaled float64
		for _, it := range items {
			scaled += it.mainC * it.cs.FlexShrink
		}
		if scaled <= 0 {
			return
		}
		for _, it := range items {
			share := it.mainC * it.cs.FlexShrink / scaled
			it.mainC += free * share // free is negative
			if it.mainC < 0 {
				it.mainC = 0
			}
		}
	}
}

// justifyOffset returns the leading free-space offset for justify-content.
func justifyOffset(kind style.JustifyContent, free float64, k int) float64 {
	if free <= 0 {
		return 0
	}
	switch kind {
	case style.JustifyCenter:
		return free / 2
	case style.JustifyEnd:
		return free
	}
	return 0
}

// justifyExtra returns the extra per-gap space for space-between.
func justifyExtra(kind style.JustifyContent, free float64, k int) float64 {
	if kind == style.JustifySpaceBetween && free > 0 && k > 1 {
		return free / float64(k-1)
	}
	return 0
}

// outerMain returns the item's main-axis border box size.
func (it *flexItem) outerMain() float64 { return it.mainC + it.innerM }

// layoutFlexRow lays out row children: main axis horizontal, cross axis
// vertical. x positions are final; y may need a second pass for
// align-items (stretch re-layouts, center/end shift).
func (c *layoutCtx) layoutFlexRow(cs style.ComputedStyle, items []*flexItem, contentX, contentY, contentW float64) (float64, error) {
	k := len(items)
	gap := cs.Gap

	total := gap * float64(k-1)
	for _, it := range items {
		total += it.outerMain() + it.m0 + it.m1
	}
	free := contentW - total
	adjustMain(items, free)

	// Recompute free space after grow/shrink for justify-content.
	after := contentW
	for _, it := range items {
		after -= it.outerMain() + it.m0 + it.m1
	}
	after -= gap * float64(k-1)

	offset := justifyOffset(cs.JustifyContent, after, k)
	extra := justifyExtra(cs.JustifyContent, after, k)

	// Main-axis placement is final here; cross placement follows.
	cursor := contentX + offset
	for i, it := range items {
		if i > 0 {
			cursor += gap + extra
		}
		it.x = cursor + it.m0
		it.y = contentY + it.c0
		it.forceW = floatPtr(it.mainC)
		if _, _, err := c.layoutBox(it.child, it.x, it.y, contentW, it.forceW, nil); err != nil {
			return 0, err
		}
		cursor += it.m0 + it.outerMain() + it.m1
	}

	// Container cross size: explicit height, else the tallest outer box.
	contentH := cs.Height.Value // zero-value Length is auto → Value 0
	explicit := cs.Height.Kind == style.LengthPx
	if !explicit {
		var maxCross float64
		for _, it := range items {
			if o := it.c0 + it.bh + it.c1; o > maxCross {
				maxCross = o
			}
		}
		contentH = maxCross
	}

	for _, it := range items {
		// bh must reflect the layout just stored.
		g := c.res.Boxes[it.child.ID]
		it.bh = g.Border.H

		if cs.AlignItems == style.AlignStretch && it.cs.Height.Kind == style.LengthAuto {
			target := contentH - it.c0 - it.c1
			forceH := target - (it.cs.Padding[0] + it.cs.Padding[2] + it.cs.BorderWidth[0] + it.cs.BorderWidth[2])
			if forceH < 0 {
				forceH = 0
			}
			if _, _, err := c.layoutBox(it.child, it.x, contentY+it.c0, contentW, it.forceW, floatPtr(forceH)); err != nil {
				return 0, err
			}
			continue
		}
		var y float64
		switch cs.AlignItems {
		case style.AlignCenter:
			y = contentY + it.c0 + (contentH-it.c0-it.c1-it.bh)/2
		case style.AlignEnd:
			y = contentY + contentH - it.c1 - it.bh
		default: // AlignStart (stretch items with explicit height land here too)
			y = contentY + it.c0
		}
		c.shift(it.child, 0, y-it.y)
		it.y = y
	}
	return contentH, nil
}

// layoutFlexColumn lays out column children: main axis vertical, cross
// axis horizontal. An explicit container height triggers a second pass
// that grows/shrinks items; an auto height takes the natural stack.
func (c *layoutCtx) layoutFlexColumn(cs style.ComputedStyle, items []*flexItem, contentX, contentY, contentW float64) (float64, error) {
	k := len(items)
	gap := cs.Gap
	stretch := cs.AlignItems == style.AlignStretch

	place := func(offset, extraGap float64, adjusted bool) (float64, error) {
		cursor := contentY + offset
		var sumOuter float64
		for i, it := range items {
			if i > 0 {
				cursor += gap + extraGap
			}
			it.x = contentX + it.c0
			it.y = cursor + it.m0
			it.forceW = nil
			if it.cs.Width.Kind != style.LengthAuto {
				w := it.cs.Width.Value
				if it.cs.Width.Kind == style.LengthPercent {
					w = it.cs.Width.Value / 100 * contentW
				}
				it.forceW = floatPtr(w)
			} else if !stretch {
				w, err := c.intrinsicContentWidth(it.child)
				if err != nil {
					return 0, err
				}
				it.forceW = floatPtr(w)
			}
			it.forceH = nil
			if adjusted {
				it.forceH = floatPtr(it.mainC)
			} else if it.cs.Height.Kind == style.LengthPx {
				it.forceH = floatPtr(it.cs.Height.Value)
			}
			if _, _, err := c.layoutBox(it.child, it.x, it.y, contentW, it.forceW, it.forceH); err != nil {
				return 0, err
			}
			g := c.res.Boxes[it.child.ID]
			it.bw, it.bh = g.Border.W, g.Border.H
			sumOuter += it.m0 + it.bh + it.m1
			cursor += it.m0 + it.bh + it.m1
		}
		return sumOuter + gap*float64(k-1), nil
	}

	explicit := cs.Height.Kind == style.LengthPx
	if !explicit {
		contentH, err := place(0, 0, false)
		if err != nil {
			return 0, err
		}
		if err := c.alignColumnCross(cs, items, contentX, contentW); err != nil {
			return 0, err
		}
		return contentH, err
	}

	// First pass at natural/explicit sizes, then distribute free space.
	natural, err := place(0, 0, false)
	if err != nil {
		return 0, err
	}
	// Auto-height items learned their real base size in the first pass;
	// grow/shrink needs it before the second pass forces the result.
	for _, it := range items {
		if it.cs.Height.Kind != style.LengthPx {
			it.mainC = it.bh - it.innerM
		}
	}
	free := cs.Height.Value - natural
	adjustMain(items, free)

	after := cs.Height.Value
	for _, it := range items {
		after -= it.outerMain() + it.m0 + it.m1
	}
	after -= gap * float64(k-1)
	offset := justifyOffset(cs.JustifyContent, after, k)
	extra := justifyExtra(cs.JustifyContent, after, k)

	contentH, err := place(offset, extra, true)
	if err != nil {
		return 0, err
	}
	if err := c.alignColumnCross(cs, items, contentX, contentW); err != nil {
		return 0, err
	}
	return contentH, nil
}

// alignColumnCross applies align-items on the horizontal cross axis of a
// column container (start is already the layout position).
func (c *layoutCtx) alignColumnCross(cs style.ComputedStyle, items []*flexItem, contentX, contentW float64) error {
	if cs.AlignItems == style.AlignStart || cs.AlignItems == style.AlignStretch {
		return nil
	}
	for _, it := range items {
		var dx float64
		switch cs.AlignItems {
		case style.AlignCenter:
			dx = (contentW - it.c0 - it.c1 - it.bw) / 2
		case style.AlignEnd:
			dx = contentW - it.c1 - it.bw
		}
		c.shift(it.child, dx, 0)
		it.x += dx
	}
	return nil
}

// floatPtr returns a pointer to v (for forceW/forceH arguments).
func floatPtr(v float64) *float64 { return &v }
