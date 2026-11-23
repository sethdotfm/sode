// Row selection in the import's remap step: one list over every row shown, in page order. The view
// (importremap.js) maps keys and pointer events onto these calls. cursor and anchor are indexes into keys, -1 for
// none. No imports, so Node tests load it.

export function newSelection() {
  return { keys: [], cursor: -1, anchor: -1, drag: -1, sel: new Set() };
}

// keys: the rows shown now. A selected row that is no longer shown leaves the selection.
export function setKeys(s, keys) {
  const at = s.keys[s.cursor];
  const anchor = s.keys[s.anchor];
  s.keys = [...keys];
  s.sel = new Set([...s.sel].filter((k) => s.keys.includes(k)));
  s.cursor = s.keys.indexOf(at);
  s.anchor = s.keys.indexOf(anchor);
  if (s.cursor < 0 && s.sel.size) s.cursor = s.keys.indexOf([...s.sel][0]);
}

const range = (s, a, b) => new Set(s.keys.slice(Math.min(a, b), Math.max(a, b) + 1));

// Arrow keys. extend (Shift) selects from the anchor to the new cursor.
export function move(s, delta, extend = false) {
  if (!s.keys.length) return;
  const from = s.cursor;
  s.cursor = from < 0 ? 0 : Math.max(0, Math.min(s.keys.length - 1, from + delta));
  if (!extend) return;
  if (s.anchor < 0) {
    s.anchor = from < 0 ? s.cursor : from;
    s.sel.add(s.keys[s.anchor]);
  }
  s.sel = range(s, s.anchor, s.cursor);
}

export function toggle(s) {
  const k = s.keys[s.cursor];
  if (k === undefined) return;
  if (s.sel.has(k)) s.sel.delete(k);
  else s.sel.add(k);
  s.anchor = s.cursor;
}

// add: Cmd-click on a Mac, Ctrl-click elsewhere.
export function click(s, key, { shift = false, add = false } = {}) {
  const i = s.keys.indexOf(key);
  if (i < 0) return;
  if (shift && s.anchor >= 0) s.sel = range(s, s.anchor, i);
  else if (add) {
    if (s.sel.has(key)) s.sel.delete(key);
    else s.sel.add(key);
    s.anchor = i;
  } else {
    s.sel = new Set([key]);
    s.anchor = i;
  }
  s.cursor = i;
}

export function dragFrom(s, key) {
  s.drag = s.keys.indexOf(key);
  if (s.drag >= 0) [s.anchor, s.cursor] = [s.drag, s.drag];
}

export function dragTo(s, key) {
  const i = s.keys.indexOf(key);
  if (s.drag < 0 || i < 0) return;
  s.sel = range(s, s.drag, i);
  s.cursor = i;
}

export function dragEnd(s) {
  s.drag = -1;
}

export function clear(s) {
  s.sel = new Set();
  s.anchor = -1;
}

export const picked = (s) => s.keys.filter((k) => s.sel.has(k));
