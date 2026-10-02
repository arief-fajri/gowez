# Platform Design


## Architecture principles

| | Principle |
|---|---|
| **P1** | Rendering runtime is a first-class concern — HTML/CSS does not imply a browser |
| **P2** | Svelte is an authoring/compile-time concern, not a runtime |
| **P3** | Fail boundedly — no native/IPC/JS operation waits forever |
| **P4** | Correctness over availability — explicit failure over silent wrong state |
| **P5** | Operational behavior (startup, shutdown, crash, upgrade) is product behavior |

## Invariants

### Data / state
- **I1** — a failed state update never leaves partial, inconsistent state
- **I2** — UI state and application state have a clear boundary
- **I3** — a failed native operation returns an observable error

### Interface / contract
- **I4** — the Go ↔ UI API has an explicit contract (`protocol/`)
- **I5** — breaking changes to the public runtime API are documented and versioned
- **I6** — event semantics never change silently

### Security
- **I7** — UI/JS never gets arbitrary access to the Go runtime
- **I8** — native capability only through explicit APIs
- **I9** — sensitive native operations are restricted by the permission model

### Resource
- **I10** — JS execution has a stoppable lifecycle
- **I11** — the render loop has a bounded lifecycle
- **I12** — shutdown releases native resources predictably

## Desired outcomes

1. **Native/runtime integrated** — Go is the first-class runtime.
2. **Compatible where promised** — only the documented subset; never full browser compat.
3. **Usable** — author components, run, handle state, call Go APIs.
4. **Secure by default** — no implicit native access.
5. **Observable** — startup, render, JS, IPC, native, resource, crash.
6. **Recoverable** — failures never corrupt persisted state.
7. **Upgrade-safe** — versioned runtime/UI contracts.
8. **Predictable** — bounded memory, event queue, render load, JS time, handles.

## Control flow

- **Startup** — config → window → renderer → JS runtime → UI bundle → UI tree → layout → render → ready; any failed step aborts explicitly.
- **Interaction** — input → hit test → UI node → JS handler → state change → UI update → layout → render commands → GPU.
- **Native operation** — UI → `app.invoke` → IPC dispatcher → permission gate → Go handler → OS → response.

**Success boundary:** validation ✓, native op ✓, persistence committed ✓, response returned ✓.
