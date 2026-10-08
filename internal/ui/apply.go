package ui

import (
	"fmt"
	"sort"
	"strings"
)

// InstructionVersion is the `version` integer this package applies. It must
// equal `version` in protocol/ui-instruction.schema.json; TestApplyOpsSchemaDrift
// enforces the equality so the Go applier and the schema cannot drift apart
// (same pattern as the ipcVersion guard between internal/api and internal/ipc).
const InstructionVersion = 1

// OpKind is the `kind` field of a UI instruction op. The set mirrors the
// `kind` enum in protocol/ui-instruction.schema.json; an unknown kind is
// rejected explicitly rather than ignored (G-UPG-04).
type OpKind string

// The op kinds defined by the instruction schema.
const (
	OpCreateElement       OpKind = "createElement"
	OpCreateText          OpKind = "createText"
	OpSetAttribute        OpKind = "setAttribute"
	OpSetStyle            OpKind = "setStyle"
	OpSetText             OpKind = "setText"
	OpAppendChild         OpKind = "appendChild"
	OpRemoveChild         OpKind = "removeChild"
	OpAddEventListener    OpKind = "addEventListener"
	OpRemoveEventListener OpKind = "removeEventListener"
)

// Op is one UI instruction. Adapter-allocated ids are the adapter's own
// address space; the tree allocates its own NodeIDs and ApplyOps maps between
// them, so the adapter never needs to know the runtime numbering.
type Op struct {
	Kind OpKind `json:"kind"`
	// NodeID addresses the op's subject node.
	NodeID int `json:"nodeId,omitempty"`
	// ParentID/ChildID address an appendChild/removeChild pair.
	ParentID int `json:"parentId,omitempty"`
	ChildID  int `json:"childId,omitempty"`
	// Tag is the element name for createElement.
	Tag string `json:"tag,omitempty"`
	// Name is the attribute name for setAttribute.
	Name string `json:"name,omitempty"`
	// Value is an attribute value or a single text value.
	Value string `json:"value,omitempty"`
	// Text is the text-node content for createText.
	Text string `json:"text,omitempty"`
	// Props carries attribute name/value pairs.
	Props map[string]string `json:"props,omitempty"`
	// Style carries style property/value pairs, resolved last and merged
	// into the node's inline `style` attribute.
	Style map[string]string `json:"style,omitempty"`
	// Event is the event-kind name for addEventListener/removeEventListener.
	Event string `json:"event,omitempty"`
	// HandlerID is the adapter's handle for a JS handler.
	HandlerID int `json:"handlerId,omitempty"`
}

// Batch is one complete UI instruction stream: the payload of a single
// `ui.apply` call. Version must equal InstructionVersion; Entry is the
// adapter-declared component root.
type Batch struct {
	Version int  `json:"version"`
	Entry   int  `json:"entry,omitempty"`
	Ops     []Op `json:"ops"`
}

// Binding is what ApplyOps hands back for the caller to wire into the script
// layer: one entry per addEventListener op, in stream order.
type Binding struct {
	// NodeID is the adapter-allocated id of the listening node.
	NodeID int
	// HandlerID is the adapter-allocated handler handle.
	HandlerID int
	// Kind is the resolved runtime event kind.
	Kind EventKind
	// Listener is the runtime handle from AddEventListener, needed by a
	// later removeEventListener op for the same (node, handler) pair.
	Listener ListenerID
}

// Result reports what one applied batch changed. Counters feed
// internal/observe (Module 5, P5); the rejected count is also what produces a
// Diagnostic{Component:"ui"}.
type Result struct {
	// Entry is the adapter-declared component root id.
	Entry int
	// Nodes maps adapter-allocated ids to runtime nodes, so the caller can
	// address a node after the batch (handler wiring, later updates)
	// without knowing the runtime's own numbering.
	Nodes map[int]*Node
	// Ops is the number of ops applied.
	Ops int
	// Bindings holds one entry per addEventListener op, in stream order.
	Bindings []Binding
	// Removed counts detached nodes.
	Removed int
}

