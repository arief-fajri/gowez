package ui

import "strconv"

// InputResult reports what feeding one native input event into the
// tree did: Changed means visual state moved (the caller must
// restyle/relayout before the next paint), Dispatched counts handlers
// that ran, Panics counts recovered handler faults.
type InputResult struct {
	Changed    bool
	Dispatched int
	Panics     int
}

func (r *InputResult) add(dispatched, panics int) {
	r.Dispatched += dispatched
	r.Panics += panics
}

// PointerMove updates hover state for the position and dispatches
// PointerMove to the node under the pointer. A nil hit (pointer over
// the backdrop) drops the event — unhandled events are never guessed
// at (invariant I6).
func (t *Tree) PointerMove(x, y float64) InputResult {
	var res InputResult
	target := t.HitTest(x, y)
	res.Changed = t.setHover(target)
	d, p := t.Dispatch(&Event{Kind: PointerMove, Target: target, X: x, Y: y})
	res.add(d, p)
	return res
}

// PointerDown records the press target (setting :active on it and its
// ancestors), moves focus on the primary button — nearest focusable
// ancestor wins, none clears focus — and dispatches PointerDown.
func (t *Tree) PointerDown(x, y float64, button int) InputResult {
	var res InputResult
	target := t.HitTest(x, y)

	if t.pressed != nil {
		t.setStateChain(t.pressed, StatePressed, false)
		t.pressed = nil
		res.Changed = true
	}
	if target != nil {
		t.setStateChain(target, StatePressed, true)
		t.pressed = target
		res.Changed = true
	}
	t.pressedButton = button
	if button == ButtonLeft && t.focusForPointer(target) {
		res.Changed = true
	}

	d, p := t.Dispatch(&Event{Kind: PointerDown, Target: target, X: x, Y: y, Button: button})
	res.add(d, p)
	return res
}

// PointerUp releases :active and dispatches PointerUp. A Click is
// synthesized when the release lands on the same node that took the
// press with the primary button — a release elsewhere (drag-away) or
// on a non-primary button never produces a click (docs/EVENTS.md).
func (t *Tree) PointerUp(x, y float64, button int) InputResult {
	var res InputResult
	target := t.HitTest(x, y)

	pressed, pressedButton := t.pressed, t.pressedButton
	if pressed != nil {
		t.setStateChain(pressed, StatePressed, false)
		t.pressed = nil
		t.pressedButton = 0
		res.Changed = true
	}

	d, p := t.Dispatch(&Event{Kind: PointerUp, Target: target, X: x, Y: y, Button: button})
	res.add(d, p)

	if pressed != nil && target == pressed && pressedButton == ButtonLeft && button == ButtonLeft {
		d, p := t.Dispatch(&Event{Kind: Click, Target: target, X: x, Y: y, Button: ButtonLeft})
		res.add(d, p)
	}
	return res
}

// KeyDown handles keyboard input: Tab (with Shift for backwards) is
// consumed for focus traversal and never dispatched; any other key
// dispatches to the focused node — or is dropped when nothing is
// focused. Enter/Space on a focused button synthesizes a Click after
// the key event (docs/EVENTS.md §keyboard).
func (t *Tree) KeyDown(key string, mods int) InputResult {
	var res InputResult
	if key == "tab" {
		res.Changed = t.traverseFocus(mods&ModShift != 0)
		return res
	}
	focus := t.focus
	if focus == nil {
		return res
	}
	d, p := t.Dispatch(&Event{Kind: KeyDown, Target: focus, Key: key, Modifier: mods})
	res.add(d, p)
	if focus.Kind == ElementNode && focus.Tag == "button" && (key == "enter" || key == "space") {
		d, p := t.Dispatch(&Event{Kind: Click, Target: focus, Button: ButtonLeft, Modifier: mods})
		res.add(d, p)
	}
	return res
}

