package app

// Options are the runtime options supplied by the host application through
// the public gowez.Config type.
type Options struct {
	// Title is the initial window title.
	Title string
	// Width and Height are the initial window size in logical pixels.
	Width  int
	Height int
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
