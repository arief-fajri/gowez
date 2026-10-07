package app

import (
	"strings"
	"testing"
	"time"

	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/render/backend/software"
	"github.com/arief-fajri/gowez/internal/ui"
	"github.com/arief-fajri/gowez/internal/window"
)

// findTag returns the first element node with the given tag.
func findTag(t *testing.T, tree *ui.Tree, tag string) *ui.Node {
	t.Helper()
	var found *ui.Node
	var walk func(*ui.Node)
	walk = func(n *ui.Node) {
		if found != nil || n == nil {
			return
		}
		if n.Kind == ui.ElementNode && n.Tag == tag {
			found = n
			return
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range tree.Roots() {
		walk(r)
	}
	if found == nil {
		t.Fatalf("no <%s> node in scene", tag)
	}
	return found
}

// newTestScene builds the scene the way the startup sequence does —
// runtime (IPC dispatcher + JS engine) → scene → script binding — without
// opening a window. Pass a reporter to observe script/IPC diagnostics.
func newTestScene(t *testing.T, rec *observe.Recorder, rep observe.Reporter) *uiScene {
	t.Helper()
	disp, eng, err := newRuntime(rec, rep)
	if err != nil {
		t.Fatalf("newRuntime: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	scene, err := newUIScene(rec, eng, disp)
	if err != nil {
		t.Fatalf("newUIScene: %v", err)
	}
	if err := scene.bindScript(); err != nil {
		t.Fatalf("bindScript: %v", err)
	}
	return scene
}

// warmUp draws one frame so the tree has geometry, and returns the
// center of the button in logical (event) coordinates.
func warmUp(t *testing.T, scene *uiScene, w, h int) (cx, cy float64) {
	t.Helper()
	r := software.New()
	r.BeginFrame(w, h)
	if err := scene.Draw(r, w, h, w, h); err != nil {
		t.Fatalf("warm-up Draw: %v", err)
	}
	r.EndFrame()
	btn := findTag(t, scene.tree, "button")
	b, ok := scene.geom.Boxes[btn.ID]
	if !ok {
		t.Fatal("button has no geometry after layout")
	}
	if got := len(scene.tree.Geometry()); got == 0 {
		t.Fatal("tree geometry not published by relayout")
	}
	return b.Border.X + b.Border.W/2, b.Border.Y + b.Border.H/2
}

// TestUISceneGeometryPublishedToTree closes the M3 wiring gap: layout
// results must reach the tree so hit testing works in the running app
// (M2 shipped HitTest but only tests ever called SetGeometry).
func TestUISceneGeometryPublishedToTree(t *testing.T) {
	scene := newTestScene(t, nil, nil)
	warmUp(t, scene, 800, 600)

	// The panel covers the top-left area; hit testing must find it.
	if got := scene.tree.HitTest(40, 40); got == nil {
		t.Fatal("HitTest(40,40) = nil after layout published geometry")
	}
	if got := scene.tree.HitTest(-10, -10); got != nil {
		t.Fatalf("HitTest outside the viewport = %v, want nil", got)
	}
}

// TestSceneClickUpdatesState: a click reaches the button listener,
// mutates scene state, and marks the tree dirty (state → layout).
func TestSceneClickUpdatesState(t *testing.T) {
	scene := newTestScene(t, nil, nil)
	cx, cy := warmUp(t, scene, 800, 600)

	scene.handleInput(window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft})
	scene.handleInput(window.PointerEvent{X: cx, Y: cy, Press: false, Button: window.ButtonLeft})

	// The counter is owned by the JS handler (M4): a click dispatched by
	// internal/ui ran scene.js, which called ui.setText over IPC.
	if got := scene.clicksNode.Text; got != "clicks: 1" {
		t.Fatalf("clicks text = %q, want %q", got, "clicks: 1")
	}
	if !scene.dirty {
		t.Fatal("dirty = false after a click, want true")
	}
	// Focus moved to the button on the primary press (tabindex=0).
	if btn := findTag(t, scene.tree, "button"); scene.tree.Focused() != btn {
		t.Fatalf("focused = %v, want the button", scene.tree.Focused())
	}

	// A drag-away release fires no second click.
	scene.handleInput(window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft})
	scene.handleInput(window.PointerEvent{X: cx + 500, Y: cy + 500, Press: false, Button: window.ButtonLeft})
	if got := scene.clicksNode.Text; got != "clicks: 1" {
		t.Fatalf("clicks text after drag-away = %q, want %q", got, "clicks: 1")
	}
}

// TestSceneKeyEchoUpdatesStatus: keys reach the focused node's
// listener; without focus they change nothing.
func TestSceneKeyEchoUpdatesStatus(t *testing.T) {
	scene := newTestScene(t, nil, nil)
	cx, cy := warmUp(t, scene, 800, 600)

	// No focus yet: the key is dropped, status untouched.
	scene.handleInput(window.KeyEvent{Key: "arrow-left", Press: true})
	if got := scene.keyline.Text; got != "key: —" {
		t.Fatalf("keyline without focus = %q, want unchanged", got)
	}

	// Click focuses the button; the next key echoes.
	scene.handleInput(window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft})
	scene.handleInput(window.PointerEvent{X: cx, Y: cy, Press: false, Button: window.ButtonLeft})
	scene.handleInput(window.KeyEvent{Key: "arrow-left", Press: true})
	if got := scene.keyline.Text; got != "key: arrow-left" {
		t.Fatalf("keyline = %q, want %q", got, "key: arrow-left")
	}
	if !scene.dirty {
		t.Fatal("dirty = false after a key, want true")
	}
}

// TestHandleInputRecordsMetrics: interaction is observable (P5) —
// fed events, handler runs, and unhandled events are all counted.
func TestHandleInputRecordsMetrics(t *testing.T) {
	rec := observe.NewRecorder()
	scene := newTestScene(t, rec, nil)
	warmUp(t, scene, 800, 600)

	// Motion over the backdrop: no target, no listener → unhandled.
	scene.handleInput(window.PointerEvent{X: -20, Y: -20})
	m := rec.Snapshot()
	if m.InputEvents != 1 || m.UnhandledEvents != 1 || m.DispatchedEvents != 0 {
		t.Fatalf("metrics = %+v, want 1 input, 1 unhandled, 0 dispatched", m)
	}

	// Click + key: handler runs are counted.
	cx, cy := warmUp(t, scene, 800, 600)
	scene.handleInput(window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft})
	scene.handleInput(window.PointerEvent{X: cx, Y: cy, Press: false, Button: window.ButtonLeft})
	scene.handleInput(window.KeyEvent{Key: "a", Press: true})
	m = rec.Snapshot()
	if m.InputEvents != 4 {
		t.Fatalf("InputEvents = %d, want 4", m.InputEvents)
	}
	if m.DispatchedEvents < 2 {
		t.Fatalf("DispatchedEvents = %d, want >= 2 (click + key)", m.DispatchedEvents)
	}
	if m.HandlerPanics != 0 {
		t.Fatalf("HandlerPanics = %d, want 0", m.HandlerPanics)
	}
}

