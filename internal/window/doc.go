// Package window abstracts the native desktop window and input events.
//
// The window contract lives in window.go. The SDL3 backend (sdl.go,
// purego, no cgo — DRR-001) covers {darwin,linux,windows} ×
// {amd64,arm64}; any other platform compiles against fallback.go and
// fails explicitly at runtime. Replacing a platform backend must never
// change the contract.
//
// Threading: create the window from the OS main thread and use it only
// from that goroutine (SDL requirement); Pump and Present are
// non-blocking and bounded.
package window
