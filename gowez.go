// Package gowez is a Go-native desktop application runtime.
//
// Applications author their UI in Svelte; application and native logic run in
// Go; rendering goes through the runtime's own pipeline. There is no Chromium
// dependency and no OS WebView dependency.
//
// This root package is the entire public API surface of the framework.
// Everything under internal/ is unstable and must not be imported by
// applications or examples (AGENTS.md, hard rule 5).
//
// Status: research / MVP technical validation. See README.md and
// DEVELOPMENT_GUIDE.md for the roadmap.
package gowez

import "github.com/arief-fajri/gowez/internal/app"

// ErrNotImplemented is returned by entry points whose milestone has not
// landed yet. It disappears milestone by milestone (see DEVELOPMENT_GUIDE.md).
// Since Milestone 1, Run no longer returns it; later stubs (script, IPC,
// native APIs) surface their own package-level errors.
var ErrNotImplemented = app.ErrNotImplemented

// Config describes the initial window and application options.
type Config struct {
	// Title is the window title.
	Title string
	// Width and Height are the initial window dimensions in logical pixels.
	Width  int
	Height int
}

// App is a handle to a running application instance. Methods arrive with
// later milestones; the public surface stays limited to this root package.
type App struct {
	inner *app.App
}

// Run starts the application and blocks until the window closes.
//
// Startup follows a bounded sequence: any failed step aborts explicitly —
// never a half-started application. Milestone 1 brings up window +
// software renderer; Milestone 2 adds the UI runtime (tree, style,
// layout, paint) behind the built-in demo scene. The remaining MVP
// steps (JS runtime, UI bundle) arrive with their milestones.
func Run(cfg Config) error {
	return app.Run(app.Options{
		Title:  cfg.Title,
		Width:  cfg.Width,
		Height: cfg.Height,
	})
}