// Node returns the runtime node for an adapter-allocated id.
func (r *Result) Node(adapterID int) (*Node, bool) {
	n, ok := r.Nodes[adapterID]
	return n, ok
}

// eventKindByName maps the protocol's event names to runtime kinds. It is the
// single source of truth for which event a stream may subscribe to: a name
// outside this table is rejected (I6 — an unknown event kind is never
// guessed at).
var eventKindByName = map[string]EventKind{
	"click":        Click,
	"pointer-down": PointerDown,
	"pointer-up":   PointerUp,
	"pointer-move": PointerMove,
	"key-down":     KeyDown,
	"key-up":       KeyUp,
	"textinput":    TextInput,
	"textediting":  TextEditing,
}

// EventKindName reports whether name is a subscribable event kind.
func EventKindName(name string) (EventKind, bool) {
	k, ok := eventKindByName[name]
	return k, ok
}

// Session applies instruction batches to one tree across many calls.
//
// It exists because the adapter's node ids are stable across batches: the
// mount batch creates the tree, and every later batch addresses those same
// ids to update text, add or remove nodes, or swap listeners. A stateless
// ApplyOps could only ever see one batch, so an update referring to an
// existing node would look like a dangling reference.
//
// A Session is not safe for concurrent use: the tree is single-goroutine by
// contract, and batches are applied on the UI goroutine.
type Session struct {
	tree  *Tree
	nodes map[int]*Node
	// bound tracks (node, handler) subscriptions across batches, so a
	// removeEventListener in a later batch finds the registration an earlier
	// one made.
	bound map[listenerKey]ListenerID
	// parents tracks each node's parent across batches. A removeChild in a
	// later batch refers to a relationship an earlier batch created, so this
	// cannot be derived from one batch alone — but it is durable state, and a
	// remove that does not match it is still a stream error.
	parents map[int]int
}

// NewSession starts an instruction session over a tree. The tree should be
// empty for a fresh mount; an existing tree may carry nodes from earlier work,
// but their adapter ids are unknown to the session until an op addresses them.
func NewSession(t *Tree) *Session {
	return &Session{
		tree:    t,
		nodes:   map[int]*Node{},
		bound:   map[listenerKey]ListenerID{},
		parents: map[int]int{},
	}
}

// Apply applies one batch in the session, carrying the adapter-id map and the
// listener registrations forward. A rejected batch changes nothing — neither
// the tree nor the session state (invariant I1, G-DATA-02).
func (s *Session) Apply(b *Batch) (*Result, error) {
	if s == nil || s.tree == nil {
		return nil, fmt.Errorf("ui: nil session")
	}
	res, err := applyOps(s.tree, b, s.nodes, s.bound, s.parents)
	if err != nil {
		return nil, err
	}
	for id, n := range res.Nodes {
		s.nodes[id] = n
	}
	return res, nil
}

// Tree returns the tree this session applies to.
func (s *Session) Tree() *Tree { return s.tree }

// Node returns the runtime node for an adapter-allocated id, from this or any
// earlier batch.
func (s *Session) Node(adapterID int) (*Node, bool) {
	n, ok := s.nodes[adapterID]
	return n, ok
}

// Nodes returns a copy of the adapter-id map.
func (s *Session) Nodes() map[int]*Node {
	out := make(map[int]*Node, len(s.nodes))
	for k, v := range s.nodes {
		out[k] = v
	}
	return out
}

