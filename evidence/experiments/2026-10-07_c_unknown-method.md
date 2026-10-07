# C — IPC call to an unknown method

- **Date:** 2026-10-07
- **Milestone / commit:** Milestone 4 (working tree on `bc12f0d`)
- **Environment:** Darwin 25.2.0, arm64, go1.25.5

## Setup

`tests/failure/experiment_c_ipc_test.go` builds a real dispatcher +
script engine with **no** handler registered for `app.nonexistent` and
calls it twice under one 10 s watchdog:

1. Go path: `disp.Dispatch(ctx, Request{Method: "app.nonexistent"})`.
2. JS path: a `gowez.on("probeUnknown", ...)` handler invokes
   `gowez.invoke("app.nonexistent")` and asserts in-script that the
   thrown Error carries `e.code === -32601`; a wrong/missing code throws,
   which `FireHandler` would report.

## Observed behavior

- Watchdog not tripped — both paths settled (no hang).
- Go path: `Error.Code = -32601`, message
  `unknown method "app.nonexistent"` (names the method).
- JS path: `FireHandler` returned `nil` — the script-side assertion held,
  so `.code = -32601` reached the script layer intact. The failure is an
  IPC rejection (`Metrics.IPCErrorCount >= 1`), **not** a JS crash
  (`JSExceptions = 0`).

## Expected behavior

Guard-rail contract: unknown methods fail deterministically with
`-32601`, never hang (G-REL-01); failures are observable in metrics
(P5); `protocol/ipc.schema.json` documents the code. Version mismatch
maps to `-32600` (DRR addendum, 2026-10-07).

## Verdict

Pass — classification: none (behavior as designed).

## Follow-up

None. Covered statically as well by
`internal/ipc/dispatcher_test.go`
(`TestDispatchUnknownMethodIsDeterministic`) and
end-to-end in `internal/app.TestSceneJSDrivesStateThroughIPC`.
