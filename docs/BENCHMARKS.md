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

## Location

Suites live in [`tests/bench/`](../tests/bench/) and land with Milestone 7. Results are recorded as experiment artifacts in [`evidence/experiments/`](../evidence/experiments/README.md).

## Rules

- Same application, same workload, same hardware for every baseline.
- Record environment (OS, arch, versions) with every run.
- A benchmark run without a recorded environment is not evidence.