// ApplyOps applies one instruction batch to the tree in two phases.
//
// Phase one validates the whole stream without touching the tree. Phase two
// mutates, and only once validation has fully passed. A batch that fails
// validation therefore leaves the tree byte-for-byte unchanged (invariant I1,
// guard rail G-DATA-02) — never a half-built subtree.
//
// Validation is deliberately stricter than the JSON schema in four places,
// because a stream that reaches Go has already lost the information needed to
// be lenient: duplicate node ids, structural cycles, a removeChild that
// matches no append, and an `entry` that does not resolve to a node the stream
// created.
//
// known is the adapter-id map carried over from earlier batches; it may be nil
// for a one-shot apply. Use Session for anything that updates over time — a
// one-shot call cannot remove a listener an earlier batch bound.
func ApplyOps(t *Tree, b *Batch, known map[int]*Node) (*Result, error) {
	return applyOps(t, b, known, map[listenerKey]ListenerID{}, map[int]int{})
}

// applyOps is the shared implementation behind ApplyOps and Session.Apply. The
// caller owns the lifetime of known, bound and parents; a one-shot ApplyOps
// passes empty maps, so cross-batch removals are only available through a
// Session.
func applyOps(
	t *Tree,
	b *Batch,
	known map[int]*Node,
	bound map[listenerKey]ListenerID,
	parents map[int]int,
) (*Result, error) {
	if t == nil {
		return nil, fmt.Errorf("ui: nil tree")
	}
	if b == nil {
		return nil, fmt.Errorf("ui: nil batch")
	}
	if b.Version != InstructionVersion {
		return nil, fmt.Errorf("ui: instruction version %d, want %d", b.Version, InstructionVersion)
	}

	// existing is every id an earlier batch created. A creation op may not
	// reuse one: ids are stable across a session's lifetime, so a reuse would
	// make one adapter id mean two different nodes over time.
	existing := make(map[int]bool, len(known))
	for id := range known {
		existing[id] = true
	}

	// addressable is every id a stream may reference: those it creates plus
	// those earlier batches created.
	addressable := make(map[int]bool, len(existing)+len(b.Ops))
	for id := range existing {
		addressable[id] = true
	}

	// Phase 1a — per-op shape validation. Nothing is allocated yet.
	created := make(map[int]bool, len(b.Ops))
	for i, op := range b.Ops {
		if err := validateOp(i, op, addressable, created, existing); err != nil {
			return nil, err
		}
	}

	// Phase 1b — structural validation against a projected parent map. This
	// runs before any mutation so a cycle, a dangling id, or a removeChild
	// that does not match any append cannot leave a partially attached
	// subtree behind (I1, G-DATA-02).
	// parent is the projected parent map for this batch, seeded from the
	// durable one so a remove can refer to a relationship created earlier.
	parent := make(map[int]int, len(known)+len(b.Ops))
	for child, par := range parents {
		parent[child] = par
	}
	for i, op := range b.Ops {
		switch op.Kind {
		case OpAppendChild:
			if prev, moved := parent[op.ChildID]; moved && prev != op.ParentID {
				return nil, fmt.Errorf("ui: op %d: node %d is already attached to %d", i, op.ChildID, prev)
			}
			parent[op.ChildID] = op.ParentID
			if isAncestorIn(parent, op.ChildID, op.ParentID) {
				return nil, fmt.Errorf("ui: op %d: appendChild would create a cycle (%d under %d)", i, op.ChildID, op.ParentID)
			}
		case OpRemoveChild:
			// The pair must correspond to an append in this same stream: a
			// remove for a node that was never attached here is a stream
			// error, not a silent no-op.
			if parent[op.ChildID] != op.ParentID {
				return nil, fmt.Errorf("ui: op %d: node %d is not a child of %d in this batch", i, op.ChildID, op.ParentID)
			}
			delete(parent, op.ChildID)
		}
	}

	// Phase 2 — apply. Every mutation below is guaranteed to satisfy Tree's
	// preconditions by the validation above.
	res := &Result{Entry: b.Entry, Ops: len(b.Ops)}
	// nodes starts from the carried-over map so this batch can address nodes
	// earlier batches created; res.Nodes reports only what this batch added.
	nodes := make(map[int]*Node, len(known)+len(created))
	for id, n := range known {
		nodes[id] = n
	}
	added := make(map[int]*Node, len(created))
	res.Nodes = added
	// detached records nodes the stream removed, so phase 3 does not mistake
	// them for roots that were never attached.
	detached := make(map[int]bool)

	for i, op := range b.Ops {
		if err := applyOp(t, op, nodes, bound, res, detached, parents); err != nil {
			// Unreachable by construction: phase 1 already rejected every
			// shape phase 2 can fail on. Reported rather than swallowed so a
			// future op addition cannot fail silently.
			return nil, fmt.Errorf("ui: op %d: %w", i, err)
		}
	}
	for _, id := range sortedKeys(created) {
		added[id] = nodes[id]
	}

	if b.Entry != 0 {
		if _, ok := nodes[b.Entry]; !ok {
			return nil, fmt.Errorf("ui: entry %d is not a node created by this batch", b.Entry)
		}
	}

	// Phase 3 — root every node left parentless. This is what makes the
	// outermost element(s) of a batch reachable from Roots(); anything the
	// stream attached keeps its place. Iteration is over sorted ids so the
	// root order is deterministic for a given stream (Module 6).
	for _, id := range sortedKeys(nodes) {
		n := nodes[id]
		if n.Parent != nil || t.isRoot(n) || detached[id] {
			// A node the stream removed is intentionally detached, not a root.
			continue
		}
		if err := t.AppendRoot(n); err != nil {
			return nil, fmt.Errorf("ui: rooting node %d: %w", id, err)
		}
	}
	return res, nil
}

