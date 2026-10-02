// Package assets loads the compiled UI bundle (Svelte output plus static
// resources), Milestone 5.
//
// Packaged builds embed the bundle with go:embed for single-binary
// deployment (Milestone 7); development builds read from disk through the
// same Loader so the two paths cannot diverge.
package assets
