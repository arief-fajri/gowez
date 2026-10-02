// Package window abstracts the native desktop window and input events.
//
// The window contract lives in window.go; platform implementations are
// selected with build tags (darwin.go, win32.go, linux.go, fallback.go).
// Replacing a platform backend must never change this contract.
package window
