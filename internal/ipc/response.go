package ipc

import "encoding/json"

// Response is one Go → UI reply. Exactly one of Result or Error is set.
type Response struct {
	// Version is the IPC schema version.
	Version int `json:"version"`
	// ID echoes the request ID.
	ID uint64 `json:"id"`
	// Result is the JSON-encoded success payload.
	Result json.RawMessage `json:"result,omitempty"`
	// Error is set on failure; nil on success.
	Error *Error `json:"error,omitempty"`
}

// Error is a deterministic IPC failure. Codes follow the JSON-RPC spirit
// but stay owned by protocol/ipc.schema.json.
type Error struct {
	// Code is the machine-readable failure code.
	Code int `json:"code"`
	// Message is the human-readable description.
	Message string `json:"message"`
}

// Well-known IPC codes. Unknown methods always resolve to
// CodeMethodNotFound — deterministic, observable, bounded.
const (
	// CodeMethodNotFound: no handler registered for the method.
	CodeMethodNotFound = -32601
	// CodeInvalidParams: payload failed validation.
	CodeInvalidParams = -32602
	// CodeInternal: handler failed internally.
	CodeInternal = -32603
	// CodePermissionDenied: permission gate refused the call (G-SEC-02).
	CodePermissionDenied = -32000
	// CodeTimeout: the call exceeded its deadline (G-REL-01).
	CodeTimeout = -32001
)