// sortedKeys returns the map keys in ascending order.
func sortedKeys[V any](m map[int]V) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// listenerKey identifies one (node, handler) subscription so removeEventListener
// can find the runtime handle AddEventListener returned.
type listenerKey struct {
	node    int
	handler int
}

// placeholderHandler stands in for the real handler between ApplyOps and the
// caller's SetListenerHandler call. It exists only so AddEventListener
// allocates a ListenerID; dispatching an unbound listener is a no-op.
func placeholderHandler(*Event) {}

// validateOp checks one op's required fields and referential integrity.
// addressable is every id the stream may reference (this batch's creations plus
// earlier batches'); created is the subset this batch itself creates, which is
// what makes a duplicate id an error while an update to an existing node is not.
func validateOp(i int, op Op, addressable, created, existing map[int]bool) error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("ui: op %d (%s): %s", i, op.Kind, fmt.Sprintf(format, args...))
	}

	// Every kind but appendChild/removeChild addresses a subject node.
	if op.Kind != OpAppendChild && op.Kind != OpRemoveChild {
		if op.NodeID == 0 {
			return bad("nodeId is required")
		}
	}

	switch op.Kind {
	case OpCreateElement:
		if op.NodeID == 0 {
			return bad("nodeId is required")
		}
		if created[op.NodeID] || existing[op.NodeID] {
			return bad("duplicate nodeId %d (already used by this or an earlier batch)", op.NodeID)
		}
		if strings.TrimSpace(op.Tag) == "" {
			return bad("tag is required")
		}
		if err := validTag(op.Tag); err != nil {
			return bad("tag %q: %v", op.Tag, err)
		}
		created[op.NodeID] = true
		addressable[op.NodeID] = true
		return nil

	case OpCreateText:
		if op.NodeID == 0 {
			return bad("nodeId is required")
		}
		if created[op.NodeID] || existing[op.NodeID] {
			return bad("duplicate nodeId %d (already used by this or an earlier batch)", op.NodeID)
		}
		created[op.NodeID] = true
		addressable[op.NodeID] = true
		return nil

	case OpSetAttribute:
		if !addressable[op.NodeID] {
			return bad("nodeId %d is not a known node (not created by this or any earlier batch)", op.NodeID)
		}
		if op.Name == "" {
			return bad("name is required")
		}
		return nil

	case OpSetStyle:
		if !addressable[op.NodeID] {
			return bad("nodeId %d is not a known node (not created by this or any earlier batch)", op.NodeID)
		}
		if len(op.Style) == 0 {
			return bad("style is required and must not be empty")
		}
		return nil

	case OpSetText:
		if !addressable[op.NodeID] {
			return bad("nodeId %d is not a known node (not created by this or any earlier batch)", op.NodeID)
		}
		return nil

	case OpAppendChild:
		if op.ParentID == 0 || op.ChildID == 0 {
			return bad("parentId and childId are required")
		}
		if !addressable[op.ParentID] {
			return bad("parentId %d is not a known node (not created by this or any earlier batch)", op.ParentID)
		}
		if !addressable[op.ChildID] {
			return bad("childId %d is not a known node (not created by this or any earlier batch)", op.ChildID)
		}
		if op.ParentID == op.ChildID {
			return bad("node %d cannot be its own child", op.ChildID)
		}
		return nil

	case OpRemoveChild:
		if op.ParentID == 0 || op.ChildID == 0 {
			return bad("parentId and childId are required")
		}
		if !addressable[op.ParentID] {
			return bad("parentId %d is not a known node (not created by this or any earlier batch)", op.ParentID)
		}
		if !addressable[op.ChildID] {
			return bad("childId %d is not a known node (not created by this or any earlier batch)", op.ChildID)
		}
		return nil

	case OpAddEventListener:
		if !addressable[op.NodeID] {
			return bad("nodeId %d is not a known node (not created by this or any earlier batch)", op.NodeID)
		}
		if op.HandlerID == 0 {
			return bad("handlerId is required")
		}
		if _, ok := EventKindName(op.Event); !ok {
			return bad("event %q is not a subscribable kind", op.Event)
		}
		return nil

	case OpRemoveEventListener:
		if !addressable[op.NodeID] {
			return bad("nodeId %d is not a known node (not created by this or any earlier batch)", op.NodeID)
		}
		if op.HandlerID == 0 {
			return bad("handlerId is required")
		}
		if _, ok := EventKindName(op.Event); !ok {
			return bad("event %q is not a subscribable kind", op.Event)
		}
		return nil

	default:
		return bad("unknown kind")
	}
}

