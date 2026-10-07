# B — JS exception in a handler

- **Date:** 2026-10-07
- **Milestone / commit:** Milestone 4 (working tree on `bc12f0d`)
- **Environment:** Darwin 25.2.0, arm64, go1.25.5

## Setup

`tests/failure/experiment_b_js_test.go` builds a real dispatcher +
`script.New(script.DefaultLimits, disp)` engine, registers a healthy
control handler, then registers `boom` whose body throws
`new Error("handler exploded")`. The throw is fired via `FireHandler`
under a 10 s watchdog; a post-exception block re-evals a handler and
dispatches a native probe call, again under a watchdog.

## Observed behavior

- `FireHandler` returned within the watchdog with
  `script: handler:boom: handler exploded` (message preserved,
  `*script.Error`, no panic, process alive).
- `Metrics.JSExceptions = 1`, `LastJSEvalDuration` recorded.
- A `Diagnostic{Component: "script", Err: ...handler exploded...}`
  reached the capture reporter.
- After the exception: `Eval` succeeded, the `after` handler fired, and
  `Dispatch("app.probe")` returned success — engine and dispatcher both
  reusable within the watchdogs.

## Expected behavior

Module 5 §5.3 / Module 2 §2.4: an isolated, observable JS error —
counted in metrics, reported as a diagnostic, contained by the runtime;
the native side stays controlled and bounded (G-REL-02, G-REL-01).

## Verdict

Pass — classification: none (behavior as designed).

## Follow-up

None. Engine-level evidence for the checklist item "JS exception is
observable"; the scene-level twin lives in
`internal/app.TestSceneJSHandlerFailureIsObservable`.
