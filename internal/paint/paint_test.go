package paint

import (
	"testing"

	"github.com/arief-fajri/gowez/internal/layout"
	"github.com/arief-fajri/gowez/internal/render"
	"github.com/arief-fajri/gowez/internal/render/backend/software"
	"github.com/arief-fajri/gowez/internal/style"
	"github.com/arief-fajri/gowez/internal/ui"
)

// scene builds a card with a text child and a hidden sibling:
//
//	card (200×60, bg, 2px border, 4px padding)
//	├── "Hello" (12px white)
//	└── hidden (display:none, bg — must produce no commands)
func scene(t *testing.T) (*ui.Tree, map[ui.NodeID]style.ComputedStyle, *layout.Result) {
	t.Helper()
	tree := ui.NewTree()
	card := tree.CreateElement("card")
	text := tree.CreateText("Hello")
	hidden := tree.CreateElement("hidden")
	if err := tree.AppendRoot(card); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]*ui.Node{{card, text}, {card, hidden}} {
		if err := tree.Append(pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	sheet, err := style.Parse(`
card {
	width: 200px; height: 60px; padding: 4px;
	border-width: 2px; border-color: #cccccc;
	background-color: #1e2028;
}
text { color: #ffffff; font-size: 12px; }
hidden { display: none; background-color: #ff0000; }
`)
	if err != nil {
		t.Fatal(err)
	}
	styles, err := style.Resolve(tree.Roots(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	res, err := layout.Layout(tree.Roots(), styles, ui.Size{W: 400, H: 300})
	if err != nil {
		t.Fatal(err)
	}
	return tree, styles, res
}

func TestDrawCommandStream(t *testing.T) {
	t.Parallel()
	tree, styles, res := scene(t)

	r := software.New()
	r.BeginFrame(400, 300)
	if err := Draw(r, tree.Roots(), res, styles, 1); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	r.EndFrame()

	cmds := r.Frame().Commands
	// background + 4 border edges + one text run; hidden node silent.
	if len(cmds) != 6 {
		t.Fatalf("commands = %d, want 6: %+v", len(cmds), cmds)
	}
	bg, ok := cmds[0].(render.DrawRect)
	if !ok {
		t.Fatalf("cmd[0] = %T, want DrawRect background", cmds[0])
	}
	// Content-box sizing: 200 content + 2*4 padding + 2*2 border.
	if bg.X != 0 || bg.Y != 0 || bg.W != 212 || bg.H != 72 {
		t.Errorf("background = %+v, want 0,0 212x72", bg)
	}
	if bg.Color != (render.Color{R: 0x1e / 255.0, G: 0x20 / 255.0, B: 0x28 / 255.0, A: 1}) {
		t.Errorf("background color = %+v", bg.Color)
	}
	// Border edges: top, bottom, left, right (2px, #ccc).
	wantEdges := []ui.Box{
		{X: 0, Y: 0, W: 212, H: 2},
		{X: 0, Y: 70, W: 212, H: 2},
		{X: 0, Y: 2, W: 2, H: 68},
		{X: 210, Y: 2, W: 2, H: 68},
	}
	for i, want := range wantEdges {
		rect, ok := cmds[i+1].(render.DrawRect)
		if !ok {
			t.Fatalf("cmd[%d] = %T, want DrawRect border", i+1, cmds[i+1])
		}
		got := ui.Box{X: rect.X, Y: rect.Y, W: rect.W, H: rect.H}
		if got != want {
			t.Errorf("border %d = %+v, want %+v", i, got, want)
		}
	}
	// Text run: pen at content origin (2 border + 4 padding), baseline
	// below the line top, font size from the computed style.
	txt, ok := cmds[5].(render.DrawText)
	if !ok {
		t.Fatalf("cmd[5] = %T, want DrawText", cmds[5])
	}
	if txt.Text != "Hello" {
		t.Errorf("text = %q, want Hello", txt.Text)
	}
	if txt.Options.FontSize != 12 {
		t.Errorf("FontSize = %g, want 12", txt.Options.FontSize)
	}
	if txt.X != 6 {
		t.Errorf("pen X = %g, want 6 (border 2 + padding 4)", txt.X)
	}
	if txt.Y <= 6 {
		t.Errorf("pen Y = %g, want baseline below line top 6", txt.Y)
	}
}

func TestDrawScale(t *testing.T) {
	t.Parallel()
	tree, styles, res := scene(t)

	r := software.New()
	r.BeginFrame(800, 600)
	if err := Draw(r, tree.Roots(), res, styles, 2); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	r.EndFrame()

	cmds := r.Frame().Commands
	bg := cmds[0].(render.DrawRect)
	if bg.W != 424 || bg.H != 144 {
		t.Errorf("scaled background = %gx%g, want 424x144", bg.W, bg.H)
	}
	txt := cmds[5].(render.DrawText)
	if txt.Options.FontSize != 24 {
		t.Errorf("scaled FontSize = %g, want 24", txt.Options.FontSize)
	}
}

func TestDrawTransparentBackgroundSkipped(t *testing.T) {
	t.Parallel()
	tree := ui.NewTree()
	root := tree.CreateElement("div")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	styles, err := style.Resolve(tree.Roots())
	if err != nil {
		t.Fatal(err)
	}
	res, err := layout.Layout(tree.Roots(), styles, ui.Size{W: 100, H: 50})
	if err != nil {
		t.Fatal(err)
	}
	r := software.New()
	r.BeginFrame(100, 50)
	if err := Draw(r, tree.Roots(), res, styles, 1); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	r.EndFrame()
	if got := len(r.Frame().Commands); got != 0 {
		t.Fatalf("commands = %d, want 0 (transparent background, no border, no text)", got)
	}
}

func TestDrawErrors(t *testing.T) {
	t.Parallel()
	tree, styles, res := scene(t)
	r := software.New()

	if err := Draw(r, tree.Roots(), nil, styles, 1); err == nil {
		t.Error("nil layout result must fail")
	}
	if err := Draw(r, tree.Roots(), res, styles, 0); err == nil {
		t.Error("scale 0 must fail")
	}
	if err := Draw(r, tree.Roots(), res, map[ui.NodeID]style.ComputedStyle{}, 1); err == nil {
		t.Error("missing styles must fail")
	}
}
