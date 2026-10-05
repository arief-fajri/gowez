package layout

import (
	"fmt"
	"strings"

	"github.com/arief-fajri/gowez/internal/style"
	"github.com/arief-fajri/gowez/internal/text"
	"github.com/arief-fajri/gowez/internal/ui"
)

// intrinsicContentWidth measures a node's max-content content width: the
// width of its text as a single unwrapped line, or the width its children
// need without wrapping. Percentage sizes resolve to zero during
// measurement (there is no containing block yet); the caller substitutes
// the real percentage once the container is known.
//
// Results are memoized per layout run — measurement is context-free.
func (c *layoutCtx) intrinsicContentWidth(n *ui.Node) (float64, error) {
	if v, ok := c.intrinsic[n.ID]; ok {
		return v, nil
	}
	cs, ok := c.styles[n.ID]
	if !ok {
		return 0, fmt.Errorf("layout: node %d: no computed style", n.ID)
	}
	if cs.Display == style.DisplayNone {
		return 0, nil
	}

	var content float64
	switch {
	case n.Kind == ui.TextNode:
		collapsed := strings.Join(strings.Fields(n.Text), " ")
		if collapsed == "" {
			content = 0
			break
		}
		sh, err := text.Shape(collapsed, c.font.WithSize(cs.FontSize))
		if err != nil {
			return 0, fmt.Errorf("layout: node %d: shape %q: %w", n.ID, collapsed, err)
		}
		content = sh.Width
	case cs.Display == style.DisplayFlex && cs.FlexDirection == style.Row:
		var sum float64
		var count int
		for _, child := range n.Children {
			childCS, ok := c.styles[child.ID]
			if !ok {
				return 0, fmt.Errorf("layout: node %d: no computed style", child.ID)
			}
			if childCS.Display == style.DisplayNone {
				continue
			}
			w, err := c.intrinsicBorderWidth(child)
			if err != nil {
				return 0, err
			}
			sum += w + childCS.Margin[3] + childCS.Margin[1]
			count++
		}
		content = sum
		if count > 1 {
			content += cs.Gap * float64(count-1)
		}
	default: // block stack or column flex: the widest child wins
		var widest float64
		for _, child := range n.Children {
			childCS, ok := c.styles[child.ID]
			if !ok {
				return 0, fmt.Errorf("layout: node %d: no computed style", child.ID)
			}
			if childCS.Display == style.DisplayNone {
				continue
			}
			w, err := c.intrinsicBorderWidth(child)
			if err != nil {
				return 0, err
			}
			if outer := w + childCS.Margin[3] + childCS.Margin[1]; outer > widest {
				widest = outer
			}
		}
		content = widest
	}

	c.intrinsic[n.ID] = content
	return content, nil
}

// intrinsicBorderWidth measures a node's max-content border-box width:
// explicit width short-circuits the children; percentages measure as zero
// (resolved later against the real containing block).
func (c *layoutCtx) intrinsicBorderWidth(n *ui.Node) (float64, error) {
	cs, ok := c.styles[n.ID]
	if !ok {
		return 0, fmt.Errorf("layout: node %d: no computed style", n.ID)
	}
	inner := cs.Padding[1] + cs.Padding[3] + cs.BorderWidth[1] + cs.BorderWidth[3]
	switch cs.Width.Kind {
	case style.LengthPx:
		return cs.Width.Value + inner, nil
	case style.LengthPercent:
		return inner, nil
	}
	content, err := c.intrinsicContentWidth(n)
	if err != nil {
		return 0, err
	}
	return content + inner, nil
}
