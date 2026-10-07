package script

import (
	"fmt"

	"github.com/dop251/goja"
)

// Error is an isolated JavaScript failure surfaced to the host instead of
// crashing the runtime (Module 2 §2.4: a JS exception is an observable,
// contained error — guard rail G-REL-02).
type Error struct {
	// Source is the source name passed to Eval (or "handler:<name>").
	Source string
	// Message is the exception message.
	Message string
	// Stack is the engine-provided stack, when the thrown value carries one.
	Stack string
}

// Error implements the error interface for logging and diagnostics.
func (e *Error) Error() string {
	if e == nil {
		return "script: <nil error>"
	}
	return fmt.Sprintf("script: %s: %s", e.Source, e.Message)
}

// newExceptionError converts a goja exception into the package error shape.
// Object-thrown values keep their own message and stack; primitives fall
// back to the engine's full description so the throw position is never
// lost.
func newExceptionError(sourceName string, ex *goja.Exception) *Error {
	e := &Error{Source: sourceName}
	if obj, ok := ex.Value().(*goja.Object); ok {
		if v := obj.Get("message"); v != nil {
			if s, ok := v.Export().(string); ok && s != "" {
				e.Message = s
			}
		}
		if v := obj.Get("stack"); v != nil {
			if s, ok := v.Export().(string); ok && s != "" {
				e.Stack = s
			}
		}
	}
	if e.Message == "" {
		e.Message = ex.Error()
	}
	if e.Stack == "" {
		e.Stack = ex.Error()
	}
	return e
}
