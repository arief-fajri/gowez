// Command gowez-hello is the acceptance sample: a native window with
// the Milestone 2 UI scene — UI tree → CSS subset → block/flex layout →
// paint — rasterized by the software backend. No Chromium, no WebView,
// no cgo.
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
