//go:build linux

package window

// newPlatformWindow creates the native window backend for Linux.
//
// Milestone 1: replace the ErrUnsupported stub with a real backend
// (X11/Wayland decision pending, Module 9 Open Question 5).
func newPlatformWindow(opts Options) (Window, error) {
	_ = opts
	return nil, ErrUnsupported
}
