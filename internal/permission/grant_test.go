package permission

import "testing"

// TestDenyByDefault proves the core security property: no grant, no access
// (G-SEC-02). A nil set behaves like an empty one.
func TestDenyByDefault(t *testing.T) {
	var nilSet *Set
	if nilSet.Allows(FSRead) {
		t.Fatal("nil set must deny")
	}
	empty := NewSet()
	for _, p := range []Permission{FSRead, FSWrite, DialogOpen, Clipboard, WindowCtl} {
		if empty.Allows(p) {
			t.Fatalf("empty set allowed %q", p)
		}
	}
}

func TestGrantAllowsOnlyNamedPermission(t *testing.T) {
	s := NewSet(FSRead)
	if !s.Allows(FSRead) {
		t.Fatal("granted permission denied")
	}
	if s.Allows(FSWrite) {
		t.Fatal("unrelated permission allowed — grants must be exact (no wildcard)")
	}
	if s.Allows("") {
		t.Fatal("empty permission must be denied")
	}
}

func TestGrantAddsPermission(t *testing.T) {
	s := NewSet(FSRead)
	s.Grant(WindowCtl)
	if !s.Allows(WindowCtl) || !s.Allows(FSRead) {
		t.Fatalf("grants = %+v", s)
	}
	// Idempotent: granting twice must not corrupt the set.
	s.Grant(WindowCtl)
	if !s.Allows(WindowCtl) {
		t.Fatal("second grant lost the permission")
	}
}

// TestNoWildcardPermission exists as a regression guard: the MVP permission
// constants contain no "all" grant the UI could reach (G-SEC-01).
func TestNoWildcardPermission(t *testing.T) {
	all := []Permission{FSRead, FSWrite, DialogOpen, Clipboard, WindowCtl}
	for _, p := range all {
		if string(p) == "*" || string(p) == "all" {
			t.Fatalf("wildcard permission %q exists", p)
		}
	}
}
