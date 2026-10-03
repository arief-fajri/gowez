package observe

import "fmt"

// Diagnostic describes one observable failure (principle P5, Module 5).
// Every failure mode in Module 2 §2.4 must produce one of these — an
// unexplained failure is classification D (missing observability).
type Diagnostic struct {
	// Component names the failing subsystem: "font", "window",
	// "renderer", "scene", "script", "ipc", "api", "assets".
	Component string
	// Message is the human-readable description.
	Message string
	// Err is the underlying error, when any.
	Err error
}

// String formats the diagnostic for logs.
func (d Diagnostic) String() string {
	if d.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", d.Component, d.Message, d.Err)
	}
	return fmt.Sprintf("[%s] %s", d.Component, d.Message)
}

// Reporter receives diagnostics. The application runtime wires
// StderrReporter (Milestone 1); failures always also return errors to
// their callers.
type Reporter interface {
	// Report delivers one diagnostic.
	Report(Diagnostic)
}

// NopReporter discards diagnostics. Default sink for tests.
type NopReporter struct{}

// Report implements Reporter.
func (NopReporter) Report(Diagnostic) {}
