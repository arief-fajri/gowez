package script

import "fmt"

// Error is an isolated JavaScript failure surfaced to the host instead of
// crashing the runtime (Module 2 §2.4: JS exception → error terisolasi dan
// observable).
type Error struct {
	// Source is the source name passed to Eval.
	Source string
	// Message is the exception message.
	Message string
	// Stack is the engine-provided stack, when available.
	Stack string
}

// Error implements the error interface for logging and diagnostics.
func (e *Error) Error() string {
	if e == nil {
		return "script: <nil error>"
	}
	return fmt.Sprintf("script: %s: %s", e.Source, e.Message)
}
