# DRR — Text stack for Milestone 1: go-text (typesetting + render + gfonts)

- **Date:** 2026-10-03
- **Author:** agent (M1 plan execution)
- **Status:** confirmed
- **Decision class:** B (requires explicit human confirmation — new dependency)

## Context

M1 must shape and rasterize text for `cmd/gowez-hello` (title, counter value, label) with deterministic output for golden tests, without cgo and without a browser. Hard rule 6: unsupported features fail explicitly — so the chosen stack must have a clear, documentable coverage boundary (Latin/basic shaping first, not full HarfBuzz/OpenType parity).

Candidates researched 2026-10-03:

1. **`github.com/go-text/typesetting`** — pure-Go font loading + glyph shaping (harfbuzz port + fallback), BSD-3, actively maintained by the go-text org; supports harfbuzz-style shaping, font collection, and glyph runs. Composes with:
   - **`github.com/go-text/render`** — pure-Go rasterizer (rasterx) that renders shaped runs to an `image.RGBA`, and
   - **`github.com/golang/freetype/truetype`** (gofont/goregular ships with go-text ecosystem) — embedded Go fonts for deterministic goldens.
2. **`github.com/go-gl/freetype` / `freetype/truetype` only** — no shaping (no ligatures, no Indic, no bidi): violates the fail-explicitly boundary and can't represent future shaping needs.
3. **CGO HarfBuzz/Pango bindings** — full fidelity but requires cgo + system libs on every platform: kills the build story (same objection as GLFW, Hard rule 1-adjacent: no native toolchain dependency for the MVP).

## Recommendation

Option 1, three modules, all pure Go:

- `github.com/go-text/typesetting` — shaping (`shaping.HarfbuzzShaper` / font file parsing via `opentype` APIs).
- `github.com/go-text/render` — rasterize shaped output into `image.RGBA` (this is what `internal/text` will wrap).
- `github.com/go-text/typesetting/font/opentype` + `gofont/goregular` (bundled with go-text) — deterministic fallback font embedded for tests and the M1 demo.

Boundary to document (G-UPG-04, Hard rule 6): M1 supports Latin script + basic shaping via the harfbuzz port; complex-script coverage is validated later (M4+); unsupported input fails explicitly (error), never silently drops glyphs.

## Impact

- **Contract:** `protocol/` untouched, root API untouched. `internal/text` is new/internal (Level A): `LoadFont`, `Measure`, `Shape`, `Rasterize` — consumed by M1 scene and golden tests.
- **Guard rails:** G-DEP-05 (bounded: shaping runs are per-frame, sizes capped), G-UPG-04 (unsupported = explicit error), no cgo anywhere (keeps `go build ./...` and `CGO_ENABLED=0` story intact).
- **Security:** none (no script/IPC/permission surface).
- **Scope:** MVP IN unchanged — text rendering was always M1 scope.

## Confirmation

Human approver: **user (human operator) — 2026-10-03** ("Confirm both"). Approved: option 1 (go-text/typesetting + render + gofont) as recorded.
