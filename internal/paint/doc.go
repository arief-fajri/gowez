// Package paint translates a laid-out UI tree into render commands.
//
// It is the last pure stage of the pipeline: tree + computed styles +
// layout result → DrawRect/DrawText commands. Paint makes no layout
// decisions (text lines come from layout) and never talks to a backend —
// the renderer contract stays untouched (G-UPG-03).
package paint
