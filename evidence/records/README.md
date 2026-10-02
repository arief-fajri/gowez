# Decision Request Records (DRR)

Every Level B decision (see [AGENTS.md](../../AGENTS.md) → Decision authority) gets a record here **before** the change is made. Silence is not approval: a Level B change without a confirmed DRR is blocked.

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
