// Package opengl is the GPU renderer backend (Milestone 1-GPU).
//
// It is deliberately deferred until the software backend has proven the
// render contract: the pipeline must work before graphics complexity is
// added, and G-UPG-03 requires that either backend can be swapped without
// touching the UI API.
package opengl