// KeyUp dispatches the release to the focused node; dropped when
// nothing is focused. Unlike KeyDown, Tab is delivered normally.
func (t *Tree) KeyUp(key string, mods int) InputResult {
	var res InputResult
	if t.focus == nil {
		return res
	}
	d, p := t.Dispatch(&Event{Kind: KeyUp, Target: t.focus, Key: key, Modifier: mods})
	res.add(d, p)
	return res
}

// Focused returns the node holding keyboard focus, or nil.
func (t *Tree) Focused() *Node { return t.focus }

// setHover moves :hover to n (and its ancestors), clearing the
// previous chain. Reports whether the hovered node changed.
func (t *Tree) setHover(n *Node) bool {
	if t.hover == n {
		return false
	}
	if t.hover != nil {
		t.setStateChain(t.hover, StateHovered, false)
	}
	if n != nil {
		t.setStateChain(n, StateHovered, true)
	}
	t.hover = n
	return true
}

// focusForPointer focuses the nearest focusable ancestor of target
// (target included); a target outside any focusable clears focus.
// Reports whether focus changed.
func (t *Tree) focusForPointer(target *Node) bool {
	var f *Node
	for cur := target; cur != nil; cur = cur.Parent {
		if isFocusable(cur) {
			f = cur
			break
		}
	}
	return t.setFocus(f)
}

// setFocus moves keyboard focus, mirroring :focus onto the node.
// Reports whether focus changed.
func (t *Tree) setFocus(n *Node) bool {
	if t.focus == n {
		return false
	}
	if t.focus != nil {
		t.focus.State &^= StateFocused
	}
	if n != nil {
		n.State |= StateFocused
	}
	t.focus = n
	return true
}

// traverseFocus moves focus to the next focusable node in document
// order, wrapping at both ends; backwards is Shift+Tab. With no
// focusable nodes the focus is cleared. Reports whether focus changed.
func (t *Tree) traverseFocus(backward bool) bool {
	f := t.focusables()
	if len(f) == 0 {
		return t.setFocus(nil)
	}
	idx := -1
	for i, n := range f {
		if n == t.focus {
			idx = i
			break
		}
	}
	var next *Node
	if backward {
		if idx <= 0 {
			next = f[len(f)-1]
		} else {
			next = f[idx-1]
		}
	} else if idx < 0 {
		next = f[0]
	} else {
		next = f[(idx+1)%len(f)]
	}
	return t.setFocus(next)
}

// focusables lists focusable elements in document order. An element is
// focusable when it carries a non-negative integer tabindex — a
// non-integer value makes it not focusable, an explicit divergence
// from browsers documented in docs/EVENTS.md.
func (t *Tree) focusables() []*Node {
	var out []*Node
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.Kind == ElementNode && isFocusable(n) {
			out = append(out, n)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range t.roots {
		walk(r)
	}
	return out
}

// isFocusable reports whether n accepts keyboard focus (tabindex >= 0).
func isFocusable(n *Node) bool {
	v, err := strconv.Atoi(n.GetAttribute("tabindex"))
	return err == nil && v >= 0
}

// setStateChain sets or clears bit on n and every ancestor — :hover
// and :active apply to the element and its ancestors, matching
// browser behavior (docs/EVENTS.md).
func (t *Tree) setStateChain(n *Node, bit StateBits, on bool) {
	for cur := n; cur != nil; cur = cur.Parent {
		if on {
			cur.State |= bit
		} else {
			cur.State &^= bit
		}
	}
}

// pruneInteraction drops hover/press/focus state that points into a
// detached subtree so no interaction state ever references a node
// outside the tree (invariant I1).
func (t *Tree) pruneInteraction(n *Node) {
	if t.focus != nil && isAncestor(n, t.focus) {
		t.focus.State &^= StateFocused
		t.focus = nil
	}
	if t.hover != nil && isAncestor(n, t.hover) {
		t.setStateChain(t.hover, StateHovered, false)
		t.hover = nil
	}
	if t.pressed != nil && isAncestor(n, t.pressed) {
		t.setStateChain(t.pressed, StatePressed, false)
		t.pressed = nil
		t.pressedButton = 0
	}
}
