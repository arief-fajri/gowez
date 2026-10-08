package app

import (
	"testing"

	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/render/backend/software"
	"github.com/arief-fajri/gowez/internal/ui"
	"github.com/arief-fajri/gowez/internal/window"
)

// This file is the M5 reactivity gate.
//
// It deliberately mounts the **committed slice bundle** — the artifact the
// adapter actually produced — rather than a hand-written fixture. Copying the
// runtime prelude into a test would prove only that a copy works; testing the
// shipped bundle proves the pipeline emits a reactive program, which is the
// thing the checklist asks for.
//
// What is proven, all through the real path (manifest → CSS → sandbox eval →
// ui.apply → layout):
//
//   - {#if} renders exactly one arm, and it flips when state flips
//   - keyed {#each} renders one node per item and removes only the removed one
//   - $state + a click re-renders and submits a *diff*, not a remount
//   - $derived recomputes on each render
//   - text input reaches the value and flows back into state

// sliceScene mounts the committed bundle, exactly as startup does.
func sliceScene(t *testing.T) (*bundleScene, *observe.Recorder) {
	t.Helper()
	rec := observe.NewRecorder()
	s, _ := newTestBundle(t, committedBundle(t), rec, nil)
	warmUpBundle(t, s, 900, 640)
	return s, rec
}

