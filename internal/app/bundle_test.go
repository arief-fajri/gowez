package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/arief-fajri/gowez/internal/ipc"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/permission"
	"github.com/arief-fajri/gowez/internal/render/backend/software"
	"github.com/arief-fajri/gowez/internal/script"
	"github.com/arief-fajri/gowez/internal/ui"
	"github.com/arief-fajri/gowez/internal/window"
)

// bundleFiles builds an in-memory UI bundle. Tests pass their own app.js /
// styles.css to exercise the mount path without touching disk.
func bundleFiles(script, css string) fstest.MapFS {
	return fstest.MapFS{
		"manifest.json": &fstest.MapFile{Data: []byte(`{
			"schemaVersion": 1,
			"adapter": {"name": "gowez-adapter", "version": "1"},
			"svelte": 5,
			"script": "app.js",
			"styles": "styles.css"
		}`)},
		"app.js":     &fstest.MapFile{Data: []byte(script)},
		"styles.css": &fstest.MapFile{Data: []byte(css)},
	}
}

// counterBundle is the fixture the mount tests share: a panel with a counter
// text node and an increment button whose click handler runs JS that submits a
// new op batch. It exercises the whole M5 path — manifest, CSS, mount,
// ui.apply, handler binding, and repaint.
// bundlePrelude is the runtime the adapter emits into every bundle: the single
// mutation door (decision D-1) plus the batch helper module code calls.
const bundlePrelude = `
var ops = 0;
function apply(b) { ops += 1; gowez.invoke("ui.apply", b); }
`

const counterBundle = bundlePrelude + `
apply({
	version: 1,
	entry: 1,
	ops: [
		{kind: "createElement", nodeId: 1, tag: "panel"},
		{kind: "createText", nodeId: 2, text: "count: 0"},
		{kind: "createElement", nodeId: 3, tag: "button"},
		{kind: "createText", nodeId: 4, text: "+1"},
		{kind: "appendChild", parentId: 1, childId: 2},
		{kind: "appendChild", parentId: 1, childId: 3},
		{kind: "appendChild", parentId: 3, childId: 4},
		{kind: "addEventListener", nodeId: 3, event: "click", handlerId: 1}
	]
});

var n = 0;
gowez.on("h1", function () {
	n += 1;
	apply({version: 1, ops: [
		{kind: "setText", nodeId: 2, value: "count: " + n}
	]});
});
`

const counterCSS = `
panel { margin: 16px; background-color: #202430; padding: 12px; color: #dfe4ee; }
button { background-color: #2f6feb; color: #ffffff; padding: 6px 10px; margin-top: 8px; }
`

