# Acceptance Checklists


A checklist item passes only with a test, an experiment record, or a documented observation behind it. A checked box without evidence is classification **E**.

## Core correctness (M1–M3)

- [ ] Application starts from a clean environment
- [ ] Native window opens
- [ ] Svelte UI loads
- [ ] Text renders
- [ ] Basic shapes render
- [ ] Button renders
- [ ] Mouse click reaches UI
- [ ] Keyboard input works
- [ ] UI state can update
- [ ] JS exception is observable
- [ ] Application exits cleanly

## Svelte integration (M5)

- [ ] Svelte component can be compiled
- [ ] Component state can update
- [ ] Event handler works
- [ ] Conditional rendering works
- [ ] List rendering works
- [ ] Basic component composition works
- [ ] Required browser APIs are documented
- [ ] Unsupported Svelte/browser behavior is explicit

## IPC (M4/M6)

- [ ] UI can invoke Go API
- [ ] Go can return success
- [ ] Go can return error
- [ ] Unknown method fails deterministically
- [ ] IPC cannot invoke arbitrary native function
- [ ] IPC lifecycle is bounded

## Rendering (M1/M2)

- [ ] Layout produces deterministic geometry
- [ ] Renderer receives explicit commands
- [ ] Text renders correctly
- [ ] Basic clipping works
- [ ] Resize triggers relayout
- [ ] Renderer failure is observable

## Security (M4/M6)

- [ ] JavaScript cannot access arbitrary Go functions
- [ ] Native APIs are explicit
- [ ] Sensitive APIs can be permission controlled
- [ ] Production debug interfaces are disabled
- [ ] Dependencies are reviewed

## Release (M7)

- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Svelte integration tests pass
- [ ] Renderer tests pass
- [ ] IPC tests pass
- [ ] Failure experiments pass
- [ ] Basic benchmark exists
- [ ] Documentation updated

## MVP definition of done

The MVP counts as complete for **technical validation** when:

- [ ] A Svelte application builds successfully
- [ ] No Chromium dependency
- [ ] No OS WebView dependency
- [ ] The Go binary can create a native window
- [ ] Basic UI renders through the GPU
- [ ] The user can interact with the UI
- [ ] Svelte state produces UI updates
- [ ] JS can call explicit Go APIs
- [ ] Go APIs can access at least one native capability
- [ ] Main failure modes can be tested
- [ ] Startup/memory/rendering benchmarks exist
- [ ] Browser/Svelte compatibility limits are documented

Reaching these does **not** mean the framework is production-ready — it only proves the architectural hypothesis is worth continuing.
