package observe

import (
	"fmt"
	"os"
)

// StderrReporter writes diagnostics to standard error. It is the
// production sink wired by the application runtime (Milestone 1).
type StderrReporter struct{}

// Report implements Reporter.
func (StderrReporter) Report(d Diagnostic) {
	fmt.Fprintln(os.Stderr, d.String())
}
