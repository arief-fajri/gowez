package paint

import (
	"fmt"

	"github.com/arief-fajri/gowez/internal/layout"
	"github.com/arief-fajri/gowez/internal/render"
	"github.com/arief-fajri/gowez/internal/style"
	"github.com/arief-fajri/gowez/internal/ui"
)

// Draw emits render commands for the laid-out tree in document order:
// for each node — background fill, borders, text lines, then children.
//
// Coordinates are logical (the layout result); scale converts them to the
// framebuffer's pixel space so geometry stays stable across HiDPI
// displays, exactly as the Milestone 1 scene did. Nodes without geometry
// (display: none) are skipped. A missing style entry is an error, never a
// guessed default.
func Draw(r render.Renderer, roots []*ui.Node, res *layout.Result, styles map[ui.NodeID]style.ComputedStyle, scale float64) error {
	if res == nil {
		return fmt.Errorf("paint: nil layout result")
	}
	if scale <= 0 {
		return fmt.Errorf("paint: invalid scale %g", scale)
	}
	for _, root := range roots {
		if root == nil {
			continue
		}
		if err := drawNode(r, root, res, styles, scale); err != nil {
			return err
		}
	}
	return nil
}

// drawNode paints one node's box (background, border, text lines) and
// recurses into its children in document order.
func drawNode(r render.Renderer, n *ui.Node, res *layout.Result, styles map[ui.NodeID]style.ComputedStyle, scale float64) error {
	g, ok := res.Boxes[n.ID]
	if !ok {
		return nil // display: never laid out → never painted
	}
	cs, ok := styles[n.ID]
	if !ok {
		return fmt.Errorf("paint: node %d: no computed style", n.ID)
	}

	if cs.BackgroundColor&0xff != 0 {
		r.DrawRect(g.Border.X*scale, g.Border.Y*scale, g.Border.W*scale, g.Border.H*scale, toColor(cs.BackgroundColor))
	}
	if anyBorderOpaque(cs) {
		drawBorders(r, g.Border, cs, scale)
	}
	if n.Kind == ui.TextNode {
		for _, line := range res.Lines[n.ID] {
			r.DrawText(line.X*scale, (line.Y+line.Ascent)*scale, line.Text, render.TextOptions{
				FontSize: line.FontSize * scale,
				Color:    toColor(cs.Color),
			})
		}
	}
	for _, child := range n.Children {
		if err := drawNode(r, child, res, styles, scale); err != nil {
			return err
		}
	}
	return nil
}

// anyBorderOpaque reports whether any side has a visible border.
//
// A per-side colour can be transparent while another is not — the whole point of
// `border-bottom: 1px solid red` — so this cannot be a single alpha check.
func anyBorderOpaque(cs style.ComputedStyle) bool {
	for _, c := range cs.BorderColor {
		if c&0xff != 0 {
			return true
		}
	}
	return false
}

// drawBorders fills the four border edges of a box (top, right, bottom,
// left), skipping zero-width sides.
func drawBorders(r render.Renderer, b ui.Box, cs style.ComputedStyle, scale float64) {
	t, rt, bo, le := cs.BorderWidth[0], cs.BorderWidth[1], cs.BorderWidth[2], cs.BorderWidth[3]
	x, y, w, h := b.X, b.Y, b.W, b.H

	if t > 0 && cs.BorderColor[0]&0xff != 0 {
		r.DrawRect(x*scale, y*scale, w*scale, t*scale, toColor(cs.BorderColor[0]))
	}
	if bo > 0 && cs.BorderColor[2]&0xff != 0 {
		r.DrawRect(x*scale, (y+h-bo)*scale, w*scale, bo*scale, toColor(cs.BorderColor[2]))
	}
	if le > 0 && cs.BorderColor[3]&0xff != 0 {
		r.DrawRect(x*scale, (y+t)*scale, le*scale, (h-t-bo)*scale, toColor(cs.BorderColor[3]))
	}
	if rt > 0 && cs.BorderColor[1]&0xff != 0 {
		r.DrawRect((x+w-rt)*scale, (y+t)*scale, rt*scale, (h-t-bo)*scale, toColor(cs.BorderColor[1]))
	}
}

// toColor converts a 0xRRGGBBAA style color to a render.Color.
func toColor(c uint32) render.Color {
	return render.Color{
		R: float32(c>>24&0xff) / 255,
		G: float32(c>>16&0xff) / 255,
		B: float32(c>>8&0xff) / 255,
		A: float32(c&0xff) / 255,
	}
}
