# Failure experiments

Executable failure experiments. Each experiment must run against a real build and record its result in [`evidence/experiments/`](../../evidence/experiments/README.md).

| ID | Experiment | Expected outcome | Script | Status |
|---|---|---|---|---|
| A | Renderer initialization fails at startup | No half-initialized application; diagnostic available | `experiment_a_renderer_test.go` | ✅ executed 2026-10-03 — [record](../../evidence/experiments/2026-10-03_a_renderer-init-failure.md) |
| B | JS exception in a handler | Error isolated and observable; native runtime stays controlled | `experiment_b_js_test.go` (planned) | not implemented (M4) |
| C | IPC call to an unknown method | Deterministic error (code `-32601`), no hang | `experiment_c_ipc_test.go` (planned) | not implemented (M4) |
| D | Invalid native file/system operation | Error propagates; no partial persisted state | `experiment_d_native_test.go` (planned) | not implemented (M6) |
| E | Render workload pressure | Bounded degradation; frame time and resource usage observable | `experiment_e_resource_test.go` (planned) | not implemented (M5+) |

Rules:

- Experiments are `_test.go` files in this directory (run with `go test ./tests/failure/...`).
- Experiments that need milestones not yet implemented must call `t.Skip` with the milestone name — never silently pass.
- A run without a record in `evidence/experiments/` is not evidence.
