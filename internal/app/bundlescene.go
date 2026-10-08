package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/arief-fajri/gowez/internal/assets"
	"github.com/arief-fajri/gowez/internal/ipc"
	"github.com/arief-fajri/gowez/internal/layout"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/paint"
	"github.com/arief-fajri/gowez/internal/render"
	"github.com/arief-fajri/gowez/internal/script"
	"github.com/arief-fajri/gowez/internal/style"
	"github.com/arief-fajri/gowez/internal/ui"
	"github.com/arief-fajri/gowez/internal/window"
)

// BundleManifestSchemaVersion is the manifest layout this runtime speaks.
// A bundle built for a different value is refused explicitly (P4, DRR-006).
const BundleManifestSchemaVersion = 1

// bundleSvelteRange is the supported Svelte major.minor range, advertised in
// the manifest as a major version and checked here. The adapter embeds the
// range it was built against; a mismatch is a startup failure rather than a
// runtime surprise (G-IFACE-05, G-UPG-02).
const bundleSvelteMajor = 5

// maxOpsPerBatch bounds one ui.apply call. The applier is synchronous and
// inline (it runs on the UI goroutine inside the caller's JS turn), so an
// unbounded batch would stall a frame; the limit is reported to JS as
// -32602 rather than truncating silently (G-REL-01, P4).
const maxOpsPerBatch = 2000

