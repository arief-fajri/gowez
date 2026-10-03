package opengl

import (
	"errors"

	"github.com/arief-fajri/gowez/internal/render"
)

// ErrNotImplemented is returned until the GPU backend lands, after the
// software backend proves the contract (see DEVELOPMENT_GUIDE.md).
var ErrNotImplemented = errors.New("opengl: backend not implemented yet (planned after the software backend)")

// New returns an OpenGL-backed renderer.
//
// Milestone 1-GPU. Backend choice stays open (Module 9 Open Question 5);
// whichever backend is chosen must satisfy the render.Renderer contract
// unchanged.
func New() (render.Renderer, error) {
	return nil, ErrNotImplemented
}