// textOfClass returns the text of the element carrying the given class.
func textOfClass(t *testing.T, s *bundleScene, class string) string {
	t.Helper()
	var found string
	var walk func(*ui.Node)
	walk = func(n *ui.Node) {
		if n == nil || found != "" {
			return
		}
		if n.Kind == ui.ElementNode && n.HasClass(class) {
			for _, c := range n.Children {
				if c.Kind == ui.TextNode && c.Text != "" {
					found = c.Text
					return
				}
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range s.tree.Roots() {
		walk(r)
	}
	if found == "" {
		t.Fatalf("no element with class %q", class)
	}
	return found
}

// classCount counts elements carrying a class — the list length.
func classCount(s *bundleScene, class string) int {
	n := 0
	var walk func(*ui.Node)
	walk = func(x *ui.Node) {
		if x == nil {
			return
		}
		if x.Kind == ui.ElementNode && x.HasClass(class) {
			n++
		}
		for _, c := range x.Children {
			walk(c)
		}
	}
	for _, r := range s.tree.Roots() {
		walk(r)
	}
	return n
}

// clickElement dispatches a full press+release at an element's centre.
func clickElement(t *testing.T, s *bundleScene, n *ui.Node) {
	t.Helper()
	box, ok := s.geom.Boxes[n.ID]
	if !ok {
		t.Fatalf("node %d has no geometry after layout", n.ID)
	}
	cx := box.Border.X + box.Border.W/2
	cy := box.Border.Y + box.Border.H/2
	s.HandleInput(window.PointerEvent{X: cx, Y: cy, Press: true, Button: window.ButtonLeft})
	s.HandleInput(window.PointerEvent{X: cx, Y: cy, Press: false, Button: window.ButtonLeft})
}

// firstButtonWithText finds the first button whose label matches.
func firstButtonWithText(t *testing.T, s *bundleScene, label string) *ui.Node {
	t.Helper()
	var found *ui.Node
	var walk func(*ui.Node)
	walk = func(n *ui.Node) {
		if found != nil || n == nil {
			return
		}
		if n.Kind == ui.ElementNode && n.Tag == "button" {
			for _, c := range n.Children {
				if c.Kind == ui.TextNode && c.Text == label {
					found = n
					return
				}
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range s.tree.Roots() {
		walk(r)
	}
	if found == nil {
		t.Fatalf("no button labelled %q", label)
	}
	return found
}

// TestReactivityMountRendersCurrentState is the mount-time half: the initial
// render reflects the initial state, with one {#if} arm and one node per item.
func TestReactivityMountRendersCurrentState(t *testing.T) {
	s, rec := sliceScene(t)

	if got := textOfClass(t, s, "count"); got != "0" {
		t.Errorf("count = %q, want 0", got)
	}
	// count is 0 at mount, so the {#if} must show the else arm only. Both arms
	// visible would mean the conditional was emitted structurally.
	if got := textOfClass(t, s, "hint"); got != "zero or below" {
		t.Errorf("hint = %q, want %q — {#if} must render exactly one arm", got, "zero or below")
	}
	if got := classCount(s, "name"); got != 3 {
		t.Errorf("user rows = %d, want 3 — {#each} must render one node per item", got)
	}
	if got := classCount(s, "row"); got != 3 {
		t.Errorf("row elements = %d, want 3", got)
	}
	// The nav is a keyed {#each} over the section list.
	if got := textOfClassCount(t, s, "", "button"); got < 3 {
		t.Errorf("nav buttons = %d, want at least 3", got)
	}
	// `class:active` is dynamic, so the first runtime pass must evaluate it.
	// An empty or literal class on the mounted dashboard button would mean the
	// mount stream and the emitted runtime disagreed about initial state.
	if active := firstButtonWithText(t, s, "dashboard"); !active.HasClass("active") {
		t.Errorf("initial dashboard button class = %q, want it to contain active", active.GetAttribute("class"))
	}

	if m := rec.Snapshot(); m.UIOpsRejected != 0 {
		t.Errorf("UIOpsRejected = %d at mount, want 0", m.UIOpsRejected)
	}
}

// TestReactivityStateChangeUpdatesUI is the M5 gate: a click writes state, the
// runtime re-renders, and the diff reaches the tree.
func TestReactivityStateChangeUpdatesUI(t *testing.T) {
	s, rec := sliceScene(t)

	before := rec.Snapshot()
	clickElement(t, s, firstButtonWithText(t, s, "+1"))

	if got := textOfClass(t, s, "count"); got != "1" {
		t.Fatalf("count after click = %q, want 1", got)
	}
	// The {#if} arm flipped, which means a node was replaced rather than only
	// text updated.
	if got := textOfClass(t, s, "hint"); got != "positive" {
		t.Errorf("hint after click = %q, want %q", got, "positive")
	}
	if !s.dirty {
		t.Error("dirty = false after a state change, want true")
	}

	after := rec.Snapshot()
	if after.UIOpBatches <= before.UIOpBatches {
		t.Error("a state change submitted no instruction batch")
	}
	if after.UIOpsRejected != 0 {
		t.Errorf("UIOpsRejected = %d, want 0", after.UIOpsRejected)
	}
	// The diff must be far smaller than a remount; that is the point of diffing.
	if delta := after.UIOpsApplied - before.UIOpsApplied; delta > 12 {
		t.Errorf("one state change emitted %d ops; a diff should be far smaller than a remount", delta)
	}
}

// TestReactivityListUpdateRemovesOnlyOne proves keyed reconciliation: removing
// one user removes one node instead of rebuilding the list.
func TestReactivityListUpdateRemovesOnlyOne(t *testing.T) {
	s, rec := sliceScene(t)
	if got := classCount(s, "name"); got != 3 {
		t.Fatalf("user rows = %d, want 3", got)
	}

	before := rec.Snapshot()
	clickElement(t, s, firstButtonWithText(t, s, "remove"))

	if got := classCount(s, "name"); got != 2 {
		t.Fatalf("user rows after remove = %d, want 2", got)
	}

	after := rec.Snapshot()
	if delta := after.UIOpsApplied - before.UIOpsApplied; delta > 10 {
		t.Errorf("removing one row of three emitted %d ops; keyed reconciliation should remove one node", delta)
	}
}

// TestReactivityFilteredListReactsToInput covers the full loop: platform text →
// handler → state → re-render → list shrinks. It is the one test that exercises
// text input, bind:value and {#each} together.
func TestReactivityFilteredListReactsToInput(t *testing.T) {
	s, _ := sliceScene(t)

	input := findTag(t, s.tree, "input")
	if input == nil {
		t.Fatal("no input in the mounted slice")
	}
	clickElement(t, s, input)
	if !s.FocusEditable() {
		t.Fatal("input not focused after a click; text input would never start")
	}

	// No user matches "zzz", so the {#if} arm and the filtered {#each} both
	// change as a result of typing.
	s.HandleInput(window.TextInputEvent{Text: "zzz"})

	if got := classCount(s, "name"); got != 0 {
		t.Errorf("user rows after filtering = %d, want 0", got)
	}
	// Both {#if} arms use class="hint", so the assertion is on the text itself
	// rather than the first hint found in document order.
	if !hasText(s, "no users match") {
		t.Error("the empty-state hint is missing after filtering the list to nothing")
	}
	// The counter's own {#if} is unaffected: filtering must not touch it.
	if got := textOfClass(t, s, "count"); got != "0" {
		t.Errorf("count = %q after filtering, want 0 — filtering must not change the counter", got)
	}
}

// TestReactivityRepeatClicksStayBounded guards the obvious failure mode of a
// re-render model: an unbounded op stream, or a tree that grows per click.
func TestReactivityRepeatClicksStayBounded(t *testing.T) {
	s, rec := sliceScene(t)

	beforeRows := classCount(s, "name")
	inc := firstButtonWithText(t, s, "+1")
	for i := 0; i < 5; i++ {
		clickElement(t, s, inc)
	}

	if got := textOfClass(t, s, "count"); got != "5" {
		t.Errorf("count after five clicks = %q, want 5", got)
	}
	if got := classCount(s, "name"); got != beforeRows {
		t.Errorf("user rows changed from %d to %d; a re-render must not duplicate nodes", beforeRows, got)
	}

	m := rec.Snapshot()
	if m.UIOpsRejected != 0 {
		t.Errorf("UIOpsRejected = %d, want 0", m.UIOpsRejected)
	}
	// Mount plus five clicks, one flush each: more batches would mean a
	// re-render that flushes more than once per change.
	if m.UIOpBatches > 7 {
		t.Errorf("UIOpBatches = %d for mount + 5 clicks; each click should flush once", m.UIOpBatches)
	}
}

// TestReactivityRepaintAfterStateChange closes the loop to the pixels.
func TestReactivityRepaintAfterStateChange(t *testing.T) {
	s, _ := sliceScene(t)
	clickElement(t, s, firstButtonWithText(t, s, "+1"))

	r := software.New()
	r.BeginFrame(900, 640)
	if err := s.Draw(r, 900, 640, 900, 640); err != nil {
		t.Fatalf("Draw after a state change: %v", err)
	}
	r.EndFrame()

	if got := textOfClass(t, s, "count"); got != "1" {
		t.Errorf("count = %q after repaint, want 1", got)
	}
}

// hasText reports whether any text node in the tree carries the string.
func hasText(s *bundleScene, want string) bool {
	found := false
	var walk func(*ui.Node)
	walk = func(n *ui.Node) {
		if n == nil || found {
			return
		}
		if n.Kind == ui.TextNode && n.Text == want {
			found = true
			return
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range s.tree.Roots() {
		walk(r)
	}
	return found
}

// textOfClassCount counts elements of a tag that carry the given class.
//
// An empty class means "no class filter", matching every element of the tag.
// It cannot mean "the element whose class list is empty": HasClass deliberately
// refuses the empty token, because a token match on "" would count every node.
func textOfClassCount(t *testing.T, s *bundleScene, class, tag string) int {
	t.Helper()
	n := 0
	var walk func(*ui.Node)
	walk = func(x *ui.Node) {
		if x == nil {
			return
		}
		if x.Kind == ui.ElementNode && x.Tag == tag && (class == "" || x.HasClass(class)) {
			n++
		}
		for _, c := range x.Children {
			walk(c)
		}
	}
	for _, r := range s.tree.Roots() {
		walk(r)
	}
	return n
}
