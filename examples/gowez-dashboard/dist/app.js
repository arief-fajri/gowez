(() => {
  // gowez-bundle.js
  var __version = 1;
  var __nextNodeId = 1;
  var __nextHandlerId = 1;
  var __handlers = {};
  var __effects = [];
  var __roots = [];
  var __dirty = false;
  function el(tag, attrs, children, handlers) {
    return {
      tag,
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
  function each(items, build, keyOf) {
    var out = [];
    if (!items) {
      return frag(out);
    }
    for (var i = 0; i < items.length; i++) {
      var node = build(items[i], i);
      if (node === null || node === void 0) {
        continue;
      }
      var key = keyOf(items[i], i);
      if (node.tag === null && node.children) {
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
  function apply(ops) {
    if (!ops || ops.length === 0) {
      return true;
    }
    return gowez.invoke("ui.apply", {
      version: __version,
      ops
    });
  }
  function on(handlerId, fn) {
    __handlers[handlerId] = fn;
    gowez.on("h" + handlerId, fn);
  }
  function state(initial) {
    return { value: initial };
  }
  function get(cell) {
    return cell === null || cell === void 0 ? void 0 : cell.value;
  }
  function set(cell, next) {
    if (cell === null || cell === void 0) {
      return next;
    }
    if (cell.value === next) {
      return next;
    }
    cell.value = next;
    __dirty = true;
    __flush();
    return next;
  }
  function mount(component, props) {
    var render = component(props || {});
    var root = { render, previous: null };
    __roots.push(root);
    var first = render();
    var ops = [];
    __diff(null, first, 0, ops);
    if (ops.length > 0) {
      apply(ops);
    }
    root.previous = first;
    return root;
  }
  function __flush() {
    if (!__dirty) {
      return;
    }
    __dirty = false;
    for (var i = 0; i < __roots.length; i++) {
      var root = __roots[i];
      var next = root.render();
      var ops = [];
      __diff(root.previous, next, 0, ops);
      if (ops.length > 0) {
        apply(ops);
      }
      root.previous = next;
    }
    for (var j = 0; j < __effects.length; j++) {
      __effects[j]();
    }
  }
  function __diff(prev, next, parentId, ops) {
    if (prev === null) {
      __create(next, ops);
      return;
    }
    var prevText = prev.text !== void 0;
    var nextText = next.text !== void 0;
    if (prevText !== nextText) {
      __replace(prev, next, parentId, ops);
      return;
    }
    if (nextText) {
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
  function __replace(prev, next, parentId, ops) {
    __removeSubtree(prev, parentId, ops);
    var created = __create(next, ops);
    if (parentId > 0) {
      ops.push({ kind: "appendChild", parentId, childId: created });
    }
  }
  function __diffChildren(prev, next, parentId, ops) {
    var i;
    var keyed = next.length > 0;
    for (i = 0; i < next.length; i++) {
      if (next[i].__key === null || next[i].__key === void 0) {
        keyed = false;
        break;
      }
    }
    var rootLevel = parentId <= 0;
    if (!keyed) {
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
    for (i = 0; i < prev.length; i++) {
      var p = prev[i];
      var stillThere = false;
      for (var j = 0; j < next.length; j++) {
        if (next[j].__key === p.__key) {
          stillThere = true;
          break;
        }
      }
      if (!stillThere) {
        __removeSubtree(p, parentId, ops);
      }
    }
    for (i = 0; i < next.length; i++) {
      var node = next[i];
      var before = null;
      for (var k = 0; k < prev.length; k++) {
        if (prev[k].__key === node.__key) {
          before = prev[k];
          break;
        }
      }
      if (before === null) {
        __attach(parentId, __create(node, ops), ops);
      } else {
        __diff(before, node, parentId, ops);
      }
    }
  }
  function __attach(parentId, childId, ops) {
    if (parentId <= 0) {
      return;
    }
    ops.push({ kind: "appendChild", parentId, childId });
  }
  function __create(node, ops) {
    if (node === null || node === void 0) {
      return 0;
    }
    if (node.text !== void 0) {
      node.__id = __nextNodeId++;
      ops.push({ kind: "createText", nodeId: node.__id, text: node.text });
      return node.__id;
    }
    if (node.tag === null) {
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
      if (!Object.prototype.hasOwnProperty.call(node.attrs, name)) {
        continue;
      }
      ops.push({ kind: "setAttribute", nodeId: node.__id, name, value: String(node.attrs[name]) });
    }
    for (var event in node.handlers) {
      if (!Object.prototype.hasOwnProperty.call(node.handlers, event)) {
        continue;
      }
      var handlerId = __nextHandlerId++;
      on(handlerId, node.handlers[event]);
      ops.push({ kind: "addEventListener", nodeId: node.__id, event, handlerId });
    }
    for (var c = 0; c < node.children.length; c++) {
      __attachChildren(node.__id, node.children[c], ops);
    }
    return node.__id;
  }
  function __attachChildren(parentId, node, ops) {
    if (node === null || node === void 0) {
      return;
    }
    if (node.tag === null) {
      for (var i = 0; i < node.children.length; i++) {
        __attachChildren(parentId, node.children[i], ops);
      }
      return;
    }
    var id = __create(node, ops);
    __attach(parentId, id, ops);
  }
  function __removeSubtree(node, parentId, ops) {
    if (node === null || node === void 0) {
      return;
    }
    if (node.children) {
      for (var i = 0; i < node.children.length; i++) {
        __removeSubtree(node.children[i], node.tag === null ? parentId : node.__id, ops);
      }
    }
    if (node.__id > 0 && parentId > 0) {
      ops.push({ kind: "removeChild", parentId, childId: node.__id });
    }
  }
  function __diffAttrs(prev, next, ops) {
    for (var name in next.attrs) {
      if (!Object.prototype.hasOwnProperty.call(next.attrs, name)) {
        continue;
      }
      var before = prev.attrs[name];
      var after = next.attrs[name];
      if (before === after) {
        continue;
      }
      ops.push({ kind: "setAttribute", nodeId: next.__id, name, value: String(after) });
    }
  }
  var _UserList = function(props) {
    var users = "users" in props ? props["users"] : [], items = "items" in props ? props["items"] : [], onRemove = "onRemove" in props ? props["onRemove"] : function() {
    };
    return function() {
      return el("ul", { "class": ["list", "s-UserList"].filter(Boolean).join(" ") }, [each(items, function(user, i) {
        return el("li", { "class": ["row", "s-UserList"].filter(Boolean).join(" ") }, [el("p", { "class": ["name", "s-UserList"].filter(Boolean).join(" ") }, [txt(user.name)]), el("p", { "class": ["role", "s-UserList"].filter(Boolean).join(" ") }, [txt(user.role)]), el("button", { "tabindex": "0", "class": ["", "s-UserList"].filter(Boolean).join(" ") }, [txt("remove")], { "click": function() {
          onRemove(user.id);
        } })]);
      }, function(user, i) {
        return user.id;
      })]);
    };
  };
  var _App = function(props) {
    var count = state(0);
    var query = state("");
    var active = state("dashboard");
    var sections = ["dashboard", "users", "reports"];
    var users = state([{ "id": 1, "name": "Aria", "role": "admin" }, { "id": 2, "name": "Bagus", "role": "editor" }, { "id": 3, "name": "Citra", "role": "editor" }]);
    var filtered = function() {
      return get(query) === "" ? get(users) : get(users).filter(function(u) {
        return u.name.toLowerCase().indexOf(get(query).toLowerCase()) >= 0;
      });
    };
    function inc() {
      set(count, count.value + 1);
    }
    function dec() {
      set(count, count.value - 1);
    }
    function onRemove(id) {
      set(users, get(users).filter(function(u) {
        return u.id !== id;
      }));
    }
    return function() {
      return el("section", { "class": ["shell", "s-App"].filter(Boolean).join(" ") }, [el("header", { "class": ["bar", "s-App"].filter(Boolean).join(" ") }, [el("p", { "class": ["brand", "s-App"].filter(Boolean).join(" ") }, [txt("GoWEZ")]), el("nav", { "class": ["", "s-App"].filter(Boolean).join(" ") }, [each(sections, function(s, i) {
        return el("button", { "tabindex": "0", "class": ["", get(active) === s ? " active" : "", "s-App"].filter(Boolean).join(" ") }, [txt(s)], { "click": function() {
          set(active, s);
        } });
      }, function(s, i) {
        return s;
      })])]), el("div", { "class": ["body", "s-App"].filter(Boolean).join(" ") }, [el("aside", { "class": ["side", "s-App"].filter(Boolean).join(" ") }, [el("p", { "class": ["label", "s-App"].filter(Boolean).join(" ") }, [txt("counter")]), el("p", { "class": ["count", "s-App"].filter(Boolean).join(" ") }, [txt(get(count))]), el("button", { "tabindex": "0", "class": ["", "s-App"].filter(Boolean).join(" ") }, [txt("-1")], { "click": function(ev) {
        if (typeof dec === "function") {
          dec(ev);
        }
      } }), el("button", { "tabindex": "0", "class": ["", "s-App"].filter(Boolean).join(" ") }, [txt("+1")], { "click": function(ev) {
        if (typeof inc === "function") {
          inc(ev);
        }
      } }), get(count) > 0 ? el("p", { "class": ["hint", "s-App"].filter(Boolean).join(" ") }, [txt("positive")]) : el("p", { "class": ["hint", "s-App"].filter(Boolean).join(" ") }, [txt("zero or below")])]), el("main", { "class": ["main", "s-App"].filter(Boolean).join(" ") }, [el("input", { "tabindex": "0", "placeholder": "filter users", "class": ["", "s-App"].filter(Boolean).join(" ") }, [], { "textinput": function(ev) {
        set(query, ev.text);
      } }), el("p", { "class": ["label", "s-App"].filter(Boolean).join(" ") }, [txt("search:"), txt(get(query))]), _UserList({ "users": get(users), "items": filtered(), "onRemove": onRemove })(), filtered().length === 0 ? el("p", { "class": ["hint", "s-App"].filter(Boolean).join(" ") }, [txt("no users match")]) : frag([])])])]);
    };
  };
  mount(_App, {});
})();
