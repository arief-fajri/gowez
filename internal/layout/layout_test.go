package layout

import (
	"reflect"
	"strings"
	"testing"

	"github.com/arief-fajri/gowez/internal/style"
	"github.com/arief-fajri/gowez/internal/ui"
)

const eps = 1e-9

// el creates an element attached under parent (parent == nil → root).
func el(t *testing.T, tree *ui.Tree, parent *ui.Node, tag, attrs string) *ui.Node {
	t.Helper()
	n := tree.CreateElement(tag)
	for _, kv := range strings.Fields(attrs) {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			n.SetAttribute(parts[0], strings.Trim(parts[1], `"'`))
		}
	}
	if parent == nil {
		if err := tree.AppendRoot(n); err != nil {
			t.Fatal(err)
		}
	} else if err := tree.Append(parent, n); err != nil {
		t.Fatal(err)
	}
	return n
}

// txt creates a text node under parent.
func txt(t *testing.T, tree *ui.Tree, parent *ui.Node, s string) *ui.Node {
	t.Helper()
	n := tree.CreateText(s)
	if err := tree.Append(parent, n); err != nil {
		t.Fatal(err)
	}
	return n
}

// resolve parses css and resolves styles for the tree.
func resolve(t *testing.T, tree *ui.Tree, css string) map[ui.NodeID]style.ComputedStyle {
	t.Helper()
	sheet, err := style.Parse(css)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	styles, err := style.Resolve(tree.Roots(), sheet)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return styles
}

// wantBox asserts a node's border box within eps.
func wantBox(t *testing.T, res *Result, id ui.NodeID, x, y, w, h float64) {
	t.Helper()
	g, ok := res.Boxes[id]
	if !ok {
		t.Fatalf("node %d: no geometry", id)
	}
	b := g.Border
	if diff := b.X - x; diff > eps || diff < -eps {
		t.Errorf("node %d X = %g, want %g", id, b.X, x)
	}
	if diff := b.Y - y; diff > eps || diff < -eps {
		t.Errorf("node %d Y = %g, want %g", id, b.Y, y)
	}
	if diff := b.W - w; diff > eps || diff < -eps {
		t.Errorf("node %d W = %g, want %g", id, b.W, w)
	}
	if diff := b.H - h; diff > eps || diff < -eps {
		t.Errorf("node %d H = %g, want %g", id, b.H, h)
	}
}

func TestLayoutBlockStacking(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := el(t, tree, nil, "root", "")
	a := el(t, tree, root, "a", "")
	b := el(t, tree, root, "b", "")

	styles := resolve(t, tree, `
root { padding: 10px; }
a { height: 50px; }
b { height: 30px; margin: 5px; }
`)
	res, err := Layout(tree.Roots(), styles, ui.Size{W: 800, H: 600})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	// Root: content width = 800 - 20 padding; children stack inside.
	wantBox(t, res, root.ID, 0, 0, 800, 110)
	wantBox(t, res, a.ID, 10, 10, 780, 50)
	// b sits after a plus its 5px top margin; margin: 5px also indents it
	// and narrows the auto width (margins never collapse).
	wantBox(t, res, b.ID, 15, 65, 770, 30)
	// Content box of root starts inside padding.
	if got := res.Boxes[root.ID].Content; got.X != 10 || got.Y != 10 || got.W != 780 {
		t.Errorf("root content = %+v, want {10 10 780 90}", got)
	}
}

func TestLayoutWidths(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := el(t, tree, nil, "root", "")
	pct := el(t, tree, root, "pct", "")
	fixed := el(t, tree, root, "fixed", "")

	styles := resolve(t, tree, `
pct { width: 50%; height: 10px; }
fixed { width: 120px; height: 10px; margin-left: 8px; }
`)
	res, err := Layout(tree.Roots(), styles, ui.Size{W: 400, H: 300})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	wantBox(t, res, root.ID, 0, 0, 400, 20)
	wantBox(t, res, pct.ID, 0, 0, 200, 10)    // 50% of 400
	wantBox(t, res, fixed.ID, 8, 10, 120, 10) // explicit + margin
}

