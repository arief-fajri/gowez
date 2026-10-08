// Command gowez-dashboard runs the M5 acceptance sample: an in-subset Svelte
// slice compiled by @gowez/adapter and mounted by the Go runtime.
//
// Build the bundle first. The dist directory is committed, so this runs without
// npm until you change the slice:
//
//	npm run build -w @gowez/example-gowez-dashboard
//
// Run it. os.DirFS resolves relative to the process working directory — not to
// this source file — so the path has to be right for wherever you launched from:
//
//	cd examples/gowez-dashboard && go run .
//	go run ./examples/gowez-dashboard -dist examples/gowez-dashboard/dist
//
// The example imports only the root gowez package (AGENTS.md hard rule 5); the
// whole public surface an application needs is Config.UI.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"

	"github.com/arief-fajri/gowez"
)

// distFlag names the directory holding the compiled bundle: manifest.json,
// app.js and styles.css (DRR-006).
var distFlag = flag.String("dist", "dist",
	"directory holding the compiled Svelte bundle (manifest.json, app.js, styles.css)")

func main() {
	flag.Parse()

	files := os.DirFS(*distFlag)
	if _, err := fs.Stat(files, "manifest.json"); err != nil {
		fmt.Fprintf(os.Stderr,
			"gowez-dashboard: no compiled bundle in %q (%v)\n\n"+
				"The bundle path is resolved against the current working directory.\n"+
				"Run it from the sample directory:\n"+
				"  cd examples/gowez-dashboard && go run .\n"+
				"or point -dist at it from anywhere:\n"+
				"  go run ./examples/gowez-dashboard -dist examples/gowez-dashboard/dist\n\n"+
				"Build it after changing the slice:\n"+
				"  npm install\n"+
				"  npm run build:adapter\n"+
				"  npm run build:sample\n",
			*distFlag, err)
		os.Exit(1)
	}

	if err := gowez.Run(gowez.Config{
		Title:  "GoWEZ — Svelte slice (M5)",
		Width:  900,
		Height: 640,
		UI:     files,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "gowez-dashboard: %v\n", err)
		os.Exit(1)
	}
}
