//go:build windows

package window

// newPlatformWindow creates the native window backend for Windows.
//
// Milestone 1: replace the ErrUnsupported stub with a real Win32 backend.
func newPlatformWindow(opts Options) (Window, error) {
	_ = opts
	return nil, ErrUnsupported
}