// manifest is the bundle contract (DRR-006).
type manifest struct {
	SchemaVersion int `json:"schemaVersion"`
	Adapter       struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"adapter"`
	// Svelte is the Svelte major version the adapter compiled against.
	Svelte int    `json:"svelte"`
	Script string `json:"script"`
	Styles string `json:"styles"`
}

// bundleScene mounts a compiled Svelte UI. It owns a tree the JavaScript side
// builds by submitting instruction batches through the single registered
// method ui.apply, then styles, lays out, and paints it through the ordinary
// pipeline.
//
// One door, deliberately: ui.setText belongs to the demo scene and is not
// registered here, so every mutation of a mounted bundle is observable as an
// instruction batch (guard rail G-SEC-01).
type bundleScene struct {
	tree   *ui.Tree
	sheet  *style.Stylesheet
	styles map[ui.NodeID]style.ComputedStyle
	geom   *layout.Result

	// session applies instruction batches and carries the adapter-id map and
	// the listener registrations across them, so an update batch can address
	// the nodes the mount batch created.
	session *ui.Session

	metrics  *observe.Recorder
	reporter observe.Reporter
	eng      script.Engine

	// manifest of the mounted bundle, kept for diagnostics.
	mf manifest

	dirty        bool
	lastW, lastH int

	// editable reports whether the focused node accepts text input. The
	// window layer consults it to start or stop platform text input
	// (Milestone 5 text input / IME).
	editable bool
}

// FocusNode returns the node holding keyboard focus, or nil.
func (s *bundleScene) FocusNode() *ui.Node { return s.tree.Focused() }

// FocusEditable reports whether the focused node accepts text input. The frame
// loop uses it to start or stop platform text input (Milestone 5).
func (s *bundleScene) FocusEditable() bool { return s.editable }

// SetText writes committed text to the focused node's value mirror (the text
// child the adapter created for <input>). Preedit arrives the same way, so a
// single path serves both TextInput and TextEditing.
func (s *bundleScene) SetText(text string) {
	n := s.tree.Focused()
	if n == nil {
		return
	}
	if s.writeValue(n, text) {
		s.dirty = true
	}
}

// writeValue replaces the node's mirror text, reporting whether anything
// changed. The mirror is the first text child, which is the element the
// adapter created for the input's value.
func (s *bundleScene) writeValue(n *ui.Node, text string) bool {
	for _, c := range n.Children {
		if c.Kind == ui.TextNode {
			if c.Text == text {
				return false
			}
			c.Text = text
			return true
		}
	}
	return false
}

// newBundleScene validates the manifest and stylesheet, registers ui.apply, and
// returns a scene with an empty tree ready for the mount batch.
func newBundleScene(
	files fs.FS,
	metrics *observe.Recorder,
	reporter observe.Reporter,
	eng script.Engine,
	disp *ipc.Dispatcher,
) (*bundleScene, error) {
	if files == nil {
		return nil, errors.New("bundle: nil filesystem")
	}
	if eng == nil {
		return nil, errors.New("bundle: nil script engine")
	}
	if disp == nil {
		return nil, errors.New("bundle: nil ipc dispatcher")
	}
	loader := assets.FromFS(files)

	raw, err := loader.Load("manifest.json")
	if err != nil {
		return nil, fmt.Errorf("bundle: manifest.json: %w", err)
	}
	var mf manifest
	if err := json.Unmarshal(raw, &mf); err != nil {
		return nil, fmt.Errorf("bundle: manifest.json: %w", err)
	}
	if mf.SchemaVersion != BundleManifestSchemaVersion {
		return nil, fmt.Errorf("bundle: manifest schemaVersion %d, want %d", mf.SchemaVersion, BundleManifestSchemaVersion)
	}
	if mf.Svelte != bundleSvelteMajor {
		return nil, fmt.Errorf("bundle: manifest svelte major %d, want %d", mf.Svelte, bundleSvelteMajor)
	}
	if mf.Script == "" || mf.Styles == "" {
		return nil, fmt.Errorf("bundle: manifest must name both script and styles (got script=%q styles=%q)", mf.Script, mf.Styles)
	}

	cssRaw, err := loader.Load(mf.Styles)
	if err != nil {
		return nil, fmt.Errorf("bundle: %s: %w", mf.Styles, err)
	}
	sheet, err := style.Parse(string(cssRaw))
	if err != nil {
		// style.Parse errors already carry "style: line X:Y" (CSS-SUBSET
		// error contract), so the bundle prefix is enough context.
		return nil, fmt.Errorf("bundle: %s: %w", mf.Styles, err)
	}

	scriptRaw, err := loader.Load(mf.Script)
	if err != nil {
		return nil, fmt.Errorf("bundle: %s: %w", mf.Script, err)
	}

	tree := ui.NewTree()
	s := &bundleScene{
		tree:     tree,
		session:  ui.NewSession(tree),
		sheet:    sheet,
		metrics:  metrics,
		reporter: reporter,
		eng:      eng,
		mf:       mf,
		dirty:    true,
	}

	if err := disp.RegisterInline("ui.apply", "", s.handleApply); err != nil {
		return nil, fmt.Errorf("bundle: register ui.apply: %w", err)
	}

	// Mount: the script submits its initial instruction batch through
	// ui.apply, so the tree exists before the first layout. A mount that
	// throws or times out aborts startup explicitly (I12, G-REL-02).
	if err := s.eng.Eval(mf.Script, string(scriptRaw)); err != nil {
		return nil, fmt.Errorf("bundle: %s: %w", mf.Script, err)
	}
	if len(s.tree.Roots()) == 0 {
		return nil, fmt.Errorf("bundle: %s mounted no UI (no ui.apply batch produced a root)", mf.Script)
	}
	return s, nil
}

// handleApply is the bundle path's only UI mutation door. It is registered
// inline, so it runs on the UI goroutine that called gowez.invoke — the tree
// is single-goroutine and must not be mutated from anywhere else.
//
// A rejected batch leaves the tree untouched (I1, G-DATA-02), is counted in
// UIOpsRejected, and produces a Diagnostic{Component:"ui"} (P5).
func (s *bundleScene) handleApply(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	_ = ctx

	var batch ui.Batch
	if err := json.Unmarshal(params, &batch); err != nil {
		s.recordBatch(0, true, fmt.Sprintf("ui.apply: malformed params: %v", err))
		return nil, ipc.NewCodeError(ipc.CodeInvalidParams, fmt.Sprintf("ui.apply: malformed params: %v", err))
	}
	if len(batch.Ops) > maxOpsPerBatch {
		err := fmt.Errorf("ui.apply: batch of %d ops exceeds the limit of %d", len(batch.Ops), maxOpsPerBatch)
		s.recordBatch(0, true, err.Error())
		return nil, ipc.NewCodeError(ipc.CodeInvalidParams, err.Error())
	}

	res, err := s.session.Apply(&batch)
	if err != nil {
		s.recordBatch(0, true, err.Error())
		return nil, ipc.NewCodeError(ipc.CodeInvalidParams, err.Error())
	}

	// Wire each new subscription to the script layer: the applier registered
	// a placeholder because it does not know what should run.
	for _, b := range res.Bindings {
		n, ok := s.session.Node(b.NodeID)
		if !ok {
			continue
		}
		if !s.tree.SetListenerHandler(n, b.Listener, s.handlerFor(b.HandlerID)) {
			// Wiring failed: the listener would silently stay a placeholder and
			// every interaction with this node would do nothing (I6/P4).
			s.recordBatch(0, true, fmt.Sprintf("ui.apply: cannot bind handler %d on node %d", b.HandlerID, n.ID))
		}
	}

	s.recordBatch(len(batch.Ops), false, "")
	s.dirty = true
	return json.RawMessage(`true`), nil
}

// handlerFor builds the Go-side listener for one adapter handler id. The
// handlerId → "h<id>" convention is documented in docs/SVELTE.md.
//
// The payload carries the event itself, not just the handler id: a text input
// handler must receive the committed text, and a click handler the pointer
// position, or every handler in the application would have to look state up.
func (s *bundleScene) handlerFor(handlerID int) ui.Handler {
	return func(e *ui.Event) {
		payload, err := json.Marshal(map[string]any{
			"kind":     e.Kind.String(),
			"handler":  handlerID,
			"key":      e.Key,
			"text":     e.Text,
			"start":    e.TextStart,
			"length":   e.TextLength,
			"x":        e.X,
			"y":        e.Y,
			"button":   e.Button,
			"modifier": e.Modifier,
		})
		if err != nil {
			if s.reporter != nil {
				s.reporter.Report(observe.Diagnostic{
					Component: "ui",
					Message:   fmt.Sprintf("handler %d: cannot encode event: %v", handlerID, err),
				})
			}
			return
		}
		// FireHandler failures are already observed by the engine
		// (diagnostic + JSExceptions); a listener has nothing left to report
		// and must not block the frame (docs/EVENTS.md §handler contract).
		_ = s.eng.FireHandler(fmt.Sprintf("h%d", handlerID), payload)
	}
}

// recordBatch folds one batch outcome into the metrics and, when the batch was
// refused, into the diagnostic stream.
func (s *bundleScene) recordBatch(applied int, rejected bool, message string) {
	if s.metrics != nil {
		s.metrics.RecordUIBatch(applied, rejected)
	}
	if rejected && s.reporter != nil {
		s.reporter.Report(observe.Diagnostic{
			Component: "ui",
			Message:   message,
		})
	}
}

// Tick advances per-frame state. The bundle scene has no frame counter of its
// own: the mount is one-shot, and further repaints are driven by op batches.
func (s *bundleScene) Tick() {}

// HandleInput feeds a window event into the mounted tree, keeping the
// interaction bookkeeping (dirty flag, metrics, focus-driven text input) in
// step with the demo scene so both paths share one input contract.
func (s *bundleScene) HandleInput(ev window.Event) {
	var res ui.InputResult
	switch e := ev.(type) {
	case window.PointerEvent:
		switch {
		case e.Button != 0 && e.Press:
			res = s.tree.PointerDown(e.X, e.Y, e.Button)
		case e.Button != 0:
			res = s.tree.PointerUp(e.X, e.Y, e.Button)
		default:
			res = s.tree.PointerMove(e.X, e.Y)
		}
	case window.KeyEvent:
		if e.Press {
			res = s.tree.KeyDown(e.Key, e.Modifier)
		} else {
			res = s.tree.KeyUp(e.Key, e.Modifier)
		}
	case window.TextInputEvent:
		res = s.tree.TextInput(e.Text)
	case window.TextEditingEvent:
		res = s.tree.TextEditing(e.Text, e.Start, e.Length)
	default:
		return
	}
	if res.Changed {
		s.dirty = true
	}
	s.editable = s.focusIsEditable()
	if s.metrics != nil {
		s.metrics.RecordInput(res.Dispatched, res.Panics)
	}
}

// focusIsEditable reports whether the focused node is an editable element.
func (s *bundleScene) focusIsEditable() bool {
	n := s.tree.Focused()
	return n != nil && n.Kind == ui.ElementNode && n.Tag == "input"
}

// relayout re-resolves styles and recomputes geometry, publishing geometry to
// the tree only after both passes succeed (I1: no half-applied update).
func (s *bundleScene) relayout(w, h int) error {
	styles, err := style.Resolve(s.tree.Roots(), s.sheet)
	if err != nil {
		return fmt.Errorf("bundle: style resolve: %w", err)
	}
	geom, err := layout.Layout(s.tree.Roots(), styles, ui.Size{W: float64(w), H: float64(h)})
	if err != nil {
		return fmt.Errorf("bundle: layout: %w", err)
	}
	s.styles, s.geom = styles, geom
	s.tree.SetGeometry(geom.Boxes)
	s.lastW, s.lastH = w, h
	s.dirty = false
	return nil
}

// Draw paints one frame: the scene backdrop, then the tree.
func (s *bundleScene) Draw(r render.Renderer, pw, ph, logicalW, logicalH int) error {
	if logicalW <= 0 || logicalH <= 0 || pw <= 0 || ph <= 0 {
		return fmt.Errorf("bundle: invalid frame size %dx%d (logical %dx%d)", pw, ph, logicalW, logicalH)
	}
	if s.dirty || logicalW != s.lastW || logicalH != s.lastH {
		if err := s.relayout(logicalW, logicalH); err != nil {
			return err
		}
	}
	scale := float64(pw) / float64(logicalW)
	r.DrawRect(0, 0, float64(pw), float64(ph), sceneBackground)
	if err := paint.Draw(r, s.tree.Roots(), s.geom, s.styles, scale); err != nil {
		return fmt.Errorf("bundle: paint: %w", err)
	}
	return nil
}

// SetFocusEditable is called by the input layer when the focused node changes;
// it starts or stops platform text input for editable nodes (Milestone 5).
func (s *bundleScene) SetFocusEditable(editable bool) {
	s.editable = editable
}

// ui.applyParams is the payload shape the adapter emits. It is an alias so the
// method name in the docs matches the one in the runtime.
type applyParams = ui.Batch
