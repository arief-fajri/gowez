// Command gowez-hello is the Milestone 1 acceptance sample: a native
// window, a software-rasterized scene, and live shaped text — no
// Chromium, no WebView, no cgo.
//
// It blocks until the window is closed; any startup or present failure
// prints a diagnostic and exits non-zero.
package main

import (
	"fmt"
	"os"

	"github.com/arief-fajri/gowez"
)

func main() {
	if err := gowez.Run(gowez.Config{
		Title:  "GoWEZ — hello",
		Width:  640,
		Height: 420,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
