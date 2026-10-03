# Learnings

Append-only log of things learned during implementation. After every non-trivial change (see [AGENTS.md](../AGENTS.md)), append an entry: what was tried, what actually happened, what changed in the plan.

Format:

```markdown
## YYYY-MM-DD — <short title>

- **Change:** what was implemented or attempted
- **Observation:** what actually happened (measurements, test output)
- **Implication:** what it means for the plan / guard rails / roadmap
- **Evidence:** link to evidence/experiments/ or evidence/records/ artifact
```

---

## 2026-10-03 — SDL3 via purego works; vsync does not block

- **Change:** T0 spike: throwaway program using `Zyko0/go-sdl3` (binsdl + streaming texture + PollEvent loop), run with and without cgo.
- **Observation:** Window opens on darwin/arm64, backend `metal`, `CGO_ENABLED=0` OK, 3193 frames/2s. `renderer.SetVSync(1)` succeeds and `VSync()` reports 1, yet Present never blocks (≈1600 fps unthrottled).
- **Implication:** The app loop owns frame pacing (`frameBudget = 1/60s` in `internal/app`); nothing above `internal/window` may depend on vsync blocking. Recorded as finding F3 in DRR-001; `SetVSync` kept as best-effort.
- **Evidence:** [DRR-001](records/2026-10-03_windowing-purego-sdl3.md)

## 2026-10-03 — go-text shaping: RunEnd is mandatory; size is fixed.I(px)

