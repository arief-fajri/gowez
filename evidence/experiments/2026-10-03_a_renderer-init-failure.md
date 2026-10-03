# A — Bring-up failure at startup (half-init guard)

- **Date:** 2026-10-03
- **Milestone / commit:** M1 (working tree; repository not under VCS yet)
- **Environment:** macOS darwin/arm64 (Apple M2), Go 1.25.5, GoWEZ `go test ./tests/failure`

## Setup

`tests/failure/experiment_a_renderer_test.go` injects a startup fault
through the `app.NewWindow` seam (factory returns
`injected: graphics device unavailable`) and swaps `app.NewReporter`
for a capturing reporter. `app.Run` is executed with a hard 10-second
deadline.

M1's software renderer constructor (`software.New`) cannot fail, so
the injection sits at the first failable startup step after
config/font — window bring-up — which exercises the same Module 2
§2.3 sequence. The experiment file keeps the README's experiment A
name for continuity; re-pointing it at a real renderer fault is a
follow-up for M1-GPU.

## Observed behavior

```
$ go test ./tests/failure/ -run TestAStartupFailureIsExplicit -count=1 -v
=== RUN   TestAStartupFailureIsExplicit
--- PASS: TestAStartupFailureIsExplicit (0.00s)
PASS
ok  	github.com/arief-fajri/gowez/tests/failure	3.380s
```

> **Editorial note (2026-10-03):** the module path in the quoted
> output was `github.com/volantisfrontend/gowez` when this run was
> recorded; it was reprinted after the module-path rename to
> `github.com/arief-fajri/gowez` (see
> [DRR-003](../records/2026-10-03_module-path-rename.md)). Test
> content, duration, and result are unchanged.

- `app.Run` returned an error wrapping the injected cause
  (`errors.Is(err, injected)` ✓), naming the `window` component.
- One diagnostic captured with `Component == "window"` and the
  wrapping error (P5: failure observable on stderr in production
  wiring).
- Returned in ≪ 10s — no hang (G-REL-01).
- Nothing was half-initialized: the injected factory returned a nil
  window, and no later startup step ran.

## Expected behavior

From tests/failure README row A: "No half-initialized application;
diagnostic available." From Module 2 §2.4: startup failures are
explicit, never half-initialized.

## Verdict

Pass — the guard rail held; no failure to classify (A–E applies when
the system misbehaves).

## Follow-up

- M1-GPU: re-target this experiment at a renderer that can actually
  fail to initialize.
- The two exported seams (`app.NewWindow`, `app.NewReporter`) must be
  restored by every test (defer pattern); a future refactor may
  replace them with explicit dependency injection on `app.Options`.
