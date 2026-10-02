// @gowez/adapter — Strategy B: compiler-oriented UI runtime.
//
// The adapter translates Svelte compiler output into the GoWEZ UI
// instruction stream defined by protocol/ui-instruction.schema.json, so the
// Go runtime never depends on browser/DOM semantics.
//
// Milestone 5. Until then this module only pins the contract surface.

/** Options for the Svelte adapter. */
export interface AdapterOptions {
  /**
   * Target instruction schema version.
   * Matches `version` in protocol/ui-instruction.schema.json.
   */
  version?: number;
}

/** A configured adapter instance. */
export interface Adapter {
  readonly name: string;
  readonly version: number;
}

/**
 * Create the adapter.
 *
 * TODO(M5): hook the Svelte compiler, walk its AST/ir, and emit validated
 * UI instructions. Unsupported Svelte/browser behavior must be rejected at
 * compile time with an explicit error — never emitted as a silent
 * misrender (G-UPG-04).
 */
export function createAdapter(options: AdapterOptions = {}): Adapter {
  return {
    name: "gowez-adapter",
    version: options.version ?? 1,
  };
}
