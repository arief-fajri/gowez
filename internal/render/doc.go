// Package render defines the backend-agnostic rendering contract: UI and
// layout emit commands, backends execute them.
//
// The pipeline is UI tree → style resolution → layout tree → render tree →
// render commands → GPU backend → native window. Swapping a backend must
// never change this contract (guard rail G-UPG-03).
package render
