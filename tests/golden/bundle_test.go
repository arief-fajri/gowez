// Package golden drives the real bundle path: the compiled Svelte slice is
// loaded, mounted through the sandbox, styled, laid out and painted. It exists
// because the M5 gate needs pixel evidence that a Svelte-authored UI renders
// through the runtime's own pipeline — no browser, no WebView.
//
// The test imports internal packages directly rather than going through
// internal/app, because the frame loop there opens a window. Everything below
// the window layer is the same code either way.
package golden

import (
	"context"
	"encoding/json"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/arief-fajri/gowez/internal/assets"
	"github.com/arief-fajri/gowez/internal/ipc"
	"github.com/arief-fajri/gowez/internal/layout"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/paint"
	"github.com/arief-fajri/gowez/internal/permission"
	"github.com/arief-fajri/gowez/internal/render"
	"github.com/arief-fajri/gowez/internal/render/backend/software"
	"github.com/arief-fajri/gowez/internal/script"
	"github.com/arief-fajri/gowez/internal/style"
	"github.com/arief-fajri/gowez/internal/ui"
)

// bundlePath is the committed slice bundle. It is committed so the golden and
// integration tests run offline without npm (docs/PLAN-M5 §8).
var bundlePath = filepath.Join("..", "..", "examples", "gowez-dashboard", "dist")

// mounted is a tree built by the real bundle path, ready to lay out and paint.
type mounted struct {
	tree  *ui.Tree
	sheet *style.Stylesheet
	eng   script.Engine
	disp  *ipc.Dispatcher
	rec   *observe.Recorder
}

// loadBundle mounts the compiled slice. It fails the test when the bundle is
// missing rather than rendering an empty frame, which would be a silent
// pass (P4).
func loadBundle(t *testing.T, root string) *mounted {
	t.Helper()

	if _, err := os.Stat(filepath.Join(root, "manifest.json")); err != nil {
		t.Skipf("compiled bundle missing at %s (%v); run `npm run build -w @gowez/example-gowez-dashboard`", root, err)
	}

	files := os.DirFS(root)
	loader := assets.FromFS(files)

	raw, err := loader.Load("manifest.json")
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	var mf struct {
		SchemaVersion int    `json:"schemaVersion"`
		Svelte        int    `json:"svelte"`
		Script        string `json:"script"`
		Styles        string `json:"styles"`
	}
	if err := json.Unmarshal(raw, &mf); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if mf.SchemaVersion != 1 {
		t.Fatalf("manifest schemaVersion = %d, want 1", mf.SchemaVersion)
	}
	if mf.Svelte != 5 {
		t.Fatalf("manifest svelte = %d, want 5", mf.Svelte)
	}

	cssRaw, err := loader.Load(mf.Styles)
	if err != nil {
		t.Fatalf("load styles: %v", err)
	}
	sheet, err := style.Parse(string(cssRaw))
	if err != nil {
		t.Fatalf("parse styles: %v", err)
	}

	scriptRaw, err := loader.Load(mf.Script)
	if err != nil {
		t.Fatalf("load script: %v", err)
	}

	rec := observe.NewRecorder()
	disp := ipc.NewDispatcher()
	disp.SetObservation(rec, nil)
	disp.SetGrants(permission.NewSet())

	eng, err := script.New(script.DefaultLimits, disp)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	eng.SetObservation(rec, nil)

	tree := ui.NewTree()
	session := ui.NewSession(tree)

	// The single mutation door (decision D-1): ui.apply, and nothing else on
	// the bundle path.
	var lastErr error
	if err := disp.RegisterInline("ui.apply", "", func(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
		var batch ui.Batch
		if err := json.Unmarshal(params, &batch); err != nil {
			lastErr = err
			return nil, ipc.NewCodeError(ipc.CodeInvalidParams, err.Error())
		}
		res, err := session.Apply(&batch)
		if err != nil {
			lastErr = err
			return nil, ipc.NewCodeError(ipc.CodeInvalidParams, err.Error())
		}
		for _, b := range res.Bindings {
			n, ok := session.Node(b.NodeID)
			if !ok {
				continue
			}
			_ = tree.SetListenerHandler(n, b.Listener, func(*ui.Event) {})
		}
		rec.RecordUIBatch(len(batch.Ops), false)
		return json.RawMessage(`true`), nil
	}); err != nil {
		t.Fatalf("register ui.apply: %v", err)
	}

	if err := eng.Eval(mf.Script, string(scriptRaw)); err != nil {
		t.Fatalf("mount bundle: %v", err)
	}
	if lastErr != nil {
		t.Fatalf("ui.apply during mount: %v", lastErr)
	}
	if len(tree.Roots()) == 0 {
		t.Fatal("bundle mounted no UI")
	}

	return &mounted{tree: tree, sheet: sheet, eng: eng, disp: disp, rec: rec}
}