- **Change:** `internal/text` implementation on `go-text/typesetting` + `go-text/render` + gofont.
- **Observation:** Omitting `Input.RunEnd` silently shapes an empty run (0 glyphs, advance 0 — the probe's first output). `Size: fixed.I(n)` is n pixels: "Hello" @16px = 37.94px, @32px = 75.83px (linear). `HarfbuzzShaper` zero value is usable; LRU cache documented for reuse.
- **Implication:** `Shape` always sets `RunStart/RunEnd`; tests pin width ratio and glyph counts so a regression to empty runs fails loudly.
- **Evidence:** `internal/text/text_test.go`, probe in `/tmp/textprobe`

## 2026-10-03 — image.RGBA is premultiplied; goldens must not cross color models

- **Change:** Software rasterizer fill path and `tests/golden` comparison.
- **Observation:** Storing straight-alpha bytes in `image.RGBA` (an alpha-0.5 rect) made `png.Encode` un-premultiply garbage → golden mismatch `rgba(50,128,102,128)` vs `rgba(51,128,230,128)` at the same pixel. Comparing decoded `color.NRGBA` (golden) against `color.RGBA` (buffer) via `.RGBA()` is lossy for A<255 and produced false failures even after the fill was fixed. Classification: **A — implementation failure** (fill wrote straight alpha into a premultiplied buffer; latent text-over-rect blend bug) plus an incorrect test comparison.
- **Implication:** `fillRect` now premultiplies with integer round-to-nearest (`premul`); golden tests compare **encoded PNG bytes** (both sides through the same encoder); structural alpha assertions live in unit tests (`TestFillRectPremultiplied`). Scenes must paint opaque backgrounds — written into the `window.Present` contract.
- **Evidence:** `internal/render/backend/software/raster.go`, `renderer_test.go`, `tests/golden/golden_test.go`

## 2026-10-03 — SDL/Cocoa needs the OS main thread; go test functions don't run there

- **Change:** `tests/integration` real-window checks.
- **Observation:** `window.New` from a test function failed with `No available video device`; a standalone probe confirmed SDL_Init fails from a worker goroutine and succeeds on the main goroutine with `runtime.LockOSThread()` (also after `m.Run` parks it). go test executes each `Test` in its own goroutine — `TestMain` runs on the main one.
- **Implication:** GUI lifecycle checks live in `TestMain` before `m.Run`; the `-tags integration` flag keeps them out of default runs; `gowez-hello` must call `gowez.Run` straight from `main()`. Documented in the window package and DEVELOPMENT_GUIDE §1/§4.
- **Evidence:** `tests/integration/window_test.go`

## 2026-10-03 — Experiment A passes; startup faults need seams

- **Change:** Failure experiment A implemented with exported seams `app.NewWindow` / `app.NewReporter`; run recorded.
- **Observation:** Injected bring-up failure returned within the 10s bound, wrapped the cause, and produced a `Component == "window"` diagnostic; no later startup step ran.
- **Implication:** The half-init guard (I1/I2) and observability (P5) hold for M1's sequence. Seams are a temporary device — replace with explicit DI if a third consumer appears.
- **Evidence:** [experiment A record](experiments/2026-10-03_a_renderer-init-failure.md)

## 2026-10-03 — M1 render budget: 4.2 ms/frame at 640×420 (Apple M2)

- **Change:** Micro-benchmarks in `internal/text` and `internal/render/backend/software`.
- **Observation:** `BenchmarkFrame` = 4.23 ms/op (three text runs dominate; `BenchmarkShape` = 3.6 µs for a full sentence, 1.1 µs for a counter string; command recording = 53 ns). Budget at 60 fps = 16.7 ms.
- **Implication:** The CPU reference backend holds ~25% of the frame budget for the M1 scene; `DroppedFrames` (overrun > 1.5× budget) is wired into `observe.Recorder`. Text rasterization is the hot path for M2+ optimization, not fills.
- **Evidence:** `go test ./internal/text ./internal/render/backend/software -run '^$' -bench .` output (2026-10-03, Apple M2)

## 2026-10-03 — SIGINT/SIGTERM arrive ignored by the SDL/Cocoa stack; the app must install its own handler

- **Change:** T11 smoke verification of `cmd/gowez-hello`.
- **Observation:** `kill -TERM` and `kill -INT` (with dispositions explicitly reset to `SIG_DFL` in the child via `preexec_fn` before exec) were both ignored — only `SIGKILL` stopped the process. Baseline Go programs respond to these signals, so the ignore originates in SDL3/Cocoa inside the process (inherited-`SIG_IGN` ruled out by the `preexec_fn` test).
- **Classification:** **A — implementation failure.** The design (loop exits on CloseEvent only) was insufficient: no code path existed for signal-driven shutdown, so the runtime inherited an un-killable process — a violation of the spirit of I12/G-REL-04 (predictable shutdown), even though the window-close path itself was correct.
- **Implication:** `app.loop` now installs `signal.Notify(SIGINT, SIGTERM)` (buffered channel, drained each frame → latency ≤ one frame budget, `signal.Stop` on return restores default dispositions). Graceful shutdown shares the CloseEvent path and exits 0. Verified: both signals → clean exit rc=0, empty stderr. Cross-compiles on windows/freebsd (`syscall.SIGTERM` exists there). A `SIG_DFL` reset is done in tests that need to prove handler behavior.
- **Evidence:** `internal/app/lifecycle.go` (loop sigCh), probe: `python3` subprocess with `preexec_fn=SIG_DFL` → post-fix `SIGTERM exited rc=0`, `SIGINT exited rc=0`

## 2026-10-03 — Module path renamed to match the git remote

- **Change:** `github.com/volantisfrontend/gowez` → `github.com/arief-fajri/gowez` across `go.mod`, 16 `.go` files (24 occurrences), `DEVELOPMENT_GUIDE.md`; evidence experiment quote reprinted with an editorial note (user's chosen policy).
- **Observation:** The declared module path disagreed with the git remote (`arief-fajri/gowez.git`). Pre-release state (1 commit, 0 tags, 0 consumers, no `go.sum` reference) made the rename zero-cost; after this, `rg volantisfrontend` matches only deliberate historical mentions inside `evidence/` — code, docs, and tests are clean.
- **Implication:** The root public API address is now final until the first tagged release; any future change becomes a breaking/versioned decision (G-IFACE). Editorial note policy recorded so future evidence edits stay explicit.
- **Evidence:** [DRR-003](records/2026-10-03_module-path-rename.md)

## 2026-10-03 — M1 gate closed

- **Change:** Documentation sweep marking Milestone 1 complete: README status + roadmap status column + milestone-annotated tree; CHECKLISTS M1 boxes ticked with inline evidence links (9 core/rendering + 3 MVP DoD); OBSERVABILITY "Wired"/"Status" columns; GUARDRAILS enforcement annotations (G-DEP-03, G-REL-03, G-REL-04) + first changelog entry (module-path rename); DEVELOPMENT_GUIDE milestone status column (M1 row corrected to software-only scope); AGENTS completion notes; failure/IDEA-VALIDATION/ARCHITECTURE/BENCHMARKS status updates; DRR + experiment indexes; `app.ErrNotImplemented` message de-milestoned.
- **Observation:** Operator visually verified `cmd/gowez-hello` (window opens, text renders, resize works, clean exit). Final battery green: `gofmt` empty, `go vet` clean, `go test ./...` ok, `-race` ok, integration ok, 0 broken markdown links, no stale "nothing is runnable" claims, module path consistent outside deliberate historical mentions. Checklist stays honest: Svelte/input/layout/GPU/Release boxes remain unchecked — they are M2+ gates.
- **Implication:** M1 is the first closed milestone under the system-thinking loop: design (PLATFORM) → guard rails (experiment A) → implement → observe (benchmarks, metrics) → evaluate (checklist) → record (this log). Future milestone closures follow the same sweep: tick only evidence-backed boxes, add a gate-closed entry here.
- **Evidence:** [CHECKLISTS](../docs/CHECKLISTS.md), [README roadmap](../README.md#roadmap), [experiment A](experiments/2026-10-03_a_renderer-init-failure.md), DRRs 001–003
