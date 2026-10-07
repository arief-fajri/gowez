package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/arief-fajri/gowez/internal/ipc"
	"github.com/arief-fajri/gowez/internal/layout"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/paint"
	"github.com/arief-fajri/gowez/internal/render"
	"github.com/arief-fajri/gowez/internal/script"
	"github.com/arief-fajri/gowez/internal/style"
	"github.com/arief-fajri/gowez/internal/ui"
)

// sceneBackground is the opaque backdrop painted under the UI tree. The
// present contract requires a fully opaque buffer (evidence/learnings.md,
// 2026-10-03), and the subset has no viewport-height unit — the backdrop
// belongs to the scene, not to the tree.
var sceneBackground = render.Color{R: 0x14 / 255.0, G: 0x16 / 255.0, B: 0x1c / 255.0, A: 1}

// demoCSS is the Milestone 3 acceptance stylesheet — constructs from
// docs/CSS-SUBSET.md: type/class selectors, box + flex properties, and
// the Milestone 3 pseudo-classes (:hover, :active, :focus). Pseudo
// rules change colors only — never geometry — so a hover transition
// cannot shift the boxes under the pointer (docs/EVENTS.md).
const demoCSS = `
panel { margin: 24px; background-color: #1e2028; padding: 16px;
	border-width: 1px; border-color: #333945;
	color: #a8b0c0; font-size: 13px; }
title { color: #ffffff; font-size: 22px; }
row { display: flex; justify-content: space-between; align-items: center;
	gap: 8px; height: 44px; }
swatch-a { background-color: #2f6feb; width: 120px; height: 40px; }
swatch-a:hover { background-color: #4c8dff; }
swatch-a:focus { background-color: #7fb0ff; }
swatch-b { background-color: #e0873a; width: 80px; height: 40px; }
swatch-b:hover { background-color: #f0a05a; }
swatch-b:focus { background-color: #ffc98a; }
button { background-color: #2b303b; border-width: 2px; border-color: #4c8dff;
	padding: 6px 12px; color: #ffffff; font-size: 14px; }
button:hover { background-color: #394152; }
button:active { background-color: #1f5bd6; }
button:focus { border-color: #7fb0ff; }
status { color: #e0873a; font-size: 13px; }
`

// demoJS is the Milestone 4 acceptance script: the scene's interaction
// logic runs in the embedded engine, reaches Go through gowez.invoke (the
// IPC dispatcher), and writes results back into the UI tree. Host surface
// and divergences: docs/SCRIPT.md.
//
//	__scene        node ids injected by bindScript (scene-ids.js)
//	gowez.on       register a handler (bound to the tree by Go)
//	gowez.invoke   synchronous IPC call; throws an Error with .code on failure
//	gowez.call     Promise-returning form of invoke
const demoJS = `
var sceneState = { clicks: 0, info: null };

gowez.on("counterClick", function () {
	sceneState.clicks += 1;
	gowez.invoke("ui.setText", { nodeId: __scene.clicks, text: "clicks: " + sceneState.clicks });
});

gowez.on("keyEcho", function (ev) {
	gowez.invoke("ui.setText", { nodeId: __scene.keyline, text: "key: " + ev.key });
});

// Startup proof: JS reaches a Go API through the dispatcher and the answer
// lands in the UI (checklist: "UI can invoke Go API", "Go can return
// success").
sceneState.info = gowez.invoke("app.getInfo");
gowez.invoke("ui.setText", {
	nodeId: __scene.status,
	text: "js: ok - ipc: " + sceneState.info.name + "/" + sceneState.info.ipcVersion + "/" + sceneState.info.engine
});
`

// uiScene is the Milestone 4 acceptance scene: a UI tree styled, laid
// out, and painted through the runtime pipeline, with interaction logic
// living in the embedded JavaScript engine — clicking the button and
// echoing keys run JS handlers that call back into Go over IPC.
//
// Relayout happens when the viewport size changes or when scene state
// changes (frame counter, JS-driven text, interaction state); each pass
// is timed into the metrics recorder (Module 5 §5.1).
type uiScene struct {
	tree       *ui.Tree
	sheet      *style.Stylesheet
	styles     map[ui.NodeID]style.ComputedStyle
	geom       *layout.Result
	counter    *ui.Node
	clicksNode *ui.Node
	keyline    *ui.Node
	statusNode *ui.Node
	buttonNode *ui.Node
	panelNode  *ui.Node
	// nodesByID is the address space ui.setText exposes to JavaScript —
	// only nodes registered here are reachable (G-SEC-01: no tree-wide
	// ambient access from the script layer).
	nodesByID map[ui.NodeID]*ui.Node
	metrics   *observe.Recorder
	eng       script.Engine

	dirty        bool
	lastW, lastH int
	frames       int
}

