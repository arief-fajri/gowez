//go:build !darwin && !windows && !linux

package window

// newPlatformWindow is the fallback for platforms without a backend.
// The build must stay portable: unsupported platforms fail explicitly,
// never half-initialize (Module 2 §2.4).
func newPlatformWindow(opts Options) (Window, error) {
	_ = opts
	return nil, ErrUnsupported
}