// fakeWindow is a scripted window.Window for headless loop tests: each
// Pump returns the next scripted batch, and the last frame carries a
// CloseEvent so the loop exits.
type fakeWindow struct {
	pumps    [][]window.Event
	idx      int
	presents int
	w, h     int
}

func (f *fakeWindow) Title() string         { return "fake" }
func (f *fakeWindow) SetTitle(string)       {}
func (f *fakeWindow) Size() (int, int)      { return f.w, f.h }
func (f *fakeWindow) PixelSize() (int, int) { return f.w, f.h }
func (f *fakeWindow) Show()                 {}
func (f *fakeWindow) Close()                {}

func (f *fakeWindow) Pump() []window.Event {
	if f.idx >= len(f.pumps) {
		return nil
	}
	p := f.pumps[f.idx]
	f.idx++
	return p
}

func (f *fakeWindow) Present(px []byte, w, h int) error {
	f.presents++
	return nil
}

// TestLoopDispatchesInput drives the real frame loop with a scripted
// window: pump → dispatch → state change → dirty → draw. This is the
// end-to-end "mouse click reaches UI" evidence.
func TestLoopDispatchesInput(t *testing.T) {
	a, err := New(Options{Title: "loop", Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	scene := newTestScene(t, a.metrics, nil)
	cx, cy := warmUp(t, scene, 800, 600)

	win := &fakeWindow{
		w: 800, h: 600,
		pumps: [][]window.Event{
			{},
			{window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft}},
			{window.PointerEvent{X: cx, Y: cy, Press: false, Button: window.ButtonLeft}},
			{window.KeyEvent{Key: "arrow-left", Press: true}},
			{window.CloseEvent{}},
		},
	}
	rend := software.New()
	if err := a.loop(win, rend, scene, time.Now()); err != nil {
		t.Fatalf("loop: %v", err)
	}

	if got := scene.clicksNode.Text; got != "clicks: 1" {
		t.Fatalf("clicks text after loop = %q, want %q (click synthesized in the loop, handled by scene.js)", got, "clicks: 1")
	}
	if got := scene.keyline.Text; got != "key: arrow-left" {
		t.Fatalf("keyline after loop = %q, want %q", got, "key: arrow-left")
	}
	if win.presents < 4 {
		t.Fatalf("presents = %d, want >= 4", win.presents)
	}
	m := a.Metrics()
	if m.InputEvents != 3 {
		t.Fatalf("InputEvents = %d, want 3 (down, up, key)", m.InputEvents)
	}
	if m.DispatchedEvents < 2 {
		t.Fatalf("DispatchedEvents = %d, want >= 2", m.DispatchedEvents)
	}
	if m.FrameCount == 0 || m.LayoutCount == 0 {
		t.Fatalf("frame/layout metrics = %d/%d, want both > 0", m.FrameCount, m.LayoutCount)
	}
}

// TestLoopRecoversHandlerPanic: a panicking handler must not stop the
// loop — the fault is recovered, reported, and the app keeps running.
func TestLoopRecoversHandlerPanic(t *testing.T) {
	a, err := New(Options{Title: "panic", Width: 400, Height: 300})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec := &captureReporter{}
	a.reporter = rec
	scene := newTestScene(t, a.metrics, nil)
	a.wireScenePanic(scene)
	btn := findTag(t, scene.tree, "button")
	scene.tree.AddEventListener(btn, ui.Click, func(*ui.Event) { panic("handler boom") })
	cx, cy := warmUp(t, scene, 400, 300)

	win := &fakeWindow{
		w: 400, h: 300,
		pumps: [][]window.Event{
			{window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft}},
			{window.PointerEvent{X: cx, Y: cy, Press: false, Button: window.ButtonLeft}},
			{window.CloseEvent{}},
		},
	}
	rend := software.New()
	if err := a.loop(win, rend, scene, time.Now()); err != nil {
		t.Fatalf("loop survived the panic but returned an error: %v", err)
	}
	if got := a.Metrics().HandlerPanics; got != 1 {
		t.Fatalf("HandlerPanics = %d, want 1", got)
	}
	if len(rec.diags) != 1 || rec.diags[0].Component != "ui" {
		t.Fatalf("diagnostics = %+v, want one ui diagnostic", rec.diags)
	}
}

