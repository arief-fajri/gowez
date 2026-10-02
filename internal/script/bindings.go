package script

// Binding exposes exactly one host function to JavaScript under an explicit
// name. Arbitrary Go function exposure is forbidden (G-SEC-01): bindings are
// registered deliberately, one at a time, and routed through the IPC
// dispatcher with its permission gate.
type Binding struct {
	// Name is the identifier visible to JavaScript.
	Name string
	// Handler executes the call. Handlers are bounded and must return
	// errors rather than panic (guard rail G-REL-01).
	Handler func(args []any) (any, error)
}
