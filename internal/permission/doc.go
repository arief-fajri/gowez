// Package permission implements the explicit grant model for native
// capabilities (invariants I7–I9, guard rails G-SEC-01/G-SEC-02/G-SEC-03).
//
// UI/JavaScript never holds implicit access to native operations: a call is
// allowed only when its permission was granted at startup. Absence of a
// grant means denial — secure by default (Module 1 §1.4 outcome 4).
package permission