// captureReporter collects diagnostics for assertions.
type captureReporter struct {
	diags []observe.Diagnostic
}

func (c *captureReporter) Report(d observe.Diagnostic) {
	c.diags = append(c.diags, d)
}

// TestSceneJSDrivesStateThroughIPC is the Milestone 4 end-to-end path:
// input → dispatch → JS handler → gowez.invoke → IPC dispatcher →
// ui.setText → dirty → next frame. It also pins the startup success path
// (scene.js invoked app.getInfo and the result is on screen).
// Checklist evidence: "UI can invoke Go API", "Go can return success",
// "UI state can update".
func TestSceneJSDrivesStateThroughIPC(t *testing.T) {
	rec := observe.NewRecorder()
	scene := newTestScene(t, rec, nil)
	cx, cy := warmUp(t, scene, 800, 600)

	// Startup: scene.js called app.getInfo through the dispatcher and wrote
	// the answer into the status line.
	if got := scene.statusNode.Text; !strings.Contains(got, "ipc: gowez/1/goja") {
		t.Fatalf("status = %q, want the app.getInfo payload", got)
	}

	// Click → counterClick (JS) → ui.setText (IPC).
	scene.handleInput(window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft})
	scene.handleInput(window.PointerEvent{X: cx, Y: cy, Press: false, Button: window.ButtonLeft})
	if got := scene.clicksNode.Text; got != "clicks: 1" {
		t.Fatalf("clicks = %q, want %q", got, "clicks: 1")
	}
	if !scene.dirty {
		t.Fatal("dirty = false after a JS-driven update")
	}

	// Key → keyEcho (JS) → ui.setText (IPC).
	scene.handleInput(window.KeyEvent{Key: "enter", Press: true})
	if got := scene.keyline.Text; got != "key: enter" {
		t.Fatalf("keyline = %q, want %q", got, "key: enter")
	}

	m := rec.Snapshot()
	if m.IPCCount < 4 { // getInfo + status + clicks + keyline
		t.Fatalf("IPCCount = %d, want >= 4", m.IPCCount)
	}
	if m.IPCErrorCount != 0 {
		t.Fatalf("IPCErrorCount = %d, want 0", m.IPCErrorCount)
	}
	if m.JSExceptions != 0 {
		t.Fatalf("JSExceptions = %d, want 0", m.JSExceptions)
	}
	if m.LastIPCDuration <= 0 {
		t.Fatalf("LastIPCDuration = %v, want > 0 (P5)", m.LastIPCDuration)
	}
	if m.LastJSEvalDuration <= 0 {
		t.Fatalf("LastJSEvalDuration = %v, want > 0 (P5)", m.LastJSEvalDuration)
	}
}

// TestSceneJSHandlerFailureIsObservable: a throwing JS handler is isolated
// and observable — JSExceptions counts it, a Component "script" diagnostic
// is reported, and the scene keeps drawing (guard rail G-REL-02, failure
// experiment B's unit-level shape).
func TestSceneJSHandlerFailureIsObservable(t *testing.T) {
	rec := observe.NewRecorder()
	rep := &captureReporter{}
	scene := newTestScene(t, rec, rep)

	if err := scene.eng.Eval("bad.js", `gowez.on("boom", function () { throw new Error("handler boom"); });`); err != nil {
		t.Fatalf("eval: %v", err)
	}
	if err := scene.eng.FireHandler("boom", nil); err == nil {
		t.Fatal("throwing handler must surface an error")
	}

	m := rec.Snapshot()
	if m.JSExceptions == 0 {
		t.Fatalf("JSExceptions = 0, want >= 1 (P5)")
	}
	found := false
	for _, d := range rep.diags {
		if d.Component == "script" {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v, want a script component", rep.diags)
	}

	// The runtime survives: the scene still draws a frame.
	r := software.New()
	r.BeginFrame(800, 600)
	if err := scene.Draw(r, 800, 600, 800, 600); err != nil {
		t.Fatalf("scene unusable after JS exception: %v", err)
	}
	r.EndFrame()
}