// validTag keeps element names to a conservative identifier shape. The
// adapter only ever emits tags it validated, so this is a defense against a
// malformed stream, not the subset policy itself (that lives in the adapter).
func validTag(tag string) error {
	for i, r := range tag {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r == '-' || r == '_' || r == ':':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return fmt.Errorf("invalid character %q", r)
		}
	}
	return nil
}

// isAncestorIn reports whether candidate appears in id's parent chain within
// the projected parent map. Used to reject cycles before any mutation.
func isAncestorIn(parent map[int]int, candidate, id int) bool {
	for cur, i := id, 0; i <= len(parent); i++ {
		next, ok := parent[cur]
		if !ok {
			return false
		}
		if next == candidate {
			return true
		}
		cur = next
	}
	return false
}

// applyOp performs the mutation for one validated op.
func applyOp(
	t *Tree,
	op Op,
	nodes map[int]*Node,
	listeners map[listenerKey]ListenerID,
	res *Result,
	detached map[int]bool,
	parents map[int]int,
) error {
	switch op.Kind {
	case OpCreateElement:
		n := t.CreateElement(op.Tag)
		nodes[op.NodeID] = n

	case OpCreateText:
		n := t.CreateText(op.Text)
		nodes[op.NodeID] = n

	case OpSetAttribute:
		nodes[op.NodeID].SetAttribute(op.Name, op.Value)

	case OpSetStyle:
		// Inline style is resolved last by internal/style, so merging into
		// the node's existing `style` attribute preserves the ordering
		// guarantee (docs/CSS-SUBSET.md).
		n := nodes[op.NodeID]
		merged, err := mergeInlineStyle(n.GetAttribute("style"), op.Style)
		if err != nil {
			return err
		}
		n.SetAttribute("style", merged)

	case OpSetText:
		n := nodes[op.NodeID]
		// Stricter than the schema: setText on an element would have nowhere
		// to render, so it is a stream error rather than a silent no-op.
		if n.Kind != TextNode {
			return fmt.Errorf("setText targets element <%s> (node %d); only text nodes carry text", n.Tag, n.ID)
		}
		n.Text = op.Value

	case OpAppendChild:
		// Rooting is not decided here: a node that is still parentless at the
		// end of the batch becomes a root. Deciding it per-op would root an
		// intermediate parent that a later op attaches, splitting one
		// component across several roots.
		if err := t.Append(nodes[op.ParentID], nodes[op.ChildID]); err != nil {
			return err
		}
		parents[op.ChildID] = op.ParentID

	case OpRemoveChild:
		parent, child := nodes[op.ParentID], nodes[op.ChildID]
		if child.Parent != parent {
			return fmt.Errorf("node %d is not a child of %d", op.ChildID, op.ParentID)
		}
		if err := t.Detach(child); err != nil {
			return err
		}
		delete(parents, op.ChildID)
		detached[op.ChildID] = true
		res.Removed++

	case OpAddEventListener:
		kind, _ := EventKindName(op.Event)
		key := listenerKey{node: op.NodeID, handler: op.HandlerID}
		if _, dup := listeners[key]; dup {
			// The same handler on the same node twice is a stream error, not
			// a second registration (docs/EVENTS.md §dispatch order).
			return fmt.Errorf("handler %d is already bound on node %d", op.HandlerID, op.NodeID)
		}
		// Registered with a placeholder: AddEventListener treats a nil
		// handler as a no-op, and the applier does not know what should run
		// (the caller binds it to the script layer). SetListenerHandler
		// installs the real behavior without disturbing registration order.
		id := t.AddEventListener(nodes[op.NodeID], kind, placeholderHandler)
		listeners[key] = id
		res.Bindings = append(res.Bindings, Binding{
			NodeID: op.NodeID, HandlerID: op.HandlerID, Kind: kind, Listener: id,
		})

	case OpRemoveEventListener:
		key := listenerKey{node: op.NodeID, handler: op.HandlerID}
		id, ok := listeners[key]
		if !ok {
			return fmt.Errorf("handler %d is not bound on node %d", op.HandlerID, op.NodeID)
		}
		if t.RemoveEventListener(nodes[op.NodeID], id) {
			delete(listeners, key)
		}

	default:
		return fmt.Errorf("unknown kind %q", op.Kind)
	}
	return nil
}

