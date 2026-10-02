# Failure Modes


## Expected behavior per failure

| Failure | Expected behavior |
|---|---|
| Svelte build fails | build stops with an explicit error |
| UI asset missing | startup fails with a clear diagnostic |
| JS exception | error is isolated and observable; runtime survives |
| IPC handler not found | deterministic error (code `-32601`) |
| Native API fails | error returned; no silent failure |
| Renderer init fails | application never reaches ready state |
| GPU/resource failure | bounded failure; diagnostics available |
| Window creation fails | startup fails explicitly |
| Invalid UI state | update rejected, not applied |
| Process crash | OS-level termination; persisted state stays valid |
| Interrupted operation | no partial persisted state |
| Unsupported CSS/UI feature | explicit unsupported error — never a silent misrender |

## Classification — classify before you fix

Every failure is classified **before** any code change:

| Type | Pattern | Response |
|---|---|---|
| **A — Implementation failure** | design correct, implementation buggy | fix implementation |
| **B — Design failure** | implementation correct, design insufficient | update docs/PLATFORM.md |
| **C — Missing guard rail** | system entered a forbidden state | add guard rail (docs/GUARDRAILS.md) |
| **D — Missing observability** | failure happened but cause is invisible | add measurement (docs/OBSERVABILITY.md) |
| **E — Incorrect acceptance criteria** | checklist passed but behavior unsafe | update docs/CHECKLISTS.md |

## Failure experiments

Executable experiments A–E live in [`tests/failure/`](../tests/failure/README.md); every run records a result in [`evidence/experiments/`](../evidence/experiments/README.md).
