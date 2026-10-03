package text

import (
	"image/color"
	"image/draw"

	"github.com/go-text/render"
)

// Draw rasterizes the shaped run into dst.
//
// x is the left edge of the run and baselineY is the text baseline, both
// in dst pixel coordinates. A nil or empty run draws nothing.
func (sh *Shaped) Draw(dst draw.Image, x, baselineY int, col color.Color) {
	if sh == nil || len(sh.run.Glyphs) == 0 {
		return
	}
	r := &render.Renderer{
		FontSize: float32(sh.sizePx),
		PixScale: 1,
		Color:    col,
	}
	r.DrawShapedRunAt(sh.run, dst, x, baselineY)
}
