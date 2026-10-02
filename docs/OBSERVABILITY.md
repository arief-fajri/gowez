# Observability


Guard rails hold only if they can be measured. A system property may not change without a defined way to observe it (P5).

## Runtime metrics (Module 5 §5.1)

| Area | Metrics | Owner package |
|---|---|---|
| Startup | duration, initialization failures | `internal/app`, `internal/observe` |
| Rendering | frame time, dropped frames, render errors | `internal/render`, `internal/observe` |
| JS | execution errors/duration, uncaught exceptions | `internal/script`, `internal/observe` |
| IPC | invocation count, error count, duration | `internal/ipc`, `internal/observe` |
| Resource | memory, CPU, GPU/resource failures | `internal/observe` |

Collection surface: `observe.Recorder` (`internal/observe`). Wiring each event happens with its milestone.

## Failure experiments (Module 5 §5.3)

| ID | Experiment | Proves |
|---|---|---|
| A | renderer init failure | G-REL-03, no half-initialized app |
| B | JS exception | G-REL-02, isolation |
| C | unknown IPC method | deterministic error, bounded call |
| D | invalid native operation | error propagation, no partial state |
| E | resource pressure | bounded degradation |

Location: [`tests/failure/`](../tests/failure/README.md) → records in [`evidence/experiments/`](../evidence/experiments/README.md).

## Diagnostics

Every failure mode in [FAILURE-MODES.md](./FAILURE-MODES.md) must surface through `observe.Diagnostic` (component + message + error). A failure with no diagnostic is classification **D**.
