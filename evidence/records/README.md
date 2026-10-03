# Decision Request Records (DRR)

Every Level B decision (see [AGENTS.md](../../AGENTS.md) → Decision authority) gets a record here **before** the change is made. Silence is not approval: a Level B change without a confirmed DRR is blocked.

## Index

| Record | Subject | Status |
|---|---|---|
| [2026-10-03_windowing-purego-sdl3.md](2026-10-03_windowing-purego-sdl3.md) | Windowing backend for M1: purego SDL3 (DRR-001) | ✅ confirmed |
| [2026-10-03_text-stack-gotext.md](2026-10-03_text-stack-gotext.md) | Text stack for M1: go-typesetting (DRR-002) | ✅ confirmed |
| [2026-10-03_module-path-rename.md](2026-10-03_module-path-rename.md) | Module path → `github.com/arief-fajri/gowez` (DRR-003) | ✅ confirmed |

File naming: `YYYY-MM-DD_<slug>.md`.

Template:

```markdown
# DRR — <title>

- **Date:** YYYY-MM-DD
- **Author:** (human or agent)
- **Status:** open / confirmed / rejected
- **Decision class:** B (requires explicit human confirmation)

## Context
What triggers this decision.

## Options considered
1. …
2. …

## Recommendation
Chosen option and why.

## Impact
Contract (protocol/, root API), guard rails (G-*), security, scope (MVP IN/OUT).

## Confirmation
Human approver + date. Required before implementation.
```