// newUIScene builds the demo tree and stylesheet, and registers the
// runtime-owned ui.setText method on the dispatcher. The first layout
// runs on the first Draw, against the real viewport size. Script loading
// happens separately in bindScript so a script failure reports as a
// "script" startup fault, not a scene fault.
func newUIScene(metrics *observe.Recorder, eng script.Engine, disp *ipc.Dispatcher) (*uiScene, error) {
	if eng == nil {
		return nil, fmt.Errorf("scene: nil script engine")
	}
	if disp == nil {
		return nil, fmt.Errorf("scene: nil ipc dispatcher")
	}
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
	swatchA := mk(row, "swatch-a")
	swatchB := mk(row, "swatch-b")
	button := mk(row, "button")
	text(button, "Apply")
	counter := text(panel, "frame 000000")
	clicks := text(panel, "clicks: 0")
	clicks.SetAttribute("style", "color: #e0873a")
	keyline := text(panel, "key: —")
	keyline.SetAttribute("style", "color: #7fb0ff")
	status := text(panel, "js: —")
	status.SetAttribute("style", "color: #6fcf97")

	// Focusable elements for Tab traversal (docs/EVENTS.md §focus).
	swatchA.SetAttribute("tabindex", "0")
	swatchB.SetAttribute("tabindex", "0")
	button.SetAttribute("tabindex", "0")

	s := &uiScene{
		tree:       tree,
		sheet:      sheet,
		counter:    counter,
		clicksNode: clicks,
		keyline:    keyline,
		statusNode: status,
		buttonNode: button,
		panelNode:  panel,
		nodesByID: map[ui.NodeID]*ui.Node{
			clicks.ID:  clicks,
			keyline.ID: keyline,
			status.ID:  status,
		},
		metrics: metrics,
		eng:     eng,
		dirty:   true,
	}

	// ui.setText is the script layer's only UI mutation door: inline (the
	// tree is single-goroutine — it runs on the UI goroutine that called
	// gowez.invoke), permission-free (it touches no OS capability), and
	// addressable only for nodes the scene registered above.
	if err := disp.RegisterInline("ui.setText", "", s.handleSetText); err != nil {
		return nil, fmt.Errorf("scene: register ui.setText: %w", err)
	}
	return s, nil
}

// handleSetText is the ui.setText IPC handler: {nodeId, text} → node text
// + dirty. Unknown ids fail with CodeInvalidParams — a failed update never
// applies (I1).
func (s *uiScene) handleSetText(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	_ = ctx
	var p struct {
		NodeID int    `json:"nodeId"`
		Text   string `json:"text"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, ipc.NewCodeError(ipc.CodeInvalidParams, fmt.Sprintf("ui.setText: bad params: %v", err))
	}
	n, ok := s.nodesByID[ui.NodeID(p.NodeID)]
	if !ok {
		return nil, ipc.NewCodeError(ipc.CodeInvalidParams, fmt.Sprintf("ui.setText: unknown node %d", p.NodeID))
	}
	n.Text = p.Text
	s.dirty = true
	return json.RawMessage(`true`), nil
}

// bindScript loads the scene script and binds its handlers to the tree.
// Startup order: the node-id prelude first (demoJS reads __scene), then
// demoJS itself, then a hard check that every handler the listeners need
// actually exists — a missing handler is an explicit load failure, never a
// silent dead button.
func (s *uiScene) bindScript() error {
	ids := fmt.Sprintf("var __scene = {clicks: %d, keyline: %d, status: %d};",
		s.clicksNode.ID, s.keyline.ID, s.statusNode.ID)
	if err := s.eng.Eval("scene-ids.js", ids); err != nil {
		return fmt.Errorf("scene ids: %w", err)
	}
	if err := s.eng.Eval("scene.js", demoJS); err != nil {
		return fmt.Errorf("scene.js: %w", err)
	}
	for _, name := range []string{"counterClick", "keyEcho"} {
		if !s.eng.HasHandler(name) {
			return fmt.Errorf("scene.js did not register handler %q", name)
		}
	}

	// Thin Go-side listeners: dispatch (bubbling, focus, metrics) stays in
	// internal/ui; the logic moves to JS. FireHandler failures are already
	// observed by the engine (diagnostic + JSExceptions), so the listener
	// itself has nothing left to report (docs/EVENTS.md §handler contract:
	// inline on the UI goroutine, must not block).
	s.tree.AddEventListener(s.buttonNode, ui.Click, func(*ui.Event) {
		_ = s.eng.FireHandler("counterClick", json.RawMessage(`{"type":"click"}`))
	})
	s.tree.AddEventListener(s.panelNode, ui.KeyDown, func(e *ui.Event) {
		payload, _ := json.Marshal(map[string]any{"type": "key-down", "key": e.Key})
		_ = s.eng.FireHandler("keyEcho", payload)
	})
	return nil
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
	// Publish geometry to the tree so hit testing tracks this pass —
	// assigned only after style and layout both succeeded (invariant
	// I1: no half-applied update).
	s.tree.SetGeometry(geom.Boxes)
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
