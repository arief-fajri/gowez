/**
 * The emitted runtime.
 *
 * Strategy B emits JavaScript that drives the UI through one door: `ui.apply`.
 * This module produces that JavaScript, and it owns the reactivity model
 * decided for M5: **a full re-render per state change, diffed against the
 * previous tree by node id**.
 *
 * Why re-render rather than a virtual DOM:
 *
 * - The Go runtime already owns layout and painting. A JS-side DOM would
 *   duplicate that work rather than share it.
 * - The diff is over a tiny node shape, so a full re-render is cheap — and,
 *   more importantly, **deterministic**: the same state always yields the same
 *   op stream, which the golden tests and the op-count metrics rely on
 *   (Module 6).
 * - A batch is atomic, so a rejected render cannot leave a half-updated tree
 *   (I1).
 *
 * Everything here uses only `gowez.invoke` / `gowez.on`, the host surface the
 * M4 sandbox already provides (docs/SCRIPT.md). No timers, no DOM, no modules.
 */
import { APPLY_METHOD, INSTRUCTION_VERSION } from './subset.js';

/**
 * RUNTIME is the prelude every bundle starts with.
 *
 * Node shape the generated render functions produce:
 *
 *     el('div', {class: 'x'}, [ ... ], {click: fn})   → element
 *     txt('hello')                                     → text
 *     frag([ ... ])                                    → no node of its own
 *
 * Ids are allocated by the runtime, never by generated code, so two renders can
 * both start from 1 without colliding in the runtime's permanent address space.
 */
