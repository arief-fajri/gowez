package app

import (
	"fmt"
	"time"

	"github.com/arief-fajri/gowez/internal/layout"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/paint"
	"github.com/arief-fajri/gowez/internal/render"
	"github.com/arief-fajri/gowez/internal/style"
	"github.com/arief-fajri/gowez/internal/ui"
)

// sceneBackground is the opaque backdrop painted under the UI tree. The
// present contract requires a fully opaque buffer (evidence/learnings.md,
// 2026-10-03), and the subset has no viewport-height unit — the backdrop
// belongs to the scene, not to the tree.
var sceneBackground = render.Color{R: 0x14 / 255.0, G: 0x16 / 255.0, B: 0x1c / 255.0, A: 1}

// demoCSS is the Milestone 2 acceptance stylesheet — only constructs from
// docs/CSS-SUBSET.md (type/class selectors, box + flex properties).
const demoCSS = `
panel { margin: 24px; background-color: #1e2028; padding: 16px;
	border-width: 1px; border-color: #333945;
	color: #a8b0c0; font-size: 13px; }
title { color: #ffffff; font-size: 22px; }
row { display: flex; justify-content: space-between; align-items: center;
	gap: 8px; height: 44px; }
swatch-a { background-color: #2f6feb; width: 120px; height: 40px; }
swatch-b { background-color: #e0873a; width: 80px; height: 40px; }
button { background-color: #2b303b; border-width: 2px; border-color: #4c8dff;
	padding: 6px 12px; color: #ffffff; font-size: 14px; }
status { color: #e0873a; font-size: 13px; }
`

// uiScene is the Milestone 2 acceptance scene: a UI tree styled, laid
// out, and painted through the runtime pipeline on every frame. It
// replaces the Milestone 1 hand-drawn scene — cmd/gowez-hello is
// unchanged, but its pixels now come from the UI runtime.
//
// Relayout happens when the viewport size changes or when scene state
// (the frame counter) changes; each pass is timed into the metrics
// recorder (Module 5 §5.1).
type uiScene struct {
	tree    *ui.Tree
	sheet   *style.Stylesheet
	styles  map[ui.NodeID]style.ComputedStyle
	geom    *layout.Result
	counter *ui.Node
	metrics *observe.Recorder

	dirty        bool
	lastW, lastH int
	frames       int
}

// newUIScene builds the demo tree and stylesheet. The first layout runs
// on the first Draw, against the real viewport size.
func newUIScene(metrics *observe.Recorder) (*uiScene, error) {
	sheet, err := style.Parse(demoCSS)
	if err != nil {
		return nil, fmt.Errorf("scene: demo stylesheet: %w", err)
	}
	tree := ui.NewTree()
	mk := func(parent *ui.Node, tag string) *ui.Node {
		n := tree.CreateElement(tag)
		if parent == nil {
			_ = tree.AppendRoot(n)
		} else {
			_ = tree.Append(parent, n)
		}
		return n
	}
	text := func(parent *ui.Node, s string) *ui.Node {
		n := tree.CreateText(s)
		_ = tree.Append(parent, n)
		return n
	}

	panel := mk(nil, "panel")
	text(mk(panel, "title"), "GoWEZ")
	text(panel, "UI tree → style → layout → paint → commands. No Chromium, no WebView, no OS web view.")
	row := mk(panel, "row")
	mk(row, "swatch-a")
	mk(row, "swatch-b")
	button := mk(row, "button")
	text(button, "Apply")
	counter := text(panel, "frame 000000")

	return &uiScene{
		tree:    tree,
		sheet:   sheet,
		counter: counter,
		metrics: metrics,
		dirty:   true,
	}, nil
}

// Tick advances the scene state; the counter text change marks the tree
// dirty so the next Draw relayouts (state → layout → render).
func (s *uiScene) Tick() {
	s.frames++
	s.counter.Text = fmt.Sprintf("frame %06d", s.frames)
	s.dirty = true
}

// relayout re-resolves styles and recomputes geometry for the given
// viewport, recording the pass duration (P5: layout is observable).
func (s *uiScene) relayout(w, h int) error {
	start := time.Now()
	styles, err := style.Resolve(s.tree.Roots(), s.sheet)
	if err != nil {
		return fmt.Errorf("scene: style resolve: %w", err)
	}
	geom, err := layout.Layout(s.tree.Roots(), styles, ui.Size{W: float64(w), H: float64(h)})
	if err != nil {
		return fmt.Errorf("scene: layout: %w", err)
	}
	s.styles, s.geom = styles, geom
	s.lastW, s.lastH = w, h
	s.dirty = false
	if s.metrics != nil {
		s.metrics.RecordLayout(time.Since(start))
	}
	return nil
}

// Draw paints one frame: backdrop, then the laid-out tree scaled from
// logical pixels (layout units) to framebuffer pixels (HiDPI).
func (s *uiScene) Draw(r render.Renderer, pw, ph, logicalW, logicalH int) error {
	if logicalW <= 0 || logicalH <= 0 || pw <= 0 || ph <= 0 {
		return fmt.Errorf("scene: invalid frame size %dx%d (logical %dx%d)", pw, ph, logicalW, logicalH)
	}
	scale := float64(pw) / float64(logicalW)
	if s.dirty || logicalW != s.lastW || logicalH != s.lastH {
		if err := s.relayout(logicalW, logicalH); err != nil {
			return err
		}
	}
	r.DrawRect(0, 0, float64(pw), float64(ph), sceneBackground)
	if err := paint.Draw(r, s.tree.Roots(), s.geom, s.styles, scale); err != nil {
		return fmt.Errorf("scene: paint: %w", err)
	}
	return nil
}
