package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/arief-fajri/gowez/internal/permission"
)

func TestRegistryRegisterAndInvoke(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("app.echo", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		return params, nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Invoke(context.Background(), "app.echo", json.RawMessage(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"a":1}` {
		t.Fatalf("result = %s", got)
	}
}

func TestRegistryUnknownMethodIsDeterministic(t *testing.T) {
	r := NewRegistry()
	_, err := r.Invoke(context.Background(), "eval.exec", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestRegistryRejectsBadRegistrations(t *testing.T) {
	r := NewRegistry()
	h := func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) { return nil, nil }
	if err := r.Register("", "", h); err == nil {
		t.Error("empty name accepted")
	}
	if err := r.Register("app.x", "", nil); err == nil {
		t.Error("nil handler accepted")
	}
	if err := r.Register("app.x", "", h); err != nil {
		t.Fatal(err)
	}
	if err := r.Register("app.x", "", h); err == nil {
		t.Error("duplicate accepted")
	}
}

// TestDefaultRegistryDeclaresPermissions pins the gate contract: every
// native method names the permission that guards it, and app.getInfo — the
// non-native success path — needs none (G-SEC-02, G-SEC-03).
func TestDefaultRegistryDeclaresPermissions(t *testing.T) {
	r, err := NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]permission.Permission{
		"app.getInfo":        "",
		"fs.readTextFile":    permission.FSRead,
		"fs.writeTextFile":   permission.FSWrite,
		"dialog.open":        permission.DialogOpen,
		"clipboard.readText": permission.Clipboard,
		"window.setTitle":    permission.WindowCtl,
	}
	names := r.Names()
	if len(names) != len(want) {
		t.Fatalf("Names() = %v", names)
	}
	for _, n := range names {
		if r.Permission(n) != want[n] {
			t.Errorf("method %q permission %q, want %q", n, r.Permission(n), want[n])
		}
	}
	if r.Permission("not.registered") != "" {
		t.Error("unknown method must report no permission")
	}
}

// TestGetInfoSucceedsWithoutImplementation proves the M4 success-path method
// returns real JSON (it is not a stub), while the M6 natives stay explicit.
func TestGetInfoSucceedsWithoutImplementation(t *testing.T) {
	r, err := NewDefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Invoke(context.Background(), "app.getInfo", nil)
	if err != nil {
		t.Fatalf("app.getInfo: %v", err)
	}
	if len(got) == 0 || got[0] != '{' {
		t.Fatalf("payload = %s", got)
	}
	if _, err := r.Invoke(context.Background(), "fs.readTextFile", nil); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("fs stub must stay explicit, got %v", err)
	}
}
