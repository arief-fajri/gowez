package app

import (
	"testing"

	"github.com/arief-fajri/gowez/internal/layout"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/render/backend/software"
)

// TestUISceneRelayout proves the M2 checklist contract "resize triggers
// relayout": the scene relayouts exactly on viewport size changes and
// state (counter) changes — and not on unchanged frames.
func TestUISceneRelayout(t *testing.T) {
	rec := observe.NewRecorder()
	scene := newTestScene(t, rec, nil)
	r := software.New()
	draw := func(w, h int) {
		t.Helper()
		r.BeginFrame(w, h)
		if err := scene.Draw(r, w, h, w, h); err != nil {
			t.Fatalf("Draw(%dx%d): %v", w, h, err)
		}
		r.EndFrame()
	}
	count := func() uint64 { return rec.Snapshot().LayoutCount }
	panelW := func(res *layout.Result) float64 {
		return res.Boxes[scene.tree.Roots()[0].ID].Border.W
	}

	// First draw lays out against the viewport.
	draw(640, 420)
	if count() != 1 {
		t.Fatalf("LayoutCount = %d after first draw, want 1", count())
	}
	firstGeom := scene.geom
	firstW := panelW(firstGeom)

	// Same size, no state change: no relayout, geometry untouched.
	draw(640, 420)
	if count() != 1 {
		t.Fatalf("LayoutCount = %d after identical draw, want 1", count())
	}
	if scene.geom != firstGeom {
		t.Fatal("geometry was replaced without a reason to relayout")
	}

	// A state change (counter) marks the tree dirty → relayout.
	scene.Tick()
	draw(640, 420)
	if count() != 2 {
		t.Fatalf("LayoutCount = %d after Tick, want 2", count())
	}

	// A viewport change → relayout with a wider panel (auto width).
	draw(1024, 768)
	if count() != 3 {
		t.Fatalf("LayoutCount = %d after resize, want 3", count())
	}
	if scene.lastW != 1024 || scene.lastH != 768 {
		t.Fatalf("viewport = %dx%d, want 1024x768", scene.lastW, scene.lastH)
	}
	if nowW := panelW(scene.geom); nowW <= firstW {
		t.Fatalf("panel width did not follow the resize: %g -> %g", firstW, nowW)
	}

	// Layout duration is observable (P5).
	if got := rec.Snapshot().LastLayoutDuration; got < 0 {
		t.Fatalf("LastLayoutDuration = %v, want >= 0", got)
	}
}

// TestUIScenePresentsOpaqueBuffer: the demo scene must cover the whole
// framebuffer (M1 present contract — no transparent pixels delivered).
func TestUIScenePresentsOpaqueBuffer(t *testing.T) {
	scene := newTestScene(t, nil, nil)
	r := software.New()
	r.BeginFrame(320, 200)
	if err := scene.Draw(r, 320, 200, 320, 200); err != nil {
		t.Fatalf("Draw: %v", err)
	}
	r.EndFrame()
	px, err := r.Pixels()
	if err != nil {
		t.Fatalf("Pixels: %v", err)
	}
	for i := 3; i < len(px); i += 4 {
		if px[i] != 255 {
			t.Fatalf("pixel %d alpha = %d, want 255 (opaque present contract)", i/4, px[i])
		}
	}
}
