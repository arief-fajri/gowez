// Package software is the CPU reference renderer backend.
//
// It exists first (Milestone 1) so the whole pipeline — layout, commands,
// failure experiments — can be tested deterministically in CI without a
// GPU, and so the renderer abstraction is proven with two backends before
// the GPU one lands (guard rail G-UPG-03).
package software
