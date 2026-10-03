# Benchmarks


Benchmarks characterize GoWEZ against baselines; they are **not** a claim of general superiority. Compare only equivalent applications in equivalent environments (Milestone 7).

## Baselines

- GoWEZ (this repository)
- Electron
- Tauri

## Measured characteristics

| Metric | Why it matters |
|---|---|
| Binary size | deployment footprint (thesis) |
| Packaged application size | distribution cost |
| Startup time | P5 operational behavior |
| Idle memory | runtime efficiency |
| CPU idle | background cost |
| Rendering latency | responsiveness |
| Time-to-first-frame | perceived startup |
| Application responsiveness | interaction quality |

## M1 micro-baseline (internal, not a cross-tool comparison)

Provisional numbers from package-level benchmarks on **Apple M2, macOS, Go 1.25.5, 2026-10-03** — recorded in [`evidence/learnings.md`](../evidence/learnings.md). These characterize the software backend for regressions; they are **not** Electron/Tauri comparisons (those land in Milestone 7).

| Benchmark | Result | Notes |
|---|---|---|
| `BenchmarkShape` (text) | ~3.6 µs/op | full sentence, HarfBuzz shaping |
| `BenchmarkShapeShort` | ~1.1 µs/op | counter-style short string |
| `BenchmarkDraw` (text raster) | ~218 µs/op | shaping + rasterization to RGBA |
| `BenchmarkFrame` (scene) | ~4.2 ms/frame | 640×420 hello scene, 3 text runs — ~25% of the 16.7 ms @60 fps budget |

Run them with `go test ./... -run '^$' -bench .` (see [DEVELOPMENT_GUIDE §1](../DEVELOPMENT_GUIDE.md#1-commands)).

## Location

Comparative suites live in [`tests/bench/`](../tests/bench/) and land with Milestone 7; M1 micro-benchmarks live in-package (`internal/text`, `internal/render/backend/software`). Results are recorded as experiment artifacts in [`evidence/experiments/`](../evidence/experiments/README.md).

## Rules

- Same application, same workload, same hardware for every baseline.
- Record environment (OS, arch, versions) with every run.
- A benchmark run without a recorded environment is not evidence.