func TestLayoutDisplayNone(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := el(t, tree, nil, "root", "")
	hidden := el(t, tree, root, "hidden", "")
	after := el(t, tree, root, "after", "")

	styles := resolve(t, tree, `
root { }
hidden { display: none; height: 40px; }
after { height: 20px; }
`)
	res, err := Layout(tree.Roots(), styles, ui.Size{W: 300, H: 200})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	if _, ok := res.Boxes[hidden.ID]; ok {
		t.Fatal("display:none node has geometry")
	}
	// The hidden node contributes no height and no margin.
	wantBox(t, res, after.ID, 0, 0, 300, 20)
	wantBox(t, res, root.ID, 0, 0, 300, 20)
}

func TestLayoutFlexRowSpaceBetween(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := el(t, tree, nil, "row", "")
	a := el(t, tree, root, "i", "")
	b := el(t, tree, root, "i", "")
	c := el(t, tree, root, "i", "")

	styles := resolve(t, tree, `
row { display: flex; flex-direction: row; justify-content: space-between; height: 40px; }
i { width: 100px; height: 40px; }
`)
	res, err := Layout(tree.Roots(), styles, ui.Size{W: 800, H: 600})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	// free = 800 - 300 = 500 → 250 between items.
	wantBox(t, res, a.ID, 0, 0, 100, 40)
	wantBox(t, res, b.ID, 350, 0, 100, 40)
	wantBox(t, res, c.ID, 700, 0, 100, 40)
}

func TestLayoutFlexRowGapAndGrow(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := el(t, tree, nil, "row", "")
	a := el(t, tree, root, "fixed", "")
	b := el(t, tree, root, "grow", "")

	styles := resolve(t, tree, `
row { display: flex; gap: 10px; height: 30px; }
fixed { width: 100px; height: 30px; }
grow { width: 100px; flex-grow: 1; height: 30px; }
`)
	res, err := Layout(tree.Roots(), styles, ui.Size{W: 400, H: 300})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	// free = 400 - 10 - 200 = 190 → the grow item absorbs it.
	wantBox(t, res, a.ID, 0, 0, 100, 30)
	wantBox(t, res, b.ID, 110, 0, 290, 30)
}

func TestLayoutFlexAlign(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := el(t, tree, nil, "row", "")
	stretched := el(t, tree, root, "s", "")
	centered := el(t, tree, root, "c", "")

	styles := resolve(t, tree, `
row { display: flex; align-items: center; height: 200px; }
s { width: 50px; align-items: stretch; }
c { width: 50px; height: 40px; }
`)
	// align-items:center on the container; item s has auto height so it
	// centers at natural size (0 height → stays at top).
	res, err := Layout(tree.Roots(), styles, ui.Size{W: 400, H: 300})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	wantBox(t, res, centered.ID, 50, 80, 50, 40) // (200-40)/2 = 80

	// Now stretch: default align-items stretches the auto-height item.
	styles = resolve(t, tree, `
row { display: flex; height: 200px; }
s { width: 50px; }
c { width: 50px; height: 40px; }
`)
	res, err = Layout(tree.Roots(), styles, ui.Size{W: 400, H: 300})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	wantBox(t, res, stretched.ID, 0, 0, 50, 200)
	wantBox(t, res, centered.ID, 50, 0, 50, 40)
}

