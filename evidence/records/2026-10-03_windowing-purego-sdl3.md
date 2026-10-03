# DRR — Windowing backend for Milestone 1: purego SDL3 (Zyko0/go-sdl3)

- **Date:** 2026-10-03
- **Author:** agent (M1 plan execution)
- **Status:** confirmed
- **Decision class:** B (requires explicit human confirmation — new dependency)

## Context

Milestone 1 needs a real OS window with event polling on Windows, macOS and Linux, without Chromium/WebView (Hard rule 1). The root `gowez` API must not widen; `internal/window` is internal and may be reworked freely, but the choice introduces a new Go dependency (Level B: "new dependencies").

Three candidates were researched 2026-10-03:

1. **`github.com/Zyko0/go-sdl3` v0.1.1** — MIT, purego (no cgo), embeds prebuilt SDL3 shared libraries for 6 targets (`{win,mac,linux} × {amd64,arm64}`) via `bin/binsdl`, error-based API, generated from SDL 3.2.2, last release Apr 2026, 11 importers.
2. **`github.com/JupiterRider/purego-sdl3`** — Unlicense, purego, but requires a system-installed SDL3 (`libsdl3.dylib`/`.so`/`.dll`); no bundled binaries → manual user install step.
3. **`github.com/go-gl/glfw` v3.4** — mature, actively maintained (Aug 2026), but **requires cgo** and a C toolchain on every build machine; violates the Go-native build story (Go 1.25 cross-compile becomes impossible without per-platform C toolchains).

### Spike evidence (T0, 2026-10-03, darwin/arm64, this machine)

Throwaway program (`bin/binsdl.Load` → `Init(INIT_VIDEO)` → `CreateWindowAndRenderer` → streaming texture upload → `RenderTexture` → `Present` → `PollEvent` loop):

```
$ CGO_ENABLED=0 ./spike_nocgo
vsync=1
OK window logical=640x480 pixels=640x480
renderer backend="metal"
frames=3333 elapsed=2s fps=1666.4 err=timeout closeEvent=false
exit=0
```

Findings:

- **F1** purego path works with `CGO_ENABLED=0` on darwin/arm64 — window opens, backend = Metal, streaming `PIXELFORMAT_RGBA32` texture upload + present OK, event poll OK.
- **F2** `binsdl` ships binaries for all 6 M1 targets (`binary_{darwin,linux,windows}_{amd64,arm64}.go` in module cache).
- **F3** **VSync is not honored**: `renderer.SetVSync(1)` succeeds and `renderer.VSync()` reports 1, yet `Present` never blocks (≈1600 fps unthrottled). The frame loop **must pace itself in software** (time-based interval), with vsync as best-effort nicety only. This is a design input, not a contract: nothing above `internal/window` may depend on vsync blocking.
- **F4** `window.SizeInPixels()` available → framebuffer can be sized in physical pixels (HiDPI-correct); on this machine pixel size == logical size (1x scale).

## Options considered

1. **Adopt `Zyko0/go-sdl3` v0.1.1 (purego + embedded SDL3 libs).** ✓ recommended.
2. Adopt `JupiterRider/purego-sdl3` (needs system SDL3). Fallback if (1) fails on a target platform — no API change needed above `internal/window`, only dependency swap.
3. Adopt `go-gl/glfw` v3.4 (cgo). Second fallback; requires a new Level B confirmation (cgo toolchain requirement is a scope/build change).

## Recommendation

Option 1, with a documented **fallback chain** recorded in this DRR:

- Primary: `github.com/Zyko0/go-sdl3 v0.1.1`
- Fallback A: system SDL3 + `github.com/JupiterRider/purego-sdl3` (spike only; re-opens this DRR)
- Fallback B: `go-gl/glfw` + GL frame blit (re-opens this DRR as Level B — cgo)

Rationale: zero-cgo on all 6 targets, self-contained binaries (no user install), MIT license, error-based idiomatic API, closest mapping to the `internal/window` contract (`Pump`, `Present(pixels, w, h)`, `PixelSize`).

## Impact

- **Contract:** `protocol/` untouched. Root `gowez` API untouched. `internal/window` contract reworked (internal, Level A): drop `Events() <-chan`, add `Pump() []Event`, `Present(pixels []byte, w, h int) error`, `PixelSize() (w, h int)`; per-OS files `darwin.go`/`win32.go`/`linux.go` collapse into one `sdl.go` under build tag `(darwin||linux||windows) && (amd64||arm64)` plus `fallback.go`.
- **Guard rails:** G-DEP-05 (bounded ops) — event pump and present are per-tick, no blocking calls in the loop. F3 (no vsync dependency) feeds docs/OBSERVABILITY.md pacing metric. Hard rule 1 holds (no Chromium/WebView).
- **Security:** none (no script/IPC surface touched; G-SEC unaffected).
- **Scope:** MVP IN unchanged — windowing was always M1 scope.

## Confirmation

Human approver: **user (human operator) — 2026-10-03** ("Confirm both"). Approved: option 1 + fallback chain as recorded.
