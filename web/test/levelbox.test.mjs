import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const fake = (tag = 'div') => {
  const set = new Set();
  const n = {
    tag,
    kids: [],
    className: '',
    attrs: {},
    dataset: {},
    style: { setProperty() {}, removeProperty() {} },
    listeners: {},
    value: '',
    placeholder: '',
    setAttribute(k, v) {
      this.attrs[k] = String(v);
    },
    addEventListener(t, f) {
      (this.listeners[t] ||= []).push(f);
    },
    append(...k) {
      this.kids.push(...k);
    },
    replaceChildren(...k) {
      this.kids = k;
    },
    closest: () => null,
    classList: { add: (c) => set.add(c), remove: (c) => set.delete(c), contains: (c) => set.has(c), toggle: (c, on) => (on ? set.add(c) : set.delete(c)) },
  };
  Object.defineProperty(n, 'children', { get: () => n.kids });
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

const { levelBox } = await import('../levelbox.js');

const levels = [1, 2, 3, 4].map((ui) => ({ level: ui, label: String(ui), swatch: 'blue' }));
const make = (opts = {}) => {
  const input = fake('input');
  const picker = fake();
  const all = fake('button');
  let changes = 0;
  const box = levelBox(input, picker, all, () => ({ levels, numbering: '1', defaultLevels: '1-2' }), { onChange: () => changes++, ...opts });
  box.reset();
  return { box, input, picker, all, changes: () => changes };
};

test('builds one picker button per level from the levels it is given', () => {
  const { picker } = make();
  assert.equal(picker.kids.length, 4);
  assert.deepEqual(
    picker.kids.map((b) => +b.dataset.ui),
    [1, 2, 3, 4],
  );
});

test('a blank box means the default levels, and the placeholder says so', () => {
  const { box, input } = make();
  assert.deepEqual(box.levels(), [1, 2]);
  assert.equal(input.placeholder, '1-2 (default)');
});

test('a picker click writes the typed form and calls onChange', () => {
  const { box, input, picker, changes } = make();
  picker.kids[2].listeners.click[0]();
  assert.equal(input.value, '1-3');
  assert.deepEqual(box.levels(), [1, 2, 3]);
  assert.equal(changes(), 1);
});

test('All selects every level, then clears to none', () => {
  const { box, input, all } = make();
  all.listeners.click[0]();
  assert.equal(input.value, '1-4');
  all.listeners.click[0]();
  assert.equal(input.value, '');
  assert.equal(box.levels(), null);
  assert.equal(input.placeholder, 'none');
});

test('typed text that is not a level list marks the input invalid', () => {
  const { input } = make();
  input.value = '9';
  input.listeners.input[0]();
  assert.ok(input.classList.contains('pb-input--invalid'));
});

test('levelbox.js imports nothing from History, plans or routes', () => {
  const src = readFileSync(fileURLToPath(new URL('../levelbox.js', import.meta.url)), 'utf8');
  const from = [...src.matchAll(/^import\s.*from\s*'\.\/([\w.-]+)'/gm)].map((m) => m[1]);
  for (const bad of ['history.js', 'plans.js', 'routes.js', 'widgets.js', 'levelview.js']) assert.ok(!from.includes(bad), bad);
});
