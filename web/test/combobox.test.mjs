import test from 'node:test';
import assert from 'node:assert/strict';

const fake = (tag = 'div') => {
  const n = {
    tag,
    kids: [],
    className: '',
    attrs: {},
    dataset: {},
    listeners: {},
    hidden: false,
    id: '',
    scrollTop: 0,
    scrollHeight: 0,
    clientHeight: 0,
    setAttribute(k, v) {
      this.attrs[k] = String(v);
    },
    getAttribute(k) {
      return this.attrs[k] ?? null;
    },
    removeAttribute(k) {
      delete this.attrs[k];
    },
    addEventListener(t, f) {
      (this.listeners[t] ||= []).push(f);
    },
    append(...k) {
      this.kids.push(...k);
    },
    prepend(...k) {
      this.kids.unshift(...k);
    },
    replaceChildren(...k) {
      this.kids = k;
    },
    contains(x) {
      return this.kids.includes(x);
    },
    querySelector() {
      return null;
    },
    scrollIntoView() {},
    classList: null,
  };
  const set = new Set();
  n.classList = {
    add: (c) => set.add(c),
    remove: (c) => set.delete(c),
    contains: (c) => set.has(c),
    toggle: (c, on) => (on ? set.add(c) : set.delete(c)),
  };
  Object.defineProperty(n, 'textContent', {
    set(v) {
      this.kids = [String(v)];
    },
  });
  return n;
};
globalThis.document = {
  createElement: fake,
  createElementNS: fake,
  addEventListener() {},
  querySelector: () => null,
  querySelectorAll: () => [],
  body: fake(),
  documentElement: { dataset: {} },
};
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

const { combobox } = await import('../widgets.js');
const { WINDOW } = await import('../listwindow.js');

const items = (n) => Array.from({ length: n }, (_, i) => ({ n: i, num: String(i + 1), name: `Src ${i + 1}` }));
const make = (opts = {}) => {
  const input = fake('input');
  input.id = 'src';
  input.value = '';
  const menu = fake('ul');
  const picked = [];
  const cb = combobox(input, menu, { source: () => items(5), onPick: (it) => picked.push(it.n), ...opts });
  return { cb, input, menu, picked };
};
const ids = (menu) => menu.kids.map((li) => +li.dataset.i);

test('browsing a long list draws only a window, around the chosen row', () => {
  const { cb, menu } = make();
  cb.browse(items(2690), 1500);
  assert.equal(menu.kids.length, WINDOW);
  assert.ok(ids(menu).includes(1500));
  assert.equal(cb.current().n, 1500);
});

test('a typed list is drawn whole, however long', () => {
  const { cb, menu } = make({ source: () => items(300) });
  cb.refresh();
  assert.equal(menu.kids.length, 300);
});

test('moving past the window draws that row and keeps the active descendant', () => {
  const { cb, input, menu } = make();
  cb.browse(items(2690), 0);
  const nav = (key) => cb.nav({ key, preventDefault() {} });
  for (let i = 0; i < 401; i++) nav('ArrowDown'); // the first only shows the cursor on row 0
  assert.equal(cb.current().n, 400);
  assert.ok(ids(menu).includes(400));
  assert.equal(menu.kids.length < 2690, true);
  assert.equal(input.getAttribute('aria-activedescendant'), 'src-opt-400');
  assert.ok(menu.kids.find((li) => li.dataset.i === '400').classList.contains('active'));
  nav('ArrowUp');
  nav('PageUp');
  assert.equal(cb.current().n, 399 - 8);
});

test('a wrap or a jump draws a fresh window there, not every row in between', () => {
  const { cb, menu } = make();
  cb.browse(items(2690), 0);
  cb.nav({ key: 'ArrowUp', preventDefault() {} });
  assert.equal(cb.current().n, 2689);
  assert.equal(menu.kids.length, WINDOW);
  assert.ok(ids(menu).includes(2689));
  cb.nav({ key: 'ArrowDown', preventDefault() {} });
  assert.equal(cb.current().n, 0);
  assert.equal(menu.kids.length, WINDOW);
});

test('moved: false after browse, true after a move, false after refresh or close', () => {
  const { cb } = make();
  cb.browse(items(50), 0);
  assert.equal(cb.moved, false);
  assert.equal(cb.accept(), false);
  cb.nav({ key: 'ArrowDown', preventDefault() {} });
  assert.equal(cb.moved, true);
  assert.equal(cb.accept(), true);
  cb.refresh();
  assert.equal(cb.moved, false);
  cb.browse(items(50), 0);
  cb.close();
  assert.equal(cb.moved, false);
});

test('an empty list is never moved', () => {
  const { cb } = make();
  cb.browse([], 0);
  assert.equal(cb.moved, false);
  assert.equal(cb.open, false);
});

test('a fresh browse list keeps a moved highlight moved when asked', () => {
  const { cb } = make();
  cb.browse(items(50), 0);
  cb.nav({ key: 'ArrowDown', preventDefault() {} });
  cb.browse(items(60), cb.current().n, { keepMoved: true });
  assert.equal(cb.moved, true);
  cb.browse(items(60), 0);
  assert.equal(cb.moved, false);
  // Never moved: a fresh list is still only browsed, Enter must not take its first row.
  cb.browse(items(70), 0, { keepMoved: true });
  assert.equal(cb.moved, false);
  assert.equal(cb.accept(), false);
});

test('a click on a row picks it, through one listener on the menu', () => {
  const { cb, menu, picked } = make();
  cb.browse(items(300), 0);
  const li = menu.kids[3];
  const row = { target: { closest: () => li }, preventDefault() {} };
  menu.listeners.mousedown[0](row);
  assert.deepEqual(picked, [3]);
});

test('a browse list shows no cursor until ↓ (or ↑) is pressed', () => {
  const { cb, input, menu } = make();
  cb.browse(items(5), 0);
  assert.equal(
    menu.kids.some((li) => li.className.includes('active')),
    false,
  );
  assert.equal(input.getAttribute('aria-activedescendant'), null);
  cb.nav({ key: 'ArrowDown', preventDefault() {} });
  assert.equal(cb.current().n, 0);
  assert.ok(menu.kids[0].classList.contains('active') || menu.kids[0].className.includes('active'));
  assert.equal(input.getAttribute('aria-activedescendant'), 'src-opt-0');
  cb.nav({ key: 'ArrowDown', preventDefault() {} });
  assert.equal(cb.current().n, 1);
});

test('a typed list keeps its cursor on the first match', () => {
  const { cb, menu } = make();
  cb.refresh();
  assert.ok(menu.kids[0].className.includes('active'));
});

test('onRender says whether the list is showing after every redraw', () => {
  const seen = [];
  const { cb } = make({ onRender: (open) => seen.push(open) });
  cb.browse(items(5), 0);
  cb.close();
  cb.refresh();
  assert.deepEqual(seen, [true, false, true]);
});

test('browse scrolls the chosen row into view even though no row is highlighted yet', () => {
  const made = document.createElement;
  const scrolled = [];
  document.createElement = (t) => {
    const n = made(t);
    n.scrollIntoView = () => scrolled.push(n);
    return n;
  };
  try {
    const { cb, menu } = make();
    cb.browse(items(5), 3);
    assert.ok(scrolled.includes(menu.kids[3]));
  } finally {
    document.createElement = made;
  }
});
