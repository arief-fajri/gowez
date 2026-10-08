// paritydump prints the Go CSS subset's accept/reject decisions as JSON so the
// TypeScript validator in packages/adapter can be compared against them.
//
// Built with `go run ./tests/parity/paritydump` by the adapter's parity test.
// It is the mechanism behind "the two validators agree", not a separate source
// of truth: every verdict comes from internal/style.
package main

import (
	"encoding/json"
	"os"

	"github.com/arief-fajri/gowez/tests/parity"
)

func main() {
	decisions := parity.DecideAll()
	if err := json.NewEncoder(os.Stdout).Encode(decisions); err != nil {
		os.Stderr.WriteString("paritydump: " + err.Error() + "\n")
		os.Exit(1)
	}
}
