package ui

import (
	"strings"
	"testing"
)

// buildFocusTree creates:
//
//	root (tabindex=1) (0,0 200x100)
//	├── btn (button, tabindex=0) (10,10 60x30)
//	│   └── label (text "OK") (14,14 40x20)
//	└── plain (60,10 40x20) — no tabindex
func buildFocusTree(t *testing.T) (*Tree, *Node, *Node, *Node) {
	t.Helper()
	tree := NewTree()
	root := tree.CreateElement("div")
	btn := tree.CreateElement("button")
	label := tree.CreateText("OK")
	plain := tree.CreateElement("div")
	root.SetAttribute("tabindex", "1")
	btn.SetAttribute("tabindex", "0")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]*Node{{root, btn}, {btn, label}, {root, plain}} {
		if err := tree.Append(pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	tree.SetGeometry(map[NodeID]Geometry{
		root.ID:  {Border: Box{X: 0, Y: 0, W: 200, H: 100}},
		btn.ID:   {Border: Box{X: 10, Y: 10, W: 60, H: 30}},
		label.ID: {Border: Box{X: 14, Y: 14, W: 40, H: 20}},
		plain.ID: {Border: Box{X: 60, Y: 10, W: 40, H: 20}},
	})
	return tree, root, btn, plain
}

func TestAddRemoveEventListener(t *testing.T) {
	t.Parallel()
	tree := NewTree()
	n := tree.CreateElement("button")

	if id := tree.AddEventListener(nil, Click, func(*Event) {}); id != 0 {
		t.Fatalf("AddEventListener(nil) = %v, want 0", id)
	}
	if id := tree.AddEventListener(n, Click, nil); id != 0 {
		t.Fatalf("AddEventListener(nil handler) = %v, want 0", id)
	}
	id := tree.AddEventListener(n, Click, func(*Event) {})
	if id == 0 {
		t.Fatal("AddEventListener returned 0 for a valid registration")
	}
	if tree.RemoveEventListener(n, 9999) {
		t.Fatal("RemoveEventListener(unknown) = true, want false")
	}
	if !tree.RemoveEventListener(n, id) {
		t.Fatal("RemoveEventListener(existing) = false, want true")
	}
	if tree.RemoveEventListener(n, id) {
		t.Fatal("second RemoveEventListener = true, want false (idempotent)")
	}
}

// TestDispatchBubblesTargetFirst proves the DOM-like order: listeners
// on the target run first, then ancestors, each node in registration
// order.
func TestDispatchBubblesTargetFirst(t *testing.T) {
	t.Parallel()
	tree, root, btn, _ := buildFocusTree(t)

	var order []string
	tree.AddEventListener(root, Click, func(e *Event) { order = append(order, "root:"+e.CurrentTarget.Tag) })
	tree.AddEventListener(btn, Click, func(e *Event) { order = append(order, "btn:"+e.CurrentTarget.Tag) })
	tree.AddEventListener(root, PointerDown, func(e *Event) { order = append(order, "wrong-kind") })

	n, p := tree.Dispatch(&Event{Kind: Click, Target: btn})
	if n != 2 || p != 0 {
		t.Fatalf("Dispatch = (%d, %d), want (2, 0)", n, p)
	}
	want := []string{"btn:button", "root:div"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestDispatchStopPropagation(t *testing.T) {
	t.Parallel()
	tree, root, btn, _ := buildFocusTree(t)

	var ran []string
	tree.AddEventListener(btn, Click, func(e *Event) {
		ran = append(ran, "btn")
		e.StopPropagation()
	})
	tree.AddEventListener(root, Click, func(e *Event) { ran = append(ran, "root") })

	n, _ := tree.Dispatch(&Event{Kind: Click, Target: btn})
	if n != 1 || len(ran) != 1 || ran[0] != "btn" {
		t.Fatalf("Dispatch ran %v with n=%d, want only btn once", ran, n)
	}
}

// TestDispatchRecoversHandlerPanic proves one faulty handler cannot
// take down dispatch: the panic is recovered, counted, and reported.
func TestDispatchRecoversHandlerPanic(t *testing.T) {
	t.Parallel()
	tree, _, btn, _ := buildFocusTree(t)

	var reported any
	tree.OnPanic = func(e *Event, r any) { reported = r }
	tree.AddEventListener(btn, Click, func(*Event) { panic("boom") })
	var after bool
	tree.AddEventListener(btn, Click, func(*Event) { after = true })

	n, p := tree.Dispatch(&Event{Kind: Click, Target: btn})
	if n != 2 || p != 1 {
		t.Fatalf("Dispatch = (%d, %d), want (2, 1)", n, p)
	}
	if reported != "boom" {
		t.Fatalf("OnPanic reported %v, want \"boom\"", reported)
	}
	if !after {
		t.Fatal("listener after the panicking one did not run")
	}
}

// TestClickSynthesizedOnSameNode: press+release on the same node with
// the primary button yields exactly one Click, bubbling to ancestors.
func TestClickSynthesizedOnSameNode(t *testing.T) {
	t.Parallel()
	tree, root, btn, plain := buildFocusTree(t)

	var clicks int
	tree.AddEventListener(root, Click, func(e *Event) { clicks++; _ = e })
	tree.AddEventListener(btn, Click, func(e *Event) { clicks++; _ = e })

	down := tree.PointerDown(30, 20, ButtonLeft)
	up := tree.PointerUp(30, 20, ButtonLeft)
	if !down.Changed || !up.Changed {
		t.Fatalf("Changed = down:%v up:%v, want both true", down.Changed, up.Changed)
	}
	// One synthesized Click bubbles through btn then root: 2 handler runs.
	if clicks != 2 {
		t.Fatalf("click dispatches = %d, want 2 (btn + root bubble)", clicks)
	}
	_ = plain
}

// TestClickNotSynthesizedWhenReleasedElsewhere: a drag-away release
// fires PointerUp but never Click.
func TestClickNotSynthesizedWhenReleasedElsewhere(t *testing.T) {
	t.Parallel()
	tree := NewTree()
	root := tree.CreateElement("div")
	btn := tree.CreateElement("button")
	plain := tree.CreateElement("div")
	if err := tree.AppendRoot(root); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]*Node{{root, btn}, {root, plain}} {
		if err := tree.Append(pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	tree.SetGeometry(map[NodeID]Geometry{
		root.ID:  {Border: Box{X: 0, Y: 0, W: 200, H: 100}},
		btn.ID:   {Border: Box{X: 10, Y: 10, W: 60, H: 30}},
		plain.ID: {Border: Box{X: 60, Y: 10, W: 40, H: 20}},
	})

	var clicks int
	tree.AddEventListener(root, Click, func(*Event) { clicks++ })

	tree.PointerDown(30, 20, ButtonLeft) // btn
	up := tree.PointerUp(75, 15, ButtonLeft)
	if !up.Changed {
		t.Fatal("release elsewhere Changed = false, want true (press state cleared)")
	}
	if clicks != 0 {
		t.Fatalf("clicks = %d, want 0", clicks)
	}
	if tree.pressed != nil {
		t.Fatal("pressed state survived the release")
	}
}

// TestClickNotSynthesizedForNonPrimaryButton: right-button press and
// release dispatch down/up but no Click.
func TestClickNotSynthesizedForNonPrimaryButton(t *testing.T) {
	t.Parallel()
	tree, root, _, _ := buildFocusTree(t)

	var clicks, downs int
	tree.AddEventListener(root, Click, func(*Event) { clicks++ })
	tree.AddEventListener(root, PointerDown, func(e *Event) {
		if e.Button == ButtonRight {
			downs++
		}
	})

	tree.PointerDown(30, 20, ButtonRight)
	tree.PointerUp(30, 20, ButtonRight)
	if clicks != 0 {
		t.Fatalf("clicks = %d, want 0 (non-primary button)", clicks)
	}
	if downs != 1 {
		t.Fatalf("right-button downs = %d, want 1", downs)
	}
}

// TestHoverStatePropagatesToAncestors: :hover covers the hit node and
// its ancestor chain, and clears when the pointer leaves.
func TestHoverStatePropagatesToAncestors(t *testing.T) {
	t.Parallel()
	tree, root, btn, _ := buildFocusTree(t)

	first := tree.PointerMove(30, 20) // over label (text)
	if !first.Changed {
		t.Fatal("first PointerMove Changed = false, want true")
	}
	if root.State&StateHovered == 0 || btn.State&StateHovered == 0 {
		t.Fatalf("hover did not propagate: root=%d btn=%d", root.State, btn.State)
	}
	label := btn.Children[0]
	if label.State&StateHovered == 0 {
		t.Fatal("hit node not hovered")
	}

	// Same position again: no transition, no restyle needed.
	if again := tree.PointerMove(31, 21); again.Changed {
		t.Fatal("second PointerMove Changed = true, want false (no transition)")
	}
	// Pointer far away: chain clears.
	if away := tree.PointerMove(150, 150); !away.Changed {
		t.Fatal("move-away Changed = false, want true")
	}
	if root.State&StateHovered != 0 || btn.State&StateHovered != 0 || label.State&StateHovered != 0 {
		t.Fatal("hover state leaked after the pointer left")
	}
}

// TestPressStateClearedWhenReleasedOutside: releasing over the
// backdrop clears :active everywhere and fires no click.
func TestPressStateClearedWhenReleasedOutside(t *testing.T) {
	t.Parallel()
	tree, root, btn, _ := buildFocusTree(t)

	tree.PointerDown(30, 20, ButtonLeft)
	if btn.State&StatePressed == 0 || root.State&StatePressed == 0 {
		t.Fatal("press did not propagate to the chain")
	}
	up := tree.PointerUp(190, 90, ButtonLeft) // still inside root, not btn
	if !up.Changed {
		t.Fatal("Changed = false, want true")
	}
	if btn.State&StatePressed != 0 || root.State&StatePressed != 0 {
		t.Fatal("press state leaked after release")
	}
}

// TestPointerFocusNearestFocusable: clicking the label focuses the
// button (nearest focusable ancestor); clicking outside any focusable
// clears focus.
func TestPointerFocusNearestFocusable(t *testing.T) {
	t.Parallel()
	tree, root, btn, plain := buildFocusTree(t)

	tree.PointerDown(30, 20, ButtonLeft) // over label inside btn
	tree.PointerUp(30, 20, ButtonLeft)
	if got := tree.Focused(); got != btn {
		t.Fatalf("Focused = %v, want btn", got)
	}
	if btn.State&StateFocused == 0 {
		t.Fatal(":focus not mirrored onto the node")
	}

	// Clicking plain: nearest focusable ancestor is root.
	tree.PointerDown(75, 15, ButtonLeft)
	tree.PointerUp(75, 15, ButtonLeft)
	if got := tree.Focused(); got != root {
		t.Fatalf("Focused over plain = %v, want root (nearest focusable)", got)
	}

	// Clicking outside every node clears focus entirely.
	tree.PointerDown(250, 150, ButtonLeft)
	tree.PointerUp(250, 150, ButtonLeft)
	if got := tree.Focused(); got != nil {
		t.Fatalf("Focused outside = %v, want nil", got)
	}
	_ = plain
}

// TestTabTraversalWraps: forward cycles through focusables in document
// order with wrap; Shift+Tab goes backwards.
func TestTabTraversalWraps(t *testing.T) {
	t.Parallel()
	tree, root, btn, _ := buildFocusTree(t)

	if res := tree.KeyDown("tab", 0); !res.Changed {
		t.Fatal("first Tab Changed = false, want true")
	}
	if got := tree.Focused(); got != root {
		t.Fatalf("first Tab focus = %v, want root (first focusable)", got)
	}
	tree.KeyDown("tab", 0)
	if got := tree.Focused(); got != btn {
		t.Fatalf("second Tab focus = %v, want btn", got)
	}
	// Wrap forward back to the first.
	tree.KeyDown("tab", 0)
	if got := tree.Focused(); got != root {
		t.Fatalf("wrapped Tab focus = %v, want root", got)
	}
	// Backwards from first wraps to last.
	tree.KeyDown("tab", ModShift)
	if got := tree.Focused(); got != btn {
		t.Fatalf("Shift+Tab focus = %v, want btn (wrap to last)", got)
	}
}

// TestKeyEventsDispatchToFocusedNode: keys reach the focused node;
// without focus they are dropped (0 dispatched).
func TestKeyEventsDispatchToFocusedNode(t *testing.T) {
	t.Parallel()
	tree, root, btn, _ := buildFocusTree(t)

	var seen []string
	tree.AddEventListener(root, KeyDown, func(e *Event) { seen = append(seen, e.Key) })

	if res := tree.KeyDown("arrow-left", 0); res.Dispatched != 0 {
		t.Fatalf("KeyDown without focus dispatched %d, want 0", res.Dispatched)
	}
	tree.KeyDown("tab", 0) // focus root
	res := tree.KeyDown("arrow-left", ModShift)
	if res.Dispatched != 1 || len(seen) != 1 || seen[0] != "arrow-left" {
		t.Fatalf("focused key: res=%+v seen=%v, want 1 dispatch of arrow-left", res, seen)
	}
	// Tab is consumed for traversal, never dispatched as a key event.
	tree.KeyDown("tab", 0)
	if got := tree.KeyDown("tab", 0); got.Dispatched != 0 {
		t.Fatalf("Tab dispatched %d, want 0 (consumed)", got.Dispatched)
	}
	_ = btn
}

// TestEnterSpaceActivatesFocusedButton: keyboard activation fires a
// Click on the focused button after the key event.
func TestEnterSpaceActivatesFocusedButton(t *testing.T) {
	t.Parallel()
	tree, _, btn, _ := buildFocusTree(t)

	var kinds []EventKind
	tree.AddEventListener(btn, KeyDown, func(e *Event) { kinds = append(kinds, e.Kind) })
	tree.AddEventListener(btn, Click, func(e *Event) { kinds = append(kinds, e.Kind) })

	tree.KeyDown("tab", 0) // btn is the second focusable... first Tab → root
	tree.KeyDown("tab", 0) // now btn
	if tree.Focused() != btn {
		t.Fatalf("focus = %v, want btn", tree.Focused())
	}
	tree.KeyDown("enter", 0)
	if len(kinds) != 2 || kinds[0] != KeyDown || kinds[1] != Click {
		t.Fatalf("kinds = %v, want [KeyDown Click]", kinds)
	}

	kinds = nil
	tree.KeyDown("space", 0)
	if len(kinds) != 2 || kinds[1] != Click {
		t.Fatalf("space kinds = %v, want [KeyDown Click]", kinds)
	}
	// A non-activating key fires no click.
	kinds = nil
	tree.KeyDown("a", 0)
	if len(kinds) != 1 || kinds[0] != KeyDown {
		t.Fatalf("'a' kinds = %v, want [KeyDown]", kinds)
	}
}

// TestDetachPrunesInteractionState: a detached subtree must not leave
// hover/focus state referencing nodes outside the tree (invariant I1).
func TestDetachPrunesInteractionState(t *testing.T) {
	t.Parallel()
	tree, root, btn, plain := buildFocusTree(t)

	tree.PointerMove(30, 20) // hover label chain
	tree.KeyDown("tab", 0)
	tree.KeyDown("tab", 0) // focus btn
	if tree.Focused() != btn {
		t.Fatalf("setup: focus = %v, want btn", tree.Focused())
	}

	if err := tree.Detach(btn); err != nil {
		t.Fatal(err)
	}
	if tree.Focused() != nil {
		t.Fatal("focus survived detaching the focused node")
	}
	if btn.State&StateFocused != 0 {
		t.Fatal(":focus bit survived on the detached node")
	}
	if root.State&StateHovered != 0 {
		t.Fatal("hover on a still-attached ancestor survived the detached chain")
	}
	_ = plain
}

// TestRemovedListenerStopsFiring keeps the registry contract honest.
func TestRemovedListenerStopsFiring(t *testing.T) {
	t.Parallel()
	tree, _, btn, _ := buildFocusTree(t)

	var fired int
	id := tree.AddEventListener(btn, Click, func(*Event) { fired++ })
	tree.Dispatch(&Event{Kind: Click, Target: btn})
	if !tree.RemoveEventListener(btn, id) {
		t.Fatal("RemoveEventListener failed")
	}
	tree.Dispatch(&Event{Kind: Click, Target: btn})
	if fired != 1 {
		t.Fatalf("fired = %d after removal, want 1", fired)
	}
}