// frame is one painted frame in raw RGBA form.
type frame struct {
	px []byte
	w  int
	h  int
}

// render paints the mounted tree at the given size.
func (m *mounted) render(t *testing.T, w, h int) frame {
	t.Helper()
	styles, err := style.Resolve(m.tree.Roots(), m.sheet)
	if err != nil {
		t.Fatalf("style resolve: %v", err)
	}
	geom, err := layout.Layout(m.tree.Roots(), styles, ui.Size{W: float64(w), H: float64(h)})
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	m.tree.SetGeometry(geom.Boxes)

	r := software.New()
	r.BeginFrame(w, h)
	r.DrawRect(0, 0, float64(w), float64(h), render.Color{R: 0.08, G: 0.086, B: 0.11, A: 1})
	if err := paint.Draw(r, m.tree.Roots(), geom, styles, 1); err != nil {
		t.Fatalf("paint: %v", err)
	}
	r.EndFrame()

	px, err := r.Pixels()
	if err != nil {
		t.Fatalf("Pixels: %v", err)
	}
	fw, fh := r.Size()
	return frame{px: px, w: fw, h: fh}
}

// imageOf wraps a raw RGBA frame for the golden comparison.
func imageOf(px []byte, w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, px)
	return img
}

// assertHasTag fails when the mounted tree contains no such element.
func assertHasTag(t *testing.T, m *mounted, tag string) {
	t.Helper()
	var walk func(*ui.Node) bool
	walk = func(n *ui.Node) bool {
		if n == nil {
			return false
		}
		if n.Kind == ui.ElementNode && n.Tag == tag {
			return true
		}
		for _, c := range n.Children {
			if walk(c) {
				return true
			}
		}
		return false
	}
	for _, r := range m.tree.Roots() {
		if walk(r) {
			return
		}
	}
	t.Errorf("mounted tree contains no <%s>", tag)
}

// assertHasText fails when no text node carries the given string.
func assertHasText(t *testing.T, m *mounted, want string) {
	t.Helper()
	var walk func(*ui.Node) bool
	walk = func(n *ui.Node) bool {
		if n == nil {
			return false
		}
		if n.Kind == ui.TextNode && strings.Contains(n.Text, want) {
			return true
		}
		for _, c := range n.Children {
			if walk(c) {
				return true
			}
		}
		return false
	}
	for _, r := range m.tree.Roots() {
		if walk(r) {
			return
		}
	}
	t.Errorf("mounted tree has no text containing %q", want)
}

// bundleExists reports whether the committed slice bundle is present.
func bundleExists() bool {
	_, err := os.Stat(filepath.Join(bundlePath, "manifest.json"))
	return err == nil
}

// bundleFS opens the committed bundle, or skips when it is absent.
func bundleFS(t *testing.T) fs.FS {
	t.Helper()
	if !bundleExists() {
		t.Skip("compiled bundle not present; run the adapter build first")
	}
	return os.DirFS(bundlePath)
}

// TestGowezDashboardMounts proves the compiled Svelte slice mounts through the
// real runtime: manifest → CSS → sandbox eval → ui.apply → a laid-out tree.
// This is the "Svelte UI loads" MVP box and the first M5 gate item.
func TestGowezDashboardMounts(t *testing.T) {
	m := loadBundle(t, bundlePath)

	if got := len(m.tree.Roots()); got != 1 {
		t.Fatalf("roots = %d, want 1", got)
	}

	metrics := m.rec.Snapshot()
	if metrics.UIOpBatches == 0 {
		t.Error("no UI instruction batch was applied")
	}
	if metrics.UIOpsRejected != 0 {
		t.Errorf("UIOpsRejected = %d, want 0", metrics.UIOpsRejected)
	}

	// The slice declares its structure through markup, so the mounted tree must
	// actually contain it rather than an empty root.
	assertHasTag(t, m, "section")
	assertHasTag(t, m, "header")
	assertHasTag(t, m, "button")
	assertHasTag(t, m, "input")
	assertHasText(t, m, "GoWEZ")
}

