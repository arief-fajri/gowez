//go:build !((darwin || linux || windows) && (amd64 || arm64))

package window

// newPlatformWindow is the fallback for platforms without a backend.
// The build must stay portable: unsupported platforms fail explicitly,
// never half-initialize (Module 2 §2.4).
func newPlatformWindow(opts Options) (Window, error) {
	_ = opts
	return nil, ErrUnsupported
}

// StartTextInput and StopTextInput are part of the Window contract, but this
// backend never produces a window, so neither is reachable: New returns
// ErrUnsupported before any caller can ask for text input. Keeping the
// methods on the concrete type preserves the contract's completeness without
// pretending a headless build can accept text.