// TestLayoutFlexAlignShiftSubtree pins that align-items center moves the
// item's WHOLE subtree: children boxes and text lines must travel with the
// shifted parent. shift() used to relocate only the item itself, so a
// button's label stayed at the pre-shift position — the text rendered
// ~5.5px above the button's center (pixel evidence: golden ui.png).
func TestLayoutFlexAlignShiftSubtree(t *testing.T) {
	t.Parallel()

	t.Run("row-center", func(t *testing.T) {
		t.Parallel()
		tree := ui.NewTree()
		root := el(t, tree, nil, "row", "")
		item := el(t, tree, root, "b", "")
		el(t, tree, item, "c", "") // 30px block child, stacks first
		label := txt(t, tree, item, "Apply")

		styles := resolve(t, tree, `
row { display: flex; align-items: center; height: 200px; width: 400px; }
b { width: 100px; padding: 10px; border-width: 2px; }
c { height: 30px; }
`)
		res, err := Layout(tree.Roots(), styles, ui.Size{W: 400, H: 300})
		if err != nil {
			t.Fatalf("Layout: %v", err)
		}
		bb := res.Boxes[item.ID]
		// The item itself is vertically centered in the 200px row.
		if got := bb.Border.Y + bb.Border.H/2; got-100 > eps || 100-got > eps {
			t.Errorf("item center-y = %g, want 100 (align-items: center)", got)
		}
		// Descendants follow the shift: child box sits on the item's
		// content origin, not at the pre-shift (top-aligned) position.
		cb := res.Boxes[item.Children[0].ID]
		if diff := cb.Border.Y - bb.Content.Y; diff > eps || diff < -eps {
			t.Errorf("child Y = %g, want item content Y %g (subtree must shift with item)", cb.Border.Y, bb.Content.Y)
		}
		if diff := cb.Border.X - bb.Content.X; diff > eps || diff < -eps {
			t.Errorf("child X = %g, want item content X %g", cb.Border.X, bb.Content.X)
		}
		// The label stacks after the 30px child — on the same shifted origin.
		lines := res.Lines[label.ID]
		if len(lines) != 1 {
			t.Fatalf("label lines = %d, want 1", len(lines))
		}
		if diff := lines[0].Y - (bb.Content.Y + 30); diff > eps || diff < -eps {
			t.Errorf("label line Y = %g, want %g (content Y + 30px child) — text left behind by align shift",
				lines[0].Y, bb.Content.Y+30)
		}
		if diff := lines[0].X - bb.Content.X; diff > eps || diff < -eps {
			t.Errorf("label line X = %g, want content X %g", lines[0].X, bb.Content.X)
		}
		lb := res.Boxes[label.ID]
		if diff := lb.Border.Y - (bb.Content.Y + 30); diff > eps || diff < -eps {
			t.Errorf("label box Y = %g, want %g", lb.Border.Y, bb.Content.Y+30)
		}
	})

	t.Run("column-center", func(t *testing.T) {
		t.Parallel()
		tree := ui.NewTree()
		root := el(t, tree, nil, "col", "")
		item := el(t, tree, root, "b", "")
		el(t, tree, item, "c", "")
		label := txt(t, tree, item, "Apply")

		styles := resolve(t, tree, `
col { display: flex; flex-direction: column; align-items: center; width: 200px; }
b { padding: 10px; }
c { height: 30px; width: 40px; }
`)
		res, err := Layout(tree.Roots(), styles, ui.Size{W: 400, H: 300})
		if err != nil {
			t.Fatalf("Layout: %v", err)
		}
		bb := res.Boxes[item.ID]
		// Centered on the horizontal cross axis of the 200px column.
		if got := bb.Border.X + bb.Border.W/2; got-100 > eps || 100-got > eps {
			t.Errorf("item center-x = %g, want 100 (align-items: center)", got)
		}
		cb := res.Boxes[item.Children[0].ID]
		if diff := cb.Border.X - bb.Content.X; diff > eps || diff < -eps {
			t.Errorf("child X = %g, want item content X %g (subtree must shift with item)", cb.Border.X, bb.Content.X)
		}
		lines := res.Lines[label.ID]
		if len(lines) != 1 {
			t.Fatalf("label lines = %d, want 1", len(lines))
		}
		if diff := lines[0].X - bb.Content.X; diff > eps || diff < -eps {
			t.Errorf("label line X = %g, want content X %g — text left behind by align shift",
				lines[0].X, bb.Content.X)
		}
		if diff := lines[0].Y - (bb.Content.Y + 30); diff > eps || diff < -eps {
			t.Errorf("label line Y = %g, want %g", lines[0].Y, bb.Content.Y+30)
		}
	})
}

func TestLayoutFlexColumn(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := el(t, tree, nil, "col", "")
	a := el(t, tree, root, "a", "")
	b := el(t, tree, root, "b", "")

	styles := resolve(t, tree, `
col { display: flex; flex-direction: column; gap: 10px; width: 200px; }
a { height: 30px; width: 50%; }
b { height: 40px; }
`)
	res, err := Layout(tree.Roots(), styles, ui.Size{W: 400, H: 300})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	wantBox(t, res, a.ID, 0, 0, 100, 30)  // 50% of 200
	wantBox(t, res, b.ID, 0, 40, 200, 40) // stretched to container width
	wantBox(t, res, root.ID, 0, 0, 200, 80)
}

