package app

import "io/fs"

// Options are the runtime options supplied by the host application through
// the public gowez.Config type.
type Options struct {
	// Title is the initial window title.
	Title string
	// Width and Height are the initial window size in logical pixels.
	Width  int
	Height int
	// UI is the compiled Svelte bundle to mount, or nil for the built-in
	// demo scene. Mirrors gowez.Config.UI (DRR-006).
	UI fs.FS
}

// withDefaults fills zero-value options with MVP defaults.
func (o Options) withDefaults() Options {
	if o.Width <= 0 {
		o.Width = 800
	}
	if o.Height <= 0 {
		o.Height = 600
	}
	return o
}
