# Idea & Direction Validation


## Problem

Developers who want web-style UI (HTML/Svelte authoring) for desktop apps today must either ship Chromium (Electron) or depend on the OS WebView (Tauri). Both are outside the application's control.

## Evidence status

Only a technical hypothesis exists. Market/user evidence is **not** collected. Milestones must not assume market demand (Module 0 §2 honesty check).

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