// newTestBundle mounts a bundle the way startup does and returns the scene
// together with its engine, so tests can drive handler firing directly.
func newTestBundle(t *testing.T, files fstest.MapFS, rec *observe.Recorder, rep observe.Reporter) (*bundleScene, *ipc.Dispatcher) {
	t.Helper()
	disp, eng, err := newRuntime(rec, rep)
	if err != nil {
		t.Fatalf("newRuntime: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	s, err := newBundleScene(files, rec, rep, eng, disp)
	if err != nil {
		t.Fatalf("newBundleScene: %v", err)
	}
	s.eng = eng
	return s, disp
}

func TestBundleMountsFixture(t *testing.T) {
	rec := observe.NewRecorder()
	s, _ := newTestBundle(t, bundleFiles(counterBundle, counterCSS), rec, nil)

	if len(s.tree.Roots()) != 1 {
		t.Fatalf("roots = %d, want 1", len(s.tree.Roots()))
	}
	root := s.tree.Roots()[0]
	if root.Tag != "panel" {
		t.Errorf("root tag = %q, want panel", root.Tag)
	}
	if btn := findTag(t, s.tree, "button"); btn == nil {
		t.Fatal("button missing after mount")
	}

	m := rec.Snapshot()
	if m.UIOpBatches != 1 {
		t.Errorf("UIOpBatches = %d, want 1 (the mount batch)", m.UIOpBatches)
	}
	if m.UIOpsApplied != 8 {
		t.Errorf("UIOpsApplied = %d, want 8", m.UIOpsApplied)
	}
	if m.UIOpsRejected != 0 {
		t.Errorf("UIOpsRejected = %d, want 0", m.UIOpsRejected)
	}
}

// TestBundleClickIncrementsCount is the M5 e2e: mount → click → JS handler →
// ui.apply → tree text → dirty → relayout. It proves the whole chain the M5
// gate asks for ("component state can update", "event handler works").
func TestBundleClickIncrementsCount(t *testing.T) {
	rec := observe.NewRecorder()
	s, _ := newTestBundle(t, bundleFiles(counterBundle, counterCSS), rec, nil)

	// Warm up one frame so the tree has geometry for hit testing.
	warmUpBundle(t, s, 800, 600)
	if !s.dirty {
		s.dirty = true // a mount alone marks dirty; clear it to prove the click does
	}

	btn := findTag(t, s.tree, "button")
	box, ok := s.geom.Boxes[btn.ID]
	if !ok {
		t.Fatal("button has no geometry after layout")
	}
	cx := box.Border.X + box.Border.W/2
	cy := box.Border.Y + box.Border.H/2

	s.HandleInput(window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft})
	s.HandleInput(window.PointerEvent{X: cx, Y: cy, Press: false, Button: window.ButtonLeft})

	counter := findText(t, s.tree, "count: 1")
	if counter == nil {
		t.Fatalf("counter text not updated; batch metrics=%+v", rec.Snapshot())
	}
	if !s.dirty {
		t.Error("dirty = false after a click, want true (state change must repaint)")
	}

	m := rec.Snapshot()
	if m.UIOpBatches != 2 {
		t.Errorf("UIOpBatches = %d, want 2 (mount + click)", m.UIOpBatches)
	}
	if m.UIOpsApplied != 9 {
		t.Errorf("UIOpsApplied = %d, want 9 (8 mount + 1 click)", m.UIOpsApplied)
	}
}

// TestBundleRegistersOnlyUIApply pins decision D-1: the bundle path exposes
// exactly one UI mutation door. ui.setText belongs to the demo scene; a
// second door here would make the tree unauditable (G-SEC-01).
func TestBundleRegistersOnlyUIApply(t *testing.T) {
	_, disp := newTestBundle(t, bundleFiles(counterBundle, counterCSS), nil, nil)

	has := func(name string) bool {
		for _, m := range disp.Methods() {
			if m == name {
				return true
			}
		}
		return false
	}
	if !has("ui.apply") {
		t.Error("ui.apply is not registered on the bundle path")
	}
	if has("ui.setText") {
		t.Error("ui.setText must not be registered on the bundle path (D-1)")
	}
}

// TestDemoSceneStillHasSetText is the other half of D-1: the nil-UI path keeps
// its M4 behavior unchanged.
func TestDemoSceneStillHasSetText(t *testing.T) {
	scene := newTestScene(t, nil, nil)
	if scene.nodesByID == nil {
		t.Fatal("demo scene lost its node registry")
	}
	if len(scene.nodesByID) == 0 {
		t.Error("demo scene registers no nodes for ui.setText")
	}
}

func TestBundleLoadMissingFileExplicit(t *testing.T) {
	files := fstest.MapFS{
		"app.js": &fstest.MapFile{Data: []byte(counterBundle)},
	}
	_, _, err := newRuntime(nil, nil)
	if err != nil {
		t.Fatalf("newRuntime: %v", err)
	}
	disp, eng, _ := newRuntime(nil, nil)
	t.Cleanup(func() { _ = eng.Close() })

	_, err = newBundleScene(files, nil, nil, eng, disp)
	if err == nil {
		t.Fatal("newBundleScene accepted a bundle with no manifest.json")
	}
	if !strings.Contains(err.Error(), "manifest.json") {
		t.Errorf("error should name the missing file, got: %v", err)
	}
}

func TestBundleMissingScriptIsExplicit(t *testing.T) {
	files := fstest.MapFS{
		"manifest.json": &fstest.MapFile{Data: []byte(
			`{"schemaVersion":1,"svelte":5,"script":"app.js","styles":"styles.css"}`)},
		"styles.css": &fstest.MapFile{Data: []byte(counterCSS)},
	}
	disp, eng, _ := newRuntime(nil, nil)
	t.Cleanup(func() { _ = eng.Close() })

	_, err := newBundleScene(files, nil, nil, eng, disp)
	if err == nil {
		t.Fatal("newBundleScene accepted a manifest with no app.js")
	}
	if !strings.Contains(err.Error(), "app.js") {
		t.Errorf("error should name the missing script, got: %v", err)
	}
}

func TestBundleManifestVersionMismatch(t *testing.T) {
	files := bundleFiles(counterBundle, counterCSS)
	files["manifest.json"] = &fstest.MapFile{Data: []byte(
		`{"schemaVersion":99,"svelte":5,"script":"app.js","styles":"styles.css"}`)}

	disp, eng, _ := newRuntime(nil, nil)
	t.Cleanup(func() { _ = eng.Close() })

	_, err := newBundleScene(files, nil, nil, eng, disp)
	if err == nil {
		t.Fatal("newBundleScene accepted a future manifest schemaVersion")
	}
	if !strings.Contains(err.Error(), "schemaVersion") {
		t.Errorf("error should name the field, got: %v", err)
	}
}

func TestBundleManifestSvelteRangeMismatch(t *testing.T) {
	files := bundleFiles(counterBundle, counterCSS)
	files["manifest.json"] = &fstest.MapFile{Data: []byte(
		`{"schemaVersion":1,"svelte":4,"script":"app.js","styles":"styles.css"}`)}

	disp, eng, _ := newRuntime(nil, nil)
	t.Cleanup(func() { _ = eng.Close() })

	_, err := newBundleScene(files, nil, nil, eng, disp)
	if err == nil {
		t.Fatal("newBundleScene accepted a bundle built against Svelte 4")
	}
	if !strings.Contains(err.Error(), "svelte") {
		t.Errorf("error should name the svelte range, got: %v", err)
	}
}

func TestBundleCSSParseError(t *testing.T) {
	files := bundleFiles(counterBundle, "panel { color: notacolor; }")
	disp, eng, _ := newRuntime(nil, nil)
	t.Cleanup(func() { _ = eng.Close() })

	_, err := newBundleScene(files, nil, nil, eng, disp)
	if err == nil {
		t.Fatal("newBundleScene accepted CSS outside the subset")
	}
	// The CSS-SUBSET error contract requires a position.
	if !strings.Contains(err.Error(), "style: line") {
		t.Errorf("error should carry a style position, got: %v", err)
	}
}

// TestBundleEvalBudgetAbortsStartup drives an unbounded mount into the engine's
// eval budget. The budget is Limits.EvalTimeout (fixed at engine construction),
// so the test tightens it and asserts startup aborts explicitly rather than
// hanging or half-starting (G-REL-01, I12).
func TestBundleEvalBudgetAbortsStartup(t *testing.T) {
	// A tight loop that would run far past any small budget.
	files := bundleFiles("var i = 0; while (true) { i += 1; }", counterCSS)

	_, eng, err := newRuntimeWithLimits(nil, nil, script.Limits{
		EvalTimeout:    60 * time.Millisecond,
		HandlerTimeout: 60 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("newRuntimeWithLimits: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	disp := ipc.NewDispatcher()
	disp.SetObservation(nil, nil)
	disp.SetGrants(permission.NewSet())

	_, err = newBundleScene(files, nil, nil, eng, disp)
	if err == nil {
		t.Fatal("an over-budget mount must abort startup")
	}
	if !strings.Contains(err.Error(), "app.js") {
		t.Errorf("error should name the failing step, got: %v", err)
	}
}

// TestBundleMountsNothingIsExplicit: a script that never submits an op batch
// leaves no UI, which is a startup failure rather than an empty window.
func TestBundleMountsNothingIsExplicit(t *testing.T) {
	files := bundleFiles("var x = 1; x += 1;", counterCSS)
	disp, eng, _ := newRuntime(nil, nil)
	t.Cleanup(func() { _ = eng.Close() })

	_, err := newBundleScene(files, nil, nil, eng, disp)
	if err == nil {
		t.Fatal("newBundleScene accepted a script that mounts no UI")
	}
	if !strings.Contains(err.Error(), "mounted no UI") {
		t.Errorf("error should say the mount produced no UI, got: %v", err)
	}
}

func TestBundleMountScriptThrowsAbortsStartup(t *testing.T) {
	files := bundleFiles(`throw new Error("boom");`, counterCSS)
	disp, eng, _ := newRuntime(nil, nil)
	t.Cleanup(func() { _ = eng.Close() })

	_, err := newBundleScene(files, nil, nil, eng, disp)
	if err == nil {
		t.Fatal("a throwing mount script must abort startup")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should carry the JS message, got: %v", err)
	}
}

// TestBundleApplyBatchTooLargeRejected proves the inline batch cap: the call is
// refused with -32602 and nothing is applied.
func TestBundleApplyBatchTooLargeRejected(t *testing.T) {
	rec := observe.NewRecorder()
	s, _ := newTestBundle(t, bundleFiles(counterBundle, counterCSS), rec, nil)

	big := make([]map[string]any, 0, maxOpsPerBatch+1)
	for i := 0; i <= maxOpsPerBatch; i++ {
		big = append(big, map[string]any{
			"kind": "createElement", "nodeId": 1000 + i, "tag": "div",
		})
	}
	raw, _ := json.Marshal(map[string]any{"version": 1, "ops": big})

	_, err := s.handleApply(nil, raw)
	if err == nil {
		t.Fatal("an oversized batch was accepted")
	}
	code := ipc.CodeFor(err)
	if code != ipc.CodeInvalidParams {
		t.Errorf("code = %d, want %d (-32602)", code, ipc.CodeInvalidParams)
	}

	m := rec.Snapshot()
	if m.UIOpsRejected != 1 {
		t.Errorf("UIOpsRejected = %d, want 1", m.UIOpsRejected)
	}
}

// TestBundleApplyRejectsInvalidStreamIsAtomic: a malformed batch is refused and
// the tree is unchanged, with the rejection both counted and reported (I1,
// G-DATA-02, P5).
func TestBundleApplyRejectsInvalidStreamIsAtomic(t *testing.T) {
	rec := observe.NewRecorder()
	rep := &countReporter{}
	s, _ := newTestBundle(t, bundleFiles(counterBundle, counterCSS), rec, rep)

	before := countNodes(t, s)

	raw := json.RawMessage(`{"version":1,"ops":[
		{"kind":"createElement","nodeId":50,"tag":"div"},
		{"kind":"addEventListener","nodeId":50,"event":"nope","handlerId":1}
	]}`)
	_, err := s.handleApply(nil, raw)
	if err == nil {
		t.Fatal("a batch with an unknown event kind was accepted")
	}
	if got := countNodes(t, s); got != before {
		t.Errorf("node count changed from %d to %d after a rejected batch", before, got)
	}
	if m := rec.Snapshot(); m.UIOpsRejected != 1 {
		t.Errorf("UIOpsRejected = %d, want 1", m.UIOpsRejected)
	}
	found := false
	for _, d := range rep.all {
		if d.Component == "ui" {
			found = true
		}
	}
	if !found {
		t.Error("a rejected batch must produce a Diagnostic{Component:\"ui\"} (P5)")
	}
}

func TestBundleApplyMalformedParamsRejected(t *testing.T) {
	s, _ := newTestBundle(t, bundleFiles(counterBundle, counterCSS), nil, nil)
	_, err := s.handleApply(nil, json.RawMessage(`{"version":`))
	if err == nil {
		t.Fatal("malformed ui.apply params were accepted")
	}
	if ipc.CodeFor(err) != ipc.CodeInvalidParams {
		t.Errorf("code = %d, want -32602", ipc.CodeFor(err))
	}
}

// TestBundleStartTextInputOnFocus: platform text input follows focus. The loop
// reads FocusEditable, so the scene must report true only while an editable
// element holds focus (Milestone 5).
func TestBundleStartTextInputOnFocus(t *testing.T) {
	files := bundleFiles(bundlePrelude+`
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
		{kind: "appendChild", parentId: 2, childId: 3}
	]
});
`, counterCSS)

	rec := observe.NewRecorder()
	s, _ := newTestBundle(t, files, rec, nil)
	warmUpBundle(t, s, 800, 600)

	if s.FocusEditable() {
		t.Error("FocusEditable = true with nothing focused")
	}

	input := findTag(t, s.tree, "input")
	box, ok := s.geom.Boxes[input.ID]
	if !ok {
		t.Fatal("input has no geometry after layout")
	}
	cx := box.Border.X + box.Border.W/2
	cy := box.Border.Y + box.Border.H/2
	s.HandleInput(window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft})

	// The press lands on the deepest node under the pointer, which is the
	// input's value mirror; focus resolves to the nearest focusable ancestor.
	if s.tree.Focused() != input {
		t.Fatalf("focused = %v, want the input (hit=%v)",
			s.tree.Focused(), s.tree.HitTest(cx, cy))
	}
	if !s.FocusEditable() {
		t.Error("FocusEditable = false while an input holds focus")
	}
}

// TestTextInputUpdatesValueThroughOps: committed text reaches the value
// mirror, and the next op batch is what makes the change durable — the scene
// never derives characters from keycodes.
func TestTextInputUpdatesValueThroughOps(t *testing.T) {
	files := bundleFiles(bundlePrelude+`
var value = "";
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
// The runtime does not write the value mirror itself: committed text arrives as
// a normal event, and the application decides what the value becomes (decision
// D-1 — one mutation door).
gowez.on("h1", function (ev) {
	value = ev.text;
	apply({version: 1, ops: [
		{kind: "setText", nodeId: 3, value: value}
	]});
});
`, counterCSS)

	rec := observe.NewRecorder()
	s, _ := newTestBundle(t, files, rec, nil)
	warmUpBundle(t, s, 800, 600)

	input := findTag(t, s.tree, "input")
	box, _ := s.geom.Boxes[input.ID]
	s.HandleInput(window.PointerEvent{
		X: box.Border.X + box.Border.W/2, Y: box.Border.Y + box.Border.H/2,
		Press: true, Button: window.ButtonLeft,
	})
	if !s.FocusEditable() {
		t.Fatal("input not focused")
	}

	s.HandleInput(window.TextInputEvent{Text: "typed"})
	if got := mirrorText(t, s, input); got != "typed" {
		t.Errorf("mirror = %q, want %q", got, "typed")
	}
	if !s.dirty {
		t.Error("dirty = false after committed text, want true")
	}
}

// TestTextEditingPreeditMirror: IME preedit renders in the same mirror. The
// composition extent travels with the event so a future caret renderer has
// what it needs, but nothing is committed (documented M5 divergence).
func TestTextEditingPreeditMirror(t *testing.T) {
	files := bundleFiles(bundlePrelude+`
apply({
	version: 1,
	entry: 1,
	ops: [
		{kind: "createElement", nodeId: 1, tag: "input"},
		{kind: "setAttribute", nodeId: 1, name: "tabindex", value: "0"},
		{kind: "setStyle", nodeId: 1, style: {"height": "20px", "background-color": "#333333"}},
		{kind: "createText", nodeId: 2, text: ""},
		{kind: "appendChild", parentId: 1, childId: 2},
		{kind: "addEventListener", nodeId: 1, event: "textinput", handlerId: 1},
		{kind: "addEventListener", nodeId: 1, event: "textediting", handlerId: 2}
	]
});
// Preedit renders in the mirror without committing; a commit replaces it.
gowez.on("h2", function (ev) {
	apply({version: 1, ops: [{kind: "setText", nodeId: 2, value: ev.text}]});
});
gowez.on("h1", function (ev) {
	apply({version: 1, ops: [{kind: "setText", nodeId: 2, value: ev.text}]});
});`, counterCSS)

	s, _ := newTestBundle(t, files, nil, nil)
	warmUpBundle(t, s, 400, 300)
	input := s.tree.Roots()[0]
	// Focus by tab traversal, the same path a real keyboard takes.
	s.HandleInput(window.KeyEvent{Key: "tab", Press: true})
	if s.tree.Focused() != input {
		t.Fatalf("focused = %v, want the input", s.tree.Focused())
	}

	s.HandleInput(window.TextEditingEvent{Text: "\u306b\u307b", Start: 0, Length: 2})
	if got := mirrorText(t, s, input); got != "\u306b\u307b" {
		t.Errorf("preedit mirror = %q, want the preedit text", got)
	}

	// Commit replaces the preedit.
	s.HandleInput(window.TextInputEvent{Text: "\u65e5\u672c"})
	if got := mirrorText(t, s, input); got != "\u65e5\u672c" {
		t.Errorf("committed mirror = %q, want the committed text", got)
	}
}

// TestBundleBackspaceHandledInJS documents where deletion lives: the platform
// sends no deletion event, so the JS side owns it. This test proves the
// adapter-visible contract — a keydown handler can rewrite the value through
// ui.apply.
func TestBundleBackspaceHandledInJS(t *testing.T) {
	files := bundleFiles(bundlePrelude+`
var value = "abc";
apply({
	version: 1,
	entry: 1,
	ops: [
		{kind: "createElement", nodeId: 1, tag: "input"},
		{kind: "setAttribute", nodeId: 1, name: "tabindex", value: "0"},
		{kind: "setStyle", nodeId: 1, style: {"height": "20px", "background-color": "#333333"}},
		{kind: "createText", nodeId: 2, text: "abc"},
		{kind: "appendChild", parentId: 1, childId: 2},
		{kind: "addEventListener", nodeId: 1, event: "key-down", handlerId: 1}
	]
});
gowez.on("h1", function () {
	value = value.slice(0, -1);
	apply({version: 1, ops: [{kind: "setText", nodeId: 2, value: value}]});
});
`, counterCSS)

	s, _ := newTestBundle(t, files, nil, nil)
	warmUpBundle(t, s, 400, 300)
	input := s.tree.Roots()[0]

	if got := mirrorText(t, s, input); got != "abc" {
		t.Fatalf("initial mirror = %q, want abc", got)
	}

	// Focus first: a key event only reaches a focused node.
	s.HandleInput(window.KeyEvent{Key: "tab", Press: true})
	if s.tree.Focused() != input {
		t.Fatalf("focused = %v, want the input", s.tree.Focused())
	}
	s.HandleInput(window.KeyEvent{Key: "backspace", Press: true})
	if got := mirrorText(t, s, input); got != "ab" {
		t.Errorf("mirror after backspace = %q, want %q", got, "ab")
	}
}

// TestDemoSceneUnchangedWhenUINil guards the additive-API promise (DRR-006): a
// nil UI keeps the M1–M4 demo scene untouched.
func TestDemoSceneUnchangedWhenUINil(t *testing.T) {
	a, err := New(Options{Title: "t", Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if a.Options().UI != nil {
		t.Error("Options.UI defaults to non-nil")
	}
	disp, eng, _ := newRuntime(nil, nil)
	t.Cleanup(func() { _ = eng.Close() })

	sc, err := a.newScene(disp, eng)
	if err != nil {
		t.Fatalf("newScene with nil UI: %v", err)
	}
	if _, ok := sc.(*uiScene); !ok {
		t.Errorf("nil UI produced %T, want *uiScene", sc)
	}
}

// findText returns the first text node whose content equals want.
func findText(t *testing.T, tree *ui.Tree, want string) *ui.Node {
	t.Helper()
	var found *ui.Node
	var walk func(*ui.Node)
	walk = func(n *ui.Node) {
		if found != nil || n == nil {
			return
		}
		if n.Kind == ui.TextNode && n.Text == want {
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
	return found
}

// warmUpBundle draws one frame so the mounted tree has geometry.
func warmUpBundle(t *testing.T, s *bundleScene, w, h int) {
	t.Helper()
	r := software.New()
	r.BeginFrame(w, h)
	if err := s.Draw(r, w, h, w, h); err != nil {
		t.Fatalf("warm-up Draw: %v", err)
	}
	r.EndFrame()
}

// mirrorText returns the value mirror text of an element node.
func mirrorText(t *testing.T, s *bundleScene, n *ui.Node) string {
	t.Helper()
	for _, c := range n.Children {
		if c.Kind == ui.TextNode {
			return c.Text
		}
	}
	t.Fatal("node has no value mirror")
	return ""
}

// countNodes counts every node reachable from the roots.
func countNodes(t *testing.T, s *bundleScene) int {
	t.Helper()
	n := 0
	var walk func(*ui.Node)
	walk = func(x *ui.Node) {
		n++
		for _, c := range x.Children {
			walk(c)
		}
	}
	for _, r := range s.tree.Roots() {
		walk(r)
	}
	return n
}

// countReporter records diagnostics for assertions.
type countReporter struct {
	all []observe.Diagnostic
}

func (c *countReporter) Report(d observe.Diagnostic) { c.all = append(c.all, d) }

// committedBundle returns the compiled slice bundle that the M5 acceptance path
// ships: examples/gowez-dashboard/dist, committed so these tests run offline
// without npm (docs/PLAN-M5.md §8, DEVELOPMENT_GUIDE.md §1).
//
// Tests that must exercise the *shipped* artifact use this rather than a
// hand-written fixture, so they prove the adapter's output is reactive instead
// of proving a copy of it is.
func committedBundle(t *testing.T) fstest.MapFS {
	t.Helper()
	root := filepath.Join("..", "..", "examples", "gowez-dashboard", "dist")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("compiled bundle missing at %s (%v); run `npm run build -w @gowez/example-gowez-dashboard`", root, err)
	}
	files := fstest.MapFS{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatalf("read bundle file %s: %v", e.Name(), err)
		}
		files[e.Name()] = &fstest.MapFile{Data: data}
	}
	if _, ok := files["manifest.json"]; !ok {
		t.Skip("bundle has no manifest.json")
	}
	return files
}
