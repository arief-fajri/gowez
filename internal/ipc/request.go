package ipc

import "encoding/json"

// Request is one UI → Go invocation.
//
// The wire shape is defined by protocol/ipc.schema.json; changing it
// requires a schema version bump (G-IFACE-02, G-IFACE-03).
type Request struct {
	// Version is the IPC schema version.
	Version int `json:"version"`
	// ID correlates the response with this request.
	ID uint64 `json:"id"`
	// Method is the dotted operation name, e.g. "fs.readTextFile".
	Method string `json:"method"`
	// Params is the JSON-encoded parameter payload.
	Params json.RawMessage `json:"params,omitempty"`
}
