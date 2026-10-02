//go:build darwin

package window

// newPlatformWindow creates the native window backend for macOS.
//
// Milestone 1: replace the ErrUnsupported stub with a real backend
// (CGWindow/NSWindow binding or a windowing library — decision pending).
func newPlatformWindow(opts Options) (Window, error) {
	_ = opts
	return nil, ErrUnsupported
}
