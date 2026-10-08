/**
 * The instruction op stream: the adapter's output shape and its emitter.
 *
 * The shape mirrors protocol/ui-instruction.schema.json. Node ids are the
 * adapter's own address space, allocated by IdGen; the Go runtime maps them to
 * its internal numbering, so nothing here depends on runtime numbering.
 */
import type { Finding, Report } from './findings.js';

export type OpKind =
  | 'createElement'
  | 'createText'
  | 'setAttribute'
  | 'setStyle'
  | 'setText'
  | 'appendChild'
  | 'removeChild'
  | 'addEventListener'
  | 'removeEventListener';

export interface Op {
  kind: OpKind;
  nodeId?: number;
  parentId?: number;
  childId?: number;
  tag?: string;
  name?: string;
  value?: string;
  text?: string;
  style?: Record<string, string>;
  event?: string;
  handlerId?: number;
}

export interface Batch {
  version: number;
  entry?: number;
  ops: Op[];
}

/**
 * IdGen allocates adapter node ids from 1, matching the schema's `minimum: 1`
 * for nodeId and handlerId (0 is the "absent" value in both).
 */
export class IdGen {
  private next = 1;
  take(): number {
    return this.next++;
  }
  peek(): number {
    return this.next;
  }
}

/**
 * Ops collects a module's ops and their findings together.
 *
 * Emitters never throw: every refusal becomes a finding so report mode can see
 * the whole picture, and strict mode turns the first finding into a
 * CompileError at the end of the walk. Keeping both on one object is what makes
 * the two modes share a single implementation.
 */
export class Ops {
  readonly ops: Op[] = [];
  private readonly nodes: IdGen;
  private readonly handlers = new IdGen();

  /**
   * ids is injected rather than created so every module in one graph draws
   * from the same counter — node ids are a single flat address space across the
   * whole app, not per file.
   */
  constructor(
    private readonly report: Report,
    ids: IdGen = new IdGen(),
  ) {
    this.nodes = ids;
  }

  createElement(nodeId: number, tag: string): void {
    this.ops.push({ kind: 'createElement', nodeId, tag });
  }

  createText(nodeId: number, text: string): void {
    this.ops.push({ kind: 'createText', nodeId, text });
  }

  setAttribute(nodeId: number, name: string, value: string): void {
    this.ops.push({ kind: 'setAttribute', nodeId, name, value });
  }

  setStyle(nodeId: number, style: Record<string, string>): void {
    this.ops.push({ kind: 'setStyle', nodeId, style });
  }

  setText(nodeId: number, value: string): void {
    this.ops.push({ kind: 'setText', nodeId, value });
  }

  appendChild(parentId: number, childId: number): void {
    this.ops.push({ kind: 'appendChild', parentId, childId });
  }

  removeChild(parentId: number, childId: number): void {
    this.ops.push({ kind: 'removeChild', parentId, childId });
  }

  addEventListener(nodeId: number, event: string): number {
    const handlerId = this.handlers.take();
    this.ops.push({ kind: 'addEventListener', nodeId, event, handlerId });
    return handlerId;
  }

  removeEventListener(nodeId: number, event: string, handlerId: number): void {
    this.ops.push({ kind: 'removeEventListener', nodeId, event, handlerId });
  }

  nextNode(): number {
    return this.nodes.take();
  }

  add(finding: Finding): void {
    this.report.add(finding);
  }
}

/** styleToRecord turns a parsed declaration list into the op's style map. */
export function styleToRecord(
  declarations: ReadonlyArray<{ property: string; value: string }>,
): Record<string, string> {
  const out: Record<string, string> = {};
  // Sorted keys make the emitted op deterministic, which the golden tests and
  // the dist bundle diff both rely on.
  for (const d of [...declarations].sort((a, b) => a.property.localeCompare(b.property))) {
    out[d.property] = d.value;
  }
  return out;
}
