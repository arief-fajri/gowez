// Package api defines the built-in native capabilities exposed to the UI.
//
// The Registry in registry.go is the single door between IPC and native
// operations (G-SEC-01, G-SEC-02): nothing reaches the OS without passing
// through a registered handler that the permission gate has approved.
package api
