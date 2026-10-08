package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshot renders the tree shape so an atomicity test can assert that a
// rejected batch left nothing behind.
func snapshot(t *testing.T, roots []*Node) string {
	t.Helper()
	var b strings.Builder
	var walk func(n *Node, depth int)
	walk = func(n *Node, depth int) {
		fmt.Fprintf(&b, "%s%d:%s", strings.Repeat("  ", depth), n.ID, n.Tag)
		if n.Kind == TextNode {
			fmt.Fprintf(&b, " %s", n.Text)
		} else {
			keys := make([]string, 0, len(n.Attrs))
			for k := range n.Attrs {
				keys = append(keys, k)
			}
			for i := 0; i < len(keys); i++ {
				for j := i + 1; j < len(keys); j++ {
					if keys[j] < keys[i] {
						keys[i], keys[j] = keys[j], keys[i]
					}
				}
			}
			for _, k := range keys {
				fmt.Fprintf(&b, " %s=%q", k, n.Attrs[k])
			}
		}
		b.WriteByte('\n')
		for _, c := range n.Children {
			walk(c, depth+1)
		}
	}
	for _, r := range roots {
		walk(r, 0)
	}
	return b.String()
}

// apply is the one-shot applier used by tests that do not need cross-batch
// state. Incremental tests use a Session directly.
func apply(t *testing.T, tree *Tree, ops ...Op) (*Result, error) {
	t.Helper()
	return ApplyOps(tree, &Batch{Version: InstructionVersion, Ops: ops}, nil)
}

// applyEntry is apply with an explicit entry id, for tests that assert on it.
func applyEntry(t *testing.T, tree *Tree, entry int, ops ...Op) (*Result, error) {
	t.Helper()
	return ApplyOps(tree, &Batch{Version: InstructionVersion, Entry: entry, Ops: ops}, nil)
}

func buildTree(t *testing.T) *Tree {
	t.Helper()
	tree := NewTree()
	res, err := applyEntry(t, tree, 1,
		Op{Kind: OpCreateElement, NodeID: 1, Tag: "panel"},
		Op{Kind: OpCreateElement, NodeID: 2, Tag: "title"},
		Op{Kind: OpCreateText, NodeID: 3, Text: "GoWEZ"},
		Op{Kind: OpCreateElement, NodeID: 4, Tag: "button"},
		Op{Kind: OpCreateText, NodeID: 5, Text: "Apply"},
		Op{Kind: OpAppendChild, ParentID: 1, ChildID: 2},
		Op{Kind: OpAppendChild, ParentID: 1, ChildID: 4},
		Op{Kind: OpAppendChild, ParentID: 2, ChildID: 3},
		Op{Kind: OpAppendChild, ParentID: 4, ChildID: 5},
		Op{Kind: OpSetAttribute, NodeID: 1, Name: "class", Value: "root"},
	)
	if err != nil {
		t.Fatalf("ApplyOps: %v", err)
	}
	if res.Entry != 1 {
		t.Fatalf("Entry = %d, want 1", res.Entry)
	}
	return tree
}

