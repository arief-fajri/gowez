# Observability


Guard rails hold only if they can be measured. A system property may not change without a defined way to observe it (P5).

## Runtime metrics (Module 5 §5.1)

| Area | Metrics | Owner package | Wired |
|---|---|---|---|
| Startup | duration, initialization failures | `internal/app`, `internal/observe` | M1 ✓ (`StartupDuration`, diagnostics) |
| Rendering | frame time, dropped frames, render errors | `internal/render`, `internal/observe` | M1 ✓ (`FrameCount`, `DroppedFrames`) |
| Layout | pass count, last pass duration | `internal/app`, `internal/observe` | M2 ✓ (`LayoutCount`, `LastLayoutDuration`) |
| Input | input events, dispatched handlers, unhandled events, recovered handler panics | `internal/app`, `internal/ui`, `internal/observe` | M3 ✓ (`InputEvents`, `DispatchedEvents`, `UnhandledEvents`, `HandlerPanics`) |
| JS | eval/handler duration, isolated exceptions | `internal/script`, `internal/observe` | M4 ✓ (`JSExceptions`, `LastJSEvalDuration`, `Diagnostic{Component:"script"}`) |
| IPC | invocation count, error count, duration | `internal/ipc`, `internal/observe` | M4 ✓ (`IPCCount`, `IPCErrorCount`, `LastIPCDuration`) |
| Resource | memory, CPU, GPU/resource failures | `internal/observe` | M5+ |

Collection surface: `observe.Recorder` (`internal/observe`). Wiring each event happens with its milestone.

## Failure experiments (Module 5 §5.3)

| ID | Experiment | Proves | Status |
|---|---|---|---|
| A | renderer init failure | G-REL-03, no half-initialized app | ✅ executed 2026-10-03 — [record](../evidence/experiments/2026-10-03_a_renderer-init-failure.md) |
| B | JS exception | G-REL-02, isolation | ✅ executed 2026-10-07 — [record](../evidence/experiments/2026-10-07_b_js-handler-exception.md) |
| C | unknown IPC method | deterministic error, bounded call | ✅ executed 2026-10-07 — [record](../evidence/experiments/2026-10-07_c_unknown-method.md) |
| D | invalid native operation | error propagation, no partial state | pending (M6) |
| E | resource pressure | bounded degradation | pending (M5+) |

Location: [`tests/failure/`](../tests/failure/README.md) → records in [`evidence/experiments/`](../evidence/experiments/README.md).

## Diagnostics

Every failure mode in [FAILURE-MODES.md](./FAILURE-MODES.md) must surface through `observe.Diagnostic` (component + message + error). A failure with no diagnostic is classification **D**.