func TestLayoutFlexColumnExplicitHeightJustify(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := el(t, tree, nil, "col", "")
	a := el(t, tree, root, "a", "")
	b := el(t, tree, root, "b", "")

	styles := resolve(t, tree, `
col { display: flex; flex-direction: column; height: 200px; justify-content: center; width: 100px; }
a { height: 30px; }
b { height: 40px; }
`)
	res, err := Layout(tree.Roots(), styles, ui.Size{W: 400, H: 300})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	// free = 200 - 70 = 130 → center offset 65.
	wantBox(t, res, a.ID, 0, 65, 100, 30)
	wantBox(t, res, b.ID, 0, 95, 100, 40)
	wantBox(t, res, root.ID, 0, 0, 100, 200)
}

func TestLayoutTextWrap(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := el(t, tree, nil, "root", "")
	text := txt(t, tree, root, "go wez runtime layout wraps this sentence")

	styles := resolve(t, tree, `
root { width: 160px; padding: 8px; }
text { font-size: 16px; }
`)
	res, err := Layout(tree.Roots(), styles, ui.Size{W: 400, H: 300})
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	lines := res.Lines[text.ID]
	if len(lines) < 2 {
		t.Fatalf("lines = %d (%+v), want wrapping in a 160px content box", len(lines), lines)
	}
	var total float64
	for i, ln := range lines {
		if ln.X != 8 {
			t.Errorf("line %d X = %g, want 8 (content origin)", i, ln.X)
		}
		if ln.W > 160+eps && len(strings.Fields(ln.Text)) > 1 {
			t.Errorf("line %d %q width %g exceeds content width 160", i, ln.Text, ln.W)
		}
		if ln.Ascent <= 0 || ln.H <= ln.Ascent {
			t.Errorf("line %d metrics = ascent %g h %g", i, ln.Ascent, ln.H)
		}
		total += ln.H
	}
	// Text node border height equals the sum of its line boxes.
	if g := res.Boxes[text.ID]; g.Content.H-total > eps || total-g.Content.H > eps {
		t.Errorf("text height = %g, want %g", g.Content.H, total)
	}
	// Root height = text height + padding.
	if g := res.Boxes[root.ID]; g.Border.H < total+16-eps {
		t.Errorf("root height = %g, want >= %g", g.Border.H, total+16)
	}
}

func TestLayoutDeterministic(t *testing.T) {
	t.Parallel()
	build := func() (*ui.Tree, map[ui.NodeID]style.ComputedStyle) {
		tree := ui.NewTree()
		root := el(t, tree, nil, "root", "")
		row := el(t, tree, root, "row", "")
		el(t, tree, row, "i", "")
		el(t, tree, row, "i", "")
		text := txt(t, tree, root, "identical input produces identical geometry")
		styles := resolve(t, tree, `
root { padding: 4px; }
row { display: flex; gap: 6px; height: 40px; }
i { width: 30%; height: 20px; align-items: start; }
text { font-size: 14px; }
`)
		_ = text
		return tree, styles
	}
	treeA, stylesA := build()
	treeB, stylesB := build()
	resA, err := Layout(treeA.Roots(), stylesA, ui.Size{W: 640, H: 480})
	if err != nil {
		t.Fatalf("Layout A: %v", err)
	}
	resB, err := Layout(treeB.Roots(), stylesB, ui.Size{W: 640, H: 480})
	if err != nil {
		t.Fatalf("Layout B: %v", err)
	}
	if !reflect.DeepEqual(resA.Boxes, resB.Boxes) {
		t.Fatal("two runs of identical input produced different boxes")
	}
	if !reflect.DeepEqual(resA.Lines, resB.Lines) {
		t.Fatal("two runs of identical input produced different text lines")
	}
}

func TestLayoutErrors(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := el(t, tree, nil, "root", "")

	if _, err := Layout(tree.Roots(), nil, ui.Size{W: 100, H: 100}); err == nil {
		t.Fatal("nil styles must fail")
	}
	if _, err := Layout(tree.Roots(), map[ui.NodeID]style.ComputedStyle{}, ui.Size{W: 100, H: 100}); err == nil {
		t.Fatal("missing style entries must fail")
	}
	if _, err := Layout(tree.Roots(), map[ui.NodeID]style.ComputedStyle{root.ID: style.Initial()}, ui.Size{W: 0, H: 100}); err == nil {
		t.Fatal("invalid viewport must fail")
	}
}
