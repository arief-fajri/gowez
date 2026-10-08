package app

import (
	"testing"
	"time"

	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/render/backend/software"
	"github.com/arief-fajri/gowez/internal/window"
)

// textBundle mounts an <input> the loop can focus, so a scripted window can
// drive the real platform text-input lifecycle end to end.
const textBundle = bundlePrelude + `
apply({
	version: 1,
	entry: 1,
	ops: [
		{kind: "createElement", nodeId: 1, tag: "panel"},
		{kind: "createElement", nodeId: 2, tag: "input"},
		{kind: "setAttribute", nodeId: 2, name: "tabindex", value: "0"},
		{kind: "setStyle", nodeId: 2, style: {"height": "20px", "background-color": "#333333"}},
		{kind: "createText", nodeId: 3, text: ""},
		{kind: "appendChild", parentId: 1, childId: 2},
		{kind: "appendChild", parentId: 2, childId: 3},
		{kind: "addEventListener", nodeId: 2, event: "textinput", handlerId: 1}
	]
});
// The runtime delivers committed text as an event; the application writes the
// value back through the single mutation door (decision D-1).
gowez.on("h1", function (ev) {
	apply({version: 1, ops: [{kind: "setText", nodeId: 3, value: ev.text}]});
});
`

// TestLoopStartsAndStopsTextInputWithFocus drives the real frame loop with a
// scripted window and asserts the platform text-input lifecycle follows focus:
// started when the input gains focus, stopped once focus leaves it. This is the
// M5 text-input integration evidence, not just a unit-level toggle.
func TestLoopStartsAndStopsTextInputWithFocus(t *testing.T) {
	a, err := New(Options{
		Title:  "text",
		Width:  400,
		Height: 300,
		UI:     bundleFiles(textBundle, counterCSS),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	disp, eng, err := newRuntime(a.metrics, a.reporter)
	if err != nil {
		t.Fatalf("newRuntime: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	sc, err := a.newScene(disp, eng)
	if err != nil {
		t.Fatalf("newScene: %v", err)
	}

	// Geometry first, so the scripted click lands on the input.
	r := software.New()
	r.BeginFrame(400, 300)
	if err := sc.Draw(r, 400, 300, 400, 300); err != nil {
		t.Fatalf("warm-up Draw: %v", err)
	}
	r.EndFrame()

	bs, ok := sc.(*bundleScene)
	if !ok {
		t.Fatalf("scene = %T, want *bundleScene", sc)
	}
	input := findTag(t, bs.tree, "input")
	box := bs.geom.Boxes[input.ID]
	cx := box.Border.X + box.Border.W/2
	cy := box.Border.Y + box.Border.H/2

	win := &fakeWindow{
		w: 400, h: 300,
		pumps: [][]window.Event{
			{}, // frame 1: nothing focused, text input must stay off
			{window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft}},
			// Focus is held from the previous frame, so text must arrive.
			{window.TextInputEvent{Text: "g"}},
			{window.CloseEvent{}},
		},
	}

	if err := a.loop(win, software.New(), sc, time.Now()); err != nil {
		t.Fatalf("loop: %v", err)
	}

	if win.textStarted == 0 {
		t.Error("platform text input never started while an input held focus")
	}
	if got := mirrorText(t, bs, input); got != "g" {
		t.Errorf("committed text = %q, want %q", got, "g")
	}
	// The loop defers StopTextInput on exit when text input is still active.
	if win.textStopped != 1 {
		t.Errorf("textStopped = %d, want 1 (shutdown releases text input)", win.textStopped)
	}
	if win.textActive {
		t.Error("text input still active after the loop returned")
	}
}

// TestLoopNeverStartsTextInputWithoutEditableFocus: the demo scene has no
// editable node, so the platform must never be asked for text input. Starting
// it anyway would make SDL swallow ordinary key events.
func TestLoopNeverStartsTextInputWithoutEditableFocus(t *testing.T) {
	a, err := New(Options{Title: "demo", Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	scene := newTestScene(t, a.metrics, a.reporter)
	warmUp(t, scene, 800, 600)

	btn := findTag(t, scene.tree, "button")
	box := scene.geom.Boxes[btn.ID]
	cx := box.Border.X + box.Border.W/2
	cy := box.Border.Y + box.Border.H/2

	win := &fakeWindow{
		w: 800, h: 600,
		pumps: [][]window.Event{
			{window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft}},
			{window.CloseEvent{}},
		},
	}
	if err := a.loop(win, software.New(), scene, time.Now()); err != nil {
		t.Fatalf("loop: %v", err)
	}
	if win.textStarted != 0 {
		t.Errorf("textStarted = %d, want 0 for a scene with no editable node", win.textStarted)
	}
	if scene.FocusEditable() {
		t.Error("demo scene reports an editable focus")
	}
}

// TestTextInputMetricsAreRecorded keeps the observability promise (P5): text
// input is a measured event like any other, not an invisible side effect.
func TestTextInputMetricsAreRecorded(t *testing.T) {
	rec := observe.NewRecorder()
	files := bundleFiles(textBundle, counterCSS)
	s, _ := newTestBundle(t, files, rec, nil)
	warmUpBundle(t, s, 400, 300)

	before := rec.Snapshot().InputEvents
	s.HandleInput(window.TextInputEvent{Text: "a"})
	s.HandleInput(window.TextEditingEvent{Text: "a", Start: 0, Length: 1})
	if got := rec.Snapshot().InputEvents - before; got != 2 {
		t.Errorf("InputEvents grew by %d, want 2", got)
	}
}