// TestGowezDashboardRendersGolden pins the pixels.
//
// What this proves is the *pipeline*, not layout fidelity: a Svelte-authored UI
// goes Svelte → adapter → bundle → sandbox → ui.apply → style → layout → paint
// → pixels, and the same inputs must come out byte-identically every run
// (Module 6 determinism). A byte-exact golden catches any change in that chain.
//
// It is deliberately NOT an acceptance criterion for layout quality. The slice
// renders inside the M5 subset, which has no inline flow, no percentage
// heights, no margin collapsing and no CSS Grid, so the pinned image looks
// wrong in ways the subset cannot yet express. Regenerating this file after a
// capability lands is an improvement, not a regression — the gap register
// (packages/adapter report) is the metric that tracks what is still missing.
func TestGowezDashboardRendersGolden(t *testing.T) {
	m := loadBundle(t, bundlePath)

	f := m.render(t, 900, 640)
	if len(f.px) != f.w*f.h*4 {
		t.Fatalf("pixel buffer %d bytes, want %d", len(f.px), f.w*f.h*4)
	}

	checkGolden(t, "gowez-dashboard", imageOf(f.px, f.w, f.h))
}

// TestGowezDashboardDeterminism renders the same bundle twice and requires
// byte-identical output — the core determinism criterion for a path that mixes
// JavaScript, a stylesheet and layout.
func TestGowezDashboardDeterminism(t *testing.T) {
	renderTwice := func() []byte {
		m := loadBundle(t, bundlePath)
		return m.render(t, 640, 480).px
	}

	first := renderTwice()
	second := renderTwice()
	if len(first) != len(second) {
		t.Fatalf("frame sizes differ: %d vs %d bytes", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("frame differs at byte %d (%d vs %d): rendering is not deterministic", i, first[i], second[i])
		}
	}
}

// TestGowezDashboardCSSResolves proves the emitted stylesheet is one the Go
// subset understands: every declaration resolved and the tree got geometry.
func TestGowezDashboardCSSResolves(t *testing.T) {
	m := loadBundle(t, bundlePath)

	styles, err := style.Resolve(m.tree.Roots(), m.sheet)
	if err != nil {
		t.Fatalf("style resolve: %v", err)
	}
	geom, err := layout.Layout(m.tree.Roots(), styles, ui.Size{W: 900, H: 640})
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	if len(geom.Boxes) == 0 {
		t.Fatal("layout produced no boxes")
	}
	if len(m.sheet.Rules) == 0 {
		t.Fatal("stylesheet has no rules")
	}
}

// TestBundleStylesAreScoped proves component style scoping reaches the runtime.
//
// The adapter stamps a scope class on every rule's subject and on every element
// it renders. Without it, a rule declared in one component matches elements
// rendered by another: the sample declared `button` in both App and UserList,
// so one rule silently overrode the other. This test reads the committed bundle
// through internal/style and asserts each button resolves from *its own*
// module's rule, which is the only place scoping can actually be verified —
// asserting on the CSS text would only prove the adapter wrote the class.
func TestBundleStylesAreScoped(t *testing.T) {
	m := loadBundle(t, bundlePath)

	type want struct {
		scope   string
		padding [4]float64
	}
	// padding values taken from the sample's own source: App's button is
	// `padding: 6px 10px`, UserList's is `padding: 4px 8px`.
	wants := []want{
		{scope: "s-UserList", padding: [4]float64{4, 8, 4, 8}},
		{scope: "s-App", padding: [4]float64{6, 10, 6, 10}},
	}

	styles, err := style.Resolve(m.tree.Roots(), m.sheet)
	if err != nil {
		t.Fatalf("style resolve: %v", err)
	}

	// Group the mounted buttons by the scope class they carry.
	byScope := map[string][]ui.NodeID{}
	var walk func(*ui.Node)
	walk = func(n *ui.Node) {
		if n == nil {
			return
		}
		if n.Kind == ui.ElementNode && n.Tag == "button" {
			for _, c := range strings.Fields(n.GetAttribute("class")) {
				if strings.HasPrefix(c, "s-") {
					byScope[c] = append(byScope[c], n.ID)
				}
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range m.tree.Roots() {
		walk(r)
	}

	for _, w := range wants {
		ids := byScope[w.scope]
		if len(ids) == 0 {
			t.Errorf("no <button> carries the scope class %q; scoping did not reach the tree", w.scope)
			continue
		}
		for _, id := range ids {
			if got := styles[id].Padding; got != w.padding {
				t.Errorf("button %d: padding = %v, want %v (a rule from another module won)", id, got, w.padding)
			}
		}
	}

	// Both scopes must be present, otherwise the assertion above proved nothing.
	if len(byScope) < 2 {
		t.Errorf("expected buttons from at least 2 modules, found scopes %v", keysOf(byScope))
	}
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