export const RUNTIME = `
// GoWEZ adapter runtime — generated, do not edit.
// Host surface: gowez.invoke / gowez.on only (docs/SCRIPT.md).
//
// Model: a state write re-runs the render functions; the diff below turns the
// difference from the previously mounted tree into one instruction batch.
var __version = ${INSTRUCTION_VERSION};
var __nextNodeId = 1;
var __nextHandlerId = 1;
var __handlers = {};
var __effects = [];
var __renders = [];
var __roots = [];
var __dirty = false;

// --- node constructors (called by generated code) ---------------------

function el(tag, attrs, children, handlers) {
  return {
    tag: tag,
    attrs: attrs || {},
    children: children || [],
    handlers: handlers || {},
    __id: 0,
    __key: null
  };
}

function txt(value) {
  return { text: String(value), __id: 0, __key: null };
}

function frag(children) {
  return { tag: null, attrs: {}, children: children || [], __id: 0, __key: null };
}

// each maps a collection and tags each result with its key, so a later render
// can reorder a list without rebuilding it.
function each(items, build, keyOf) {
  var out = [];
  if (!items) { return frag(out); }
  for (var i = 0; i < items.length; i++) {
    var node = build(items[i], i);
    if (node === null || node === undefined) { continue; }
    var key = keyOf(items[i], i);
    if (node.tag === null && node.children) {
      // A fragment per item still needs its children keyed, so the whole item
      // reconciles as one unit.
      for (var j = 0; j < node.children.length; j++) {
        node.children[j].__key = key;
      }
    } else {
      node.__key = key;
    }
    out.push(node);
  }
  return frag(out);
}

// --- host access -------------------------------------------------------

// apply submits one instruction batch. It is the only mutation door.
function apply(ops) {
  if (!ops || ops.length === 0) { return true; }
  return gowez.invoke(${JSON.stringify(APPLY_METHOD)}, {
    version: __version,
    ops: ops
  });
}

// on registers a handler under the name the Go runtime fires. The convention is
// handlerId → "h<id>"; without this call a JS handler would be unreachable from
// a Go listener, and every click would silently do nothing.
function on(handlerId, fn) {
  __handlers[handlerId] = fn;
  gowez.on("h" + handlerId, fn);
}

function handlerName(id) { return "h" + id; }

// --- runes -------------------------------------------------------------

// state is a mutable cell. Reactivity is re-render, not dependency tracking:
// any write marks the tree dirty and the next flush re-runs every render
// function. That trades precision for a model with no hidden graph, which is
// the right trade for a subset this small.
function state(initial) {
  return { value: initial };
}

function get(cell) {
  return cell === null || cell === undefined ? undefined : cell.value;
}

function set(cell, next) {
  if (cell === null || cell === undefined) { return next; }
  if (cell.value === next) { return next; }
  cell.value = next;
  __dirty = true;
  __flush();
  return next;
}

function effect(fn) {
  __effects.push(fn);
}

function onMount(fn) {
  __effects.push(fn);
}

function onDestroy(fn) {
  __effects.push(function () { fn(); });
}

function tick() {
  return { then: function (fn) { fn(); } };
}

// --- mount + flush -----------------------------------------------------

// mount instantiates a component and mounts its first render.
//
// The component factory runs once here — that is where its state cells are
// created — and returns the render function the flush will re-run. Passing a
// render function directly would rebuild the cells on every flush, which is
// exactly the bug this split exists to prevent.
function mount(component, props) {
  var render = component(props || {});
  var root = { render: render, previous: null };
  __roots.push(root);
  var first = render();
  var ops = [];
  __diff(null, first, 0, ops);
  if (ops.length > 0) { apply(ops); }
  root.previous = first;
  return root;
}

// flush re-renders every root and submits the deltas. It is called from set(),
// so a state change paints in the same JS turn that caused it — there is no
// scheduler to wait for because the runtime provides no timers.
function __flush() {
  if (!__dirty) { return; }
  __dirty = false;
  for (var i = 0; i < __roots.length; i++) {
    var root = __roots[i];
    var next = root.render();
    var ops = [];
    __diff(root.previous, next, 0, ops);
    if (ops.length > 0) { apply(ops); }
    root.previous = next;
  }
  for (var j = 0; j < __effects.length; j++) {
    __effects[j]();
  }
}

// --- diff --------------------------------------------------------------

// __diff reconciles prev (the mounted tree, null on first mount) against next
// and appends instructions to ops. prev === null creates everything.
function __diff(prev, next, parentId, ops) {
  if (prev === null) {
    __create(next, ops);
    return;
  }

  var prevText = prev.text !== undefined;
  var nextText = next.text !== undefined;

  if (prevText !== nextText) {
    // One side is text and the other is an element: replace wholesale.
    __replace(prev, next, parentId, ops);
    return;
  }
  if (nextText) {
    // The fresh node inherits the mounted node's identity. Without this the new
    // tree carries __id 0, and the *next* flush has no node to write to — a
    // two-render bug that only shows on the second interaction.
    next.__id = prev.__id;
    next.__key = prev.__key;
    if (prev.text !== next.text && prev.__id > 0) {
      ops.push({ kind: "setText", nodeId: prev.__id, value: next.text });
    }
    return;
  }

  if (prev.tag !== next.tag) {
    __replace(prev, next, parentId, ops);
    return;
  }

  next.__id = prev.__id;
  next.__key = prev.__key;

  if (next.tag === null) {
    __diffChildren(prev.children, next.children, parentId, ops);
    return;
  }

  __diffAttrs(prev, next, ops);
  __diffChildren(prev.children, next.children, next.__id, ops);
}

// __replace swaps one node for another, including its whole subtree.
function __replace(prev, next, parentId, ops) {
  __removeSubtree(prev, parentId, ops);
  var created = __create(next, ops);
  if (parentId > 0) {
    ops.push({ kind: "appendChild", parentId: parentId, childId: created });
  }
}

/**
 * __diffChildren reconciles two child lists under parentId.
 *
 * parentId is the enclosing element's id (0 for the tree root, which the mount
 * already created and must never re-create). Passing it down is what lets a
 * removal deep in the tree emit the right removeChild without a search.
 */
function __diffChildren(prev, next, parentId, ops) {
  var i;
  // Fragments are reconciled as units at their own position. Hoisting their
  // children into the parent would shift every following sibling between
  // renders — an {#if} that swaps a branch for an empty fragment would make
  // the next sibling compare against the wrong node.
  var keyed = next.length > 0;
  for (i = 0; i < next.length; i++) {
    if (next[i].__key === null || next[i].__key === undefined) { keyed = false; break; }
  }
  // parentId of 0 means the tree root, which mount already created.
  var rootLevel = parentId <= 0;

  if (!keyed) {
    // Unkeyed: reconcile by position, which is what a static template means.
    var common = prev.length < next.length ? prev.length : next.length;
    for (i = 0; i < common; i++) {
      __diff(prev[i], next[i], parentId, ops);
    }
    for (i = common; i < prev.length; i++) {
      __removeSubtree(prev[i], parentId, ops);
    }
    for (i = common; i < next.length; i++) {
      __attach(parentId, __create(next[i], ops), ops);
    }
    return;
  }

  // Keyed: match by key, so a reorder emits nothing and an insert adds only the
  // new node. This is what makes a list update cheap and stable.
  for (i = 0; i < prev.length; i++) {
    var p = prev[i];
    var stillThere = false;
    for (var j = 0; j < next.length; j++) {
      if (next[j].__key === p.__key) { stillThere = true; break; }
    }
    if (!stillThere) { __removeSubtree(p, parentId, ops); }
  }
  for (i = 0; i < next.length; i++) {
    var node = next[i];
    var before = null;
    for (var k = 0; k < prev.length; k++) {
      if (prev[k].__key === node.__key) { before = prev[k]; break; }
    }
    if (before === null) {
      __attach(parentId, __create(node, ops), ops);
    } else {
      __diff(before, node, parentId, ops);
    }
  }
}

// __attach appends a freshly created node under parentId. parentId 0 is the
// tree root: mount already created it, so a child of it needs no instruction.
function __attach(parentId, childId, ops) {
  if (parentId <= 0) { return; }
  ops.push({ kind: "appendChild", parentId: parentId, childId: childId });
}

// __create emits the instructions that build a fresh subtree and records the
// runtime ids on the nodes themselves.
function __create(node, ops) {
  if (node === null || node === undefined) { return 0; }

  if (node.text !== undefined) {
    node.__id = __nextNodeId++;
    ops.push({ kind: "createText", nodeId: node.__id, text: node.text });
    return node.__id;
  }

  if (node.tag === null) {
    // A fragment has no node: its children attach to the enclosing parent.
    var ids = [];
    for (var i = 0; i < node.children.length; i++) {
      ids.push(__create(node.children[i], ops));
    }
    node.__ids = ids;
    return 0;
  }

  node.__id = __nextNodeId++;
  ops.push({ kind: "createElement", nodeId: node.__id, tag: node.tag });

  for (var name in node.attrs) {
    if (!Object.prototype.hasOwnProperty.call(node.attrs, name)) { continue; }
    ops.push({ kind: "setAttribute", nodeId: node.__id, name: name, value: String(node.attrs[name]) });
  }

  for (var event in node.handlers) {
    if (!Object.prototype.hasOwnProperty.call(node.handlers, event)) { continue; }
    var handlerId = __nextHandlerId++;
    on(handlerId, node.handlers[event]);
    ops.push({ kind: "addEventListener", nodeId: node.__id, event: event, handlerId: handlerId });
  }

  // A fragment child has no node of its own, so its grandchildren are attached
  // here. Appending the fragment's id (always 0) would be an invalid op.
  for (var c = 0; c < node.children.length; c++) {
    __attachChildren(node.__id, node.children[c], ops);
  }

  return node.__id;
}

// __attachChildren appends a node, or the members of a fragment, under parentId.
function __attachChildren(parentId, node, ops) {
  if (node === null || node === undefined) { return; }
  if (node.tag === null) {
    for (var i = 0; i < node.children.length; i++) {
      __attachChildren(parentId, node.children[i], ops);
    }
    return;
  }
  var id = __create(node, ops);
  __attach(parentId, id, ops);
}

// __removeSubtree emits the removals for a node and everything under it,
// deepest first so a parent is never detached while it still has children.
function __removeSubtree(node, parentId, ops) {
  if (node === null || node === undefined) { return; }
  if (node.children) {
    for (var i = 0; i < node.children.length; i++) {
      __removeSubtree(node.children[i], node.tag === null ? parentId : node.__id, ops);
    }
  }
  if (node.__id > 0 && parentId > 0) {
    ops.push({ kind: "removeChild", parentId: parentId, childId: node.__id });
  }
}

function __diffAttrs(prev, next, ops) {
  for (var name in next.attrs) {
    if (!Object.prototype.hasOwnProperty.call(next.attrs, name)) { continue; }
    var before = prev.attrs[name];
    var after = next.attrs[name];
    if (before === after) { continue; }
    ops.push({ kind: "setAttribute", nodeId: next.__id, name: name, value: String(after) });
  }
}
`;

/** EMITTED_GLOBALS lists the runtime names a bundle relies on. */
export const EMITTED_GLOBALS = [
  'el',
  'txt',
  'frag',
  'each',
  'state',
  'get',
  'set',
  'effect',
  'onMount',
  'onDestroy',
  'tick',
  'on',
  'handlerName',
  'mount',
] as const;