func TestApplyOpsBuildsTree(t *testing.T) {
	tree := buildTree(t)

	got := snapshot(t, tree.Roots())
	want := strings.Join([]string{
		`1:panel class="root"`,
		`  2:title`,
		"    3: GoWEZ",
		`  4:button`,
		"    5: Apply",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("tree mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestApplyOpsAtomicRejectsInvalidStream is the G-DATA-02 evidence: op 5 is
// invalid, so the whole batch must be refused with the tree untouched — not a
// partially applied stream.
func TestApplyOpsAtomicRejectsInvalidStream(t *testing.T) {
	tree := buildTree(t)
	before := snapshot(t, tree.Roots())

	_, err := apply(t, tree,
		Op{Kind: OpCreateElement, NodeID: 10, Tag: "row"},
		Op{Kind: OpCreateElement, NodeID: 11, Tag: "label"},
		Op{Kind: OpCreateText, NodeID: 12, Text: "label"},
		Op{Kind: OpAppendChild, ParentID: 10, ChildID: 11},
		Op{Kind: OpAppendChild, ParentID: 10, ChildID: 12},
		// op 5: unknown event kind — rejected.
		Op{Kind: OpAddEventListener, NodeID: 10, Event: "wiggle", HandlerID: 1},
		Op{Kind: OpAppendChild, ParentID: 11, ChildID: 12},
	)
	if err == nil {
		t.Fatal("ApplyOps accepted a stream with an unknown event kind")
	}
	if !strings.Contains(err.Error(), "wiggle") {
		t.Errorf("error should name the offending event, got: %v", err)
	}

	if after := snapshot(t, tree.Roots()); after != before {
		t.Fatalf("tree mutated by a rejected batch\n before:\n%s\n after:\n%s", before, after)
	}
}

func TestApplyOpsRejects(t *testing.T) {
	cases := []struct {
		name string
		ops  []Op
		want string
	}{
		{
			name: "UnknownEvent",
			ops: []Op{
				{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
				{Kind: OpAddEventListener, NodeID: 1, Event: "nope", HandlerID: 1},
			},
			want: "not a subscribable kind",
		},
		{
			name: "DupID",
			ops: []Op{
				{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
				{Kind: OpCreateElement, NodeID: 1, Tag: "span"},
			},
			want: "duplicate nodeId",
		},
		{
			name: "Cycle",
			ops: []Op{
				{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
				{Kind: OpCreateElement, NodeID: 2, Tag: "span"},
				{Kind: OpAppendChild, ParentID: 1, ChildID: 2},
				{Kind: OpAppendChild, ParentID: 2, ChildID: 1},
			},
			want: "cycle",
		},
		{
			name: "BadRequired",
			ops: []Op{
				{Kind: OpCreateElement, NodeID: 1, Tag: ""},
			},
			want: "tag is required",
		},
		{
			name: "MissingTagChar",
			ops: []Op{
				{Kind: OpCreateElement, NodeID: 1, Tag: "di v"},
			},
			want: "invalid character",
		},
		{
			name: "DanglingParent",
			ops: []Op{
				{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
				{Kind: OpCreateElement, NodeID: 2, Tag: "span"},
				{Kind: OpAppendChild, ParentID: 99, ChildID: 2},
			},
			want: "not a known node",
		},
		{
			name: "SetTextOnElement",
			ops: []Op{
				{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
				{Kind: OpSetText, NodeID: 1, Value: "nope"},
			},
			want: "only text nodes carry text",
		},
		{
			name: "SelfParent",
			ops: []Op{
				{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
				{Kind: OpAppendChild, ParentID: 1, ChildID: 1},
			},
			want: "cannot be its own child",
		},
		{
			name: "EmptyStyle",
			ops: []Op{
				{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
				{Kind: OpSetStyle, NodeID: 1},
			},
			want: "must not be empty",
		},
		{
			name: "RemoveChildNotChild",
			ops: []Op{
				{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
				{Kind: OpCreateElement, NodeID: 2, Tag: "span"},
				{Kind: OpCreateElement, NodeID: 3, Tag: "p"},
				{Kind: OpAppendChild, ParentID: 1, ChildID: 2},
				{Kind: OpRemoveChild, ParentID: 3, ChildID: 2},
			},
			want: "not a child of",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := NewTree()
			_, err := ApplyOps(tree, &Batch{Version: InstructionVersion, Ops: tc.ops}, nil)
			if err == nil {
				t.Fatalf("ApplyOps accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
			if got := snapshot(t, tree.Roots()); got != "" {
				t.Errorf("tree not empty after rejection:\n%s", got)
			}
		})
	}
}

func TestApplyOpsRejectsVersion(t *testing.T) {
	tree := NewTree()
	_, err := ApplyOps(tree, &Batch{Version: InstructionVersion + 1, Ops: []Op{
		{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
	}}, nil)
	if err == nil {
		t.Fatal("ApplyOps accepted a future instruction version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error should mention the version, got: %v", err)
	}
}

func TestApplyOpsRejectsBadEntry(t *testing.T) {
	tree := NewTree()
	_, err := applyEntry(t, tree, 42,
		Op{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
	)
	if err == nil {
		t.Fatal("ApplyOps accepted an entry that no op created")
	}
	if !strings.Contains(err.Error(), "entry") {
		t.Errorf("error should mention entry, got: %v", err)
	}
}

func TestApplyOpsRejectsUnknownKind(t *testing.T) {
	tree := NewTree()
	_, err := ApplyOps(tree, &Batch{
		Version: InstructionVersion,
		Ops:     []Op{{Kind: "teleport", NodeID: 1}},
	}, nil)
	if err == nil {
		t.Fatal("ApplyOps accepted an unknown op kind")
	}
	if !strings.Contains(err.Error(), "unknown kind") {
		t.Errorf("error = %v, want unknown kind", err)
	}
}

// TestApplyBindsAndRemovesListener covers the listener map: a batch returns one
// Binding per addEventListener, and the applier registers a placeholder so the
// caller can install real behavior without disturbing registration order.
func TestApplyBindsAndRemovesListener(t *testing.T) {
	tree := NewTree()
	res, err := applyEntry(t, tree, 1,
		Op{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
		Op{Kind: OpCreateElement, NodeID: 2, Tag: "button"},
		Op{Kind: OpAppendChild, ParentID: 1, ChildID: 2},
		Op{Kind: OpAddEventListener, NodeID: 2, Event: "click", HandlerID: 7},
		Op{Kind: OpAddEventListener, NodeID: 1, Event: "key-down", HandlerID: 8},
	)
	if err != nil {
		t.Fatalf("ApplyOps: %v", err)
	}
	if len(res.Bindings) != 2 {
		t.Fatalf("Bindings = %d, want 2", len(res.Bindings))
	}
	if res.Bindings[0].Kind != Click || res.Bindings[0].HandlerID != 7 {
		t.Errorf("binding 0 = %+v, want click/7", res.Bindings[0])
	}
	if res.Bindings[0].Listener == 0 || res.Bindings[1].Listener == 0 {
		t.Error("every binding must carry a runtime ListenerID")
	}

	// Dispatch before installing real behavior runs the placeholder and never
	// panics.
	target := tree.Roots()[0].Children[0]
	disp, panics := tree.Dispatch(&Event{Kind: Click, Target: target})
	if panics != 0 {
		t.Fatalf("panics = %d, want 0", panics)
	}
	if disp != 1 {
		t.Errorf("dispatched = %d, want 1 (placeholder is live)", disp)
	}

	// Installing real behavior takes effect immediately.
	var ran bool
	if !tree.SetListenerHandler(target, res.Bindings[0].Listener, func(*Event) { ran = true }) {
		t.Fatal("SetListenerHandler reported no such registration")
	}
	if _, _ = tree.Dispatch(&Event{Kind: Click, Target: target}); !ran {
		t.Error("installed handler did not run")
	}
}

func TestApplyRejectsDuplicateHandlerBinding(t *testing.T) {
	tree := NewTree()
	_, err := apply(t, tree,
		Op{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
		Op{Kind: OpAddEventListener, NodeID: 1, Event: "click", HandlerID: 3},
		Op{Kind: OpAddEventListener, NodeID: 1, Event: "click", HandlerID: 3},
	)
	if err == nil {
		t.Fatal("ApplyOps accepted the same handler twice on one node")
	}
	if !strings.Contains(err.Error(), "already bound") {
		t.Errorf("error = %v, want already bound", err)
	}
}

// TestSetStyleInlineMerge proves inline style merging: existing declarations
// keep their order, new properties are appended deterministically, and a
// repeated property is replaced in place.
func TestSetStyleInlineMerge(t *testing.T) {
	tree := NewTree()
	res, err := applyEntry(t, tree, 1,
		Op{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
		Op{Kind: OpSetAttribute, NodeID: 1, Name: "style", Value: "color: #fff; padding: 4px"},
		Op{Kind: OpSetStyle, NodeID: 1, Style: map[string]string{
			"background-color": "#000",
			"color":            "#111",
		}},
		// A child so the entry reaches the root list and is inspectable.
		Op{Kind: OpCreateElement, NodeID: 2, Tag: "span"},
		Op{Kind: OpAppendChild, ParentID: 1, ChildID: 2},
	)
	if err != nil {
		t.Fatalf("ApplyOps: %v", err)
	}
	root, ok := res.Node(1)
	if !ok {
		t.Fatal("adapter id 1 missing from result")
	}
	got := root.GetAttribute("style")
	want := "color: #111; padding: 4px; background-color: #000"
	if got != want {
		t.Errorf("style = %q, want %q", got, want)
	}
}

func TestSetStyleRejectsMalformedExisting(t *testing.T) {
	tree := NewTree()
	_, err := apply(t, tree,
		Op{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
		Op{Kind: OpSetAttribute, NodeID: 1, Name: "style", Value: "color #fff"},
		Op{Kind: OpSetStyle, NodeID: 1, Style: map[string]string{"padding": "1px"}},
	)
	if err == nil {
		t.Fatal("ApplyOps accepted a malformed inline style declaration")
	}
	if !strings.Contains(err.Error(), "malformed inline declaration") {
		t.Errorf("error = %v, want malformed inline declaration", err)
	}
}

func TestApplyRemoveChildDetaches(t *testing.T) {
	tree := NewTree()
	res, err := applyEntry(t, tree, 1,
		Op{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
		Op{Kind: OpCreateElement, NodeID: 2, Tag: "span"},
		Op{Kind: OpCreateText, NodeID: 3, Text: "bye"},
		Op{Kind: OpAppendChild, ParentID: 1, ChildID: 2},
		Op{Kind: OpAppendChild, ParentID: 2, ChildID: 3},
		Op{Kind: OpRemoveChild, ParentID: 1, ChildID: 2},
	)
	if err != nil {
		t.Fatalf("ApplyOps: %v", err)
	}
	if res.Removed != 1 {
		t.Errorf("Removed = %d, want 1", res.Removed)
	}
	// The detached node must not be resurrected as a root.
	if got := snapshot(t, tree.Roots()); got != "1:div\n" {
		t.Errorf("tree after remove:\n%s", got)
	}
}

// TestSessionUpdatesAcrossBatches is the incremental-update contract: a later
// batch addresses nodes an earlier batch created. The stateless applier would
// see those ids as dangling references.
func TestSessionUpdatesAcrossBatches(t *testing.T) {
	tree := NewTree()
	s := NewSession(tree)

	mount, err := s.Apply(&Batch{
		Version: InstructionVersion,
		Entry:   1,
		Ops: []Op{
			{Kind: OpCreateElement, NodeID: 1, Tag: "panel"},
			{Kind: OpCreateText, NodeID: 2, Text: "count: 0"},
			{Kind: OpCreateElement, NodeID: 3, Tag: "button"},
			{Kind: OpCreateText, NodeID: 4, Text: "+1"},
			{Kind: OpAppendChild, ParentID: 1, ChildID: 2},
			{Kind: OpAppendChild, ParentID: 1, ChildID: 3},
			{Kind: OpAppendChild, ParentID: 3, ChildID: 4},
			{Kind: OpAddEventListener, NodeID: 3, Event: "click", HandlerID: 1},
		},
	})
	if err != nil {
		t.Fatalf("mount batch: %v", err)
	}
	if len(mount.Nodes) != 4 {
		t.Errorf("mount created %d nodes, want 4", len(mount.Nodes))
	}

	// Update text on a node the mount created.
	if _, err := s.Apply(&Batch{
		Version: InstructionVersion,
		Ops:     []Op{{Kind: OpSetText, NodeID: 2, Value: "count: 1"}},
	}); err != nil {
		t.Fatalf("update batch: %v", err)
	}
	n, ok := s.Node(2)
	if !ok {
		t.Fatal("session lost adapter id 2")
	}
	if n.Text != "count: 1" {
		t.Errorf("text = %q, want count: 1", n.Text)
	}

	// Remove a listener bound by the mount batch.
	res, err := s.Apply(&Batch{
		Version: InstructionVersion,
		Ops: []Op{
			{Kind: OpRemoveEventListener, NodeID: 3, Event: "click", HandlerID: 1},
		},
	})
	if err != nil {
		t.Fatalf("remove-listener batch: %v", err)
	}
	if len(res.Bindings) != 0 {
		t.Errorf("Bindings = %d, want 0 on a removal batch", len(res.Bindings))
	}
	btn, _ := s.Node(3)
	if disp, _ := tree.Dispatch(&Event{Kind: Click, Target: btn}); disp != 0 {
		t.Errorf("dispatched = %d after removal, want 0", disp)
	}
}

// TestSessionRejectedBatchKeepsState: a rejected batch must not consume ids or
// forget bindings, so the next valid batch still resolves (I1 across batches).
func TestSessionRejectedBatchKeepsState(t *testing.T) {
	tree := NewTree()
	s := NewSession(tree)

	if _, err := s.Apply(&Batch{
		Version: InstructionVersion,
		Entry:   1,
		Ops: []Op{
			{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
			{Kind: OpAddEventListener, NodeID: 1, Event: "click", HandlerID: 5},
		},
	}); err != nil {
		t.Fatalf("first batch: %v", err)
	}

	if _, err := s.Apply(&Batch{
		Version: InstructionVersion,
		Ops: []Op{
			{Kind: OpCreateElement, NodeID: 1, Tag: "span"}, // duplicate id
		},
	}); err == nil {
		t.Fatal("session accepted a duplicate id after a successful batch")
	}

	// The original binding must still be removable.
	if _, err := s.Apply(&Batch{
		Version: InstructionVersion,
		Ops: []Op{
			{Kind: OpRemoveEventListener, NodeID: 1, Event: "click", HandlerID: 5},
		},
	}); err != nil {
		t.Fatalf("binding lost after a rejected batch: %v", err)
	}
	if got := snapshot(t, tree.Roots()); got != "1:div\n" {
		t.Errorf("tree changed unexpectedly:\n%s", got)
	}
}

func TestApplyRejectsReparentInOneBatch(t *testing.T) {
	tree := NewTree()
	_, err := apply(t, tree,
		Op{Kind: OpCreateElement, NodeID: 1, Tag: "div"},
		Op{Kind: OpCreateElement, NodeID: 2, Tag: "span"},
		Op{Kind: OpCreateElement, NodeID: 3, Tag: "p"},
		Op{Kind: OpAppendChild, ParentID: 1, ChildID: 2},
		Op{Kind: OpAppendChild, ParentID: 3, ChildID: 2},
	)
	if err == nil {
		t.Fatal("ApplyOps accepted re-parenting a node in one batch")
	}
	if !strings.Contains(err.Error(), "already attached") {
		t.Errorf("error = %v, want already attached", err)
	}
}

// TestApplyOpsSchemaDrift is the anti-drift guard: the version this package
// applies must equal the schema's declared floor, and the schema's `kind` enum
// must contain every kind the applier handles.
func TestApplyOpsSchemaDrift(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "protocol", "ui-instruction.schema.json"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	// The schema root describes the *instruction object*, so there is no
	// schema-level version integer to compare. The declared floor lives in
	// properties.version.minimum — that is the version the schema admits, and
	// the applier must speak exactly it.
	var schema struct {
		Properties struct {
			Version struct {
				Minimum int `json:"minimum"`
			} `json:"version"`
		} `json:"properties"`
		Defs struct {
			Op struct {
				Properties struct {
					Kind struct {
						Enum []string `json:"enum"`
					} `json:"kind"`
				} `json:"properties"`
			} `json:"op"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	if schema.Properties.Version.Minimum != InstructionVersion {
		t.Fatalf("schema properties.version.minimum %d != applier InstructionVersion %d",
			schema.Properties.Version.Minimum, InstructionVersion)
	}

	inSchema := make(map[string]bool, len(schema.Defs.Op.Properties.Kind.Enum))
	for _, k := range schema.Defs.Op.Properties.Kind.Enum {
		inSchema[k] = true
	}
	for _, k := range []OpKind{
		OpCreateElement, OpCreateText, OpSetAttribute, OpSetStyle, OpSetText,
		OpAppendChild, OpRemoveChild, OpAddEventListener, OpRemoveEventListener,
	} {
		if !inSchema[string(k)] {
			t.Errorf("applier handles %q but the schema enum does not list it", k)
		}
	}
}

func TestEventKindNamesAreStable(t *testing.T) {
	cases := map[string]EventKind{
		"click":        Click,
		"pointer-down": PointerDown,
		"pointer-up":   PointerUp,
		"pointer-move": PointerMove,
		"key-down":     KeyDown,
		"key-up":       KeyUp,
		"textinput":    TextInput,
		"textediting":  TextEditing,
	}
	for name, want := range cases {
		got, ok := EventKindName(name)
		if !ok {
			t.Errorf("EventKindName(%q) not found", name)
			continue
		}
		if got != want {
			t.Errorf("EventKindName(%q) = %v, want %v", name, got, want)
		}
		if got.String() != name {
			t.Errorf("EventKind(%q).String() = %q", name, got.String())
		}
	}
	if _, ok := EventKindName("submit"); ok {
		t.Error("EventKindName must not accept an unsupported kind")
	}
}
