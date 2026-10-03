# DRR — Module path rename: `volantisfrontend/gowez` → `arief-fajri/gowez`

- **Date:** 2026-10-03
- **Author:** agent (M1 plan execution, on user request)
- **Status:** confirmed
- **Decision class:** B (requires explicit human confirmation — root public API identity)

## Context

The module was declared as `github.com/volantisfrontend/gowez`
(everywhere: `go.mod`, all internal imports, `DEVELOPMENT_GUIDE.md`),
but the repository's git remote is
`arief-fajri/gowez.git` — i.e. the code
actually lives under `github.com/arief-fajri/gowez`. The user spotted
the inconsistency and confirmed the intended path.

The module path **is** the address of the root public API (import path
every application will use), so renaming it is a Level B decision
(G-IFACE: root public API), even though practically nobody is affected
yet:

- Repository state: 1 commit (`1769ef6 chore: initial commit`) +
  uncommitted M1 work; no tags, no releases, no external consumers.
  (SHAs rewritten 2026-10-05: author-identity mailmap cleanup +
  root commit message typo fix; original SHA `f36fc57`.)
- `go.sum` does not reference the main module path.
- Renaming now costs zero; renaming after first `go get` consumers
  exist would be a breaking change requiring a version bump.

## Options considered

1. **Rename module path to `github.com/arief-fajri/gowez`.** ✓
   recommended — matches the remote, zero consumers to break.
2. Keep `github.com/volantisfrontend/gowez` and move the repository
   under a `volantisfrontend` GitHub org later. Rejected: the org/repo
   does not exist and there is no plan to create it; every future
   clone would carry a path that points nowhere.
3. Rename to a vanity/custom domain (`gowez.dev/…`). Rejected: adds
   hosting + `go-import` meta requirements for zero benefit at this
   stage.

## Recommendation

Option 1. Mechanical string replacement of
`github.com/volantisfrontend/gowez` → `github.com/arief-fajri/gowez`
across `go.mod`, all `.go` imports (16 files, 24 occurrences), and
`DEVELOPMENT_GUIDE.md`.

**Evidence handling (user decision, 2026-10-03):** the historical
quote in `evidence/experiments/2026-10-03_a_renderer-init-failure.md`
is reprinted with the new path **plus an explicit editorial note**
recording the original path and the reason — chosen over strict
immutability so no stale pointer to a non-existent module remains in
the documentation surface.

## Impact

- **Contract:** `protocol/` untouched. The root `gowez` package API
  (types, functions) untouched — only the import path string changes.
  Future consumers run `go get github.com/arief-fajri/gowez`.
- **Guard rails:** G-IFACE satisfied — rename is documented here, is a
  deliberate version-boundary decision made while the module has zero
  consumers, and will be regression-checked by a full build/test pass.
- **Security:** none (no dependency or permission surface touched).
- **Scope:** MVP IN/OUT unchanged.
- **Git:** remote already correct; no push/commit performed as part of
  this change (working tree only).

## Confirmation

Human approver: **user (human operator) — 2026-10-03**
("eksekusi", following confirmation of path
`github.com/arief-fajri/gowez` and of the evidence-reprint policy).
