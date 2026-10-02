# Experiment records

One file per executed failure experiment or validation run.

An experiment without a record in this directory did not happen (AGENTS.md, evidence rules).

File naming: `YYYY-MM-DD_<experiment-id>_<slug>.md` — e.g. `2026-10-03_a_renderer-init-failure.md`.

Template:

```markdown
# <Experiment ID> — <title>

- **Date:** YYYY-MM-DD
- **Milestone / commit:**
- **Environment:** OS, arch, Go version

## Setup
How the failure was induced.

## Observed behavior
Exact output, logs, metrics.

## Expected behavior
From Module 5 §5.3 / Module 2 §2.4.

## Verdict
Pass / Fail — and failure classification (A–E, Module 7).

## Follow-up
Issue / DRR / roadmap change, if any.
```
