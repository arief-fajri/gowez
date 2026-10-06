# Idea & Direction Validation


## Problem

Developers who want web-style UI (HTML/Svelte authoring) for desktop apps today must either ship Chromium (Electron) or depend on the OS WebView (Tauri). Both are outside the application's control.

## Evidence status

**Technical evidence (M1, 2026-10-03):** the second half of the validation target below is proven — a pure-Go stack (no cgo) opens a native OS window, shapes and rasterizes text, and presents frames through the runtime's own software renderer ([DRR-001](../evidence/records/2026-10-03_windowing-purego-sdl3.md), [DRR-002](../evidence/records/2026-10-03_text-stack-gotext.md), [checklist](CHECKLISTS.md)).

**UI runtime (M2, 2026-10-05):** the middle of the target — `UI representation → Go runtime → UI tree/layout → renderer` — is proven end to end: UI tree, CSS subset, block/flex layout, and paint, pinned by golden `tests/golden/testdata/ui.png` ([CSS subset](CSS-SUBSET.md), [checklist](CHECKLISTS.md)).

**Interaction (M3, 2026-10-06):** input → hit test → event dispatch → handler → state update → re-layout works in the live scene (mouse, keyboard, `:hover`/`:active`/`:focus`, focus traversal), pinned by golden `tests/golden/testdata/ui-state.png`; operator visual verification confirmed 2026-10-06 ([EVENTS](EVENTS.md), [checklist](CHECKLISTS.md)).

The Svelte → compile → UI-representation half remains a hypothesis until M5.

Market/user evidence is **not** collected. Milestones must not assume market demand (Module 0 §2 honesty check).

## Validation target (MVP minimum)

```text
Svelte → compile → UI representation → Go runtime → UI tree/layout
       → GPU renderer → native window
```

Full HTML/CSS/Web API support is **not** a target.

## Key risks

- **R1** — Svelte usable without browser/WebView runtime
- **R2** — custom renderer suffices for desktop needs
- **R3** — CSS/layout complexity does not turn this into a browser engine
- **R4** — developer experience competitive with Electron/Tauri

## Decision

**GO for technical validation / MVP research only.** Re-evaluate if Svelte integration or rendering proves infeasible.