// mergeInlineStyle folds a style map into an existing inline declaration
// string, preserving the original order and replacing only the properties the
// op mentions. Sorted keys keep the output deterministic for a given input
// (Module 6 determinism), which a golden test depends on.
func mergeInlineStyle(existing string, style map[string]string) (string, error) {
	props := map[string]string{}
	order := []string{}

	add := func(prop, value string) {
		if _, seen := props[prop]; !seen {
			order = append(order, prop)
		}
		props[prop] = value
	}

	if s := strings.TrimSpace(existing); s != "" {
		for _, decl := range strings.Split(s, ";") {
			decl = strings.TrimSpace(decl)
			if decl == "" {
				continue
			}
			prop, value, ok := strings.Cut(decl, ":")
			if !ok {
				return "", fmt.Errorf("malformed inline declaration %q", decl)
			}
			add(strings.TrimSpace(prop), strings.TrimSpace(value))
		}
	}

	newProps := make([]string, 0, len(style))
	for prop := range style {
		newProps = append(newProps, prop)
	}
	sort.Strings(newProps)
	for _, prop := range newProps {
		if strings.TrimSpace(prop) == "" {
			return "", fmt.Errorf("empty style property name")
		}
		add(strings.TrimSpace(prop), style[prop])
	}

	parts := make([]string, 0, len(order))
	for _, prop := range order {
		parts = append(parts, prop+": "+props[prop])
	}
	return strings.Join(parts, "; "), nil
}
