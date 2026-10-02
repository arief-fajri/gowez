// Package ipc is the only channel between UI/JavaScript and Go native APIs.
//
// Every call is schema-versioned (G-IFACE-02), permission-checked before it
// reaches a handler (G-SEC-02), and bounded in time (G-REL-01). Unknown
// methods fail with a deterministic error — never a hang (Module 2 §2.4).
package ipc
