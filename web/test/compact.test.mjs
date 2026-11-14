// Compact view: the pref, its Alt+Shift+C toggle and the html[data-compact] that hides History.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');

const attrs = new Set();
const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
const root = new Proxy(
  {},
  {
    get: (t, k) => {
      if (k === 'toggleAttribute') return (name, on) => (on ? attrs.add(name) : attrs.delete(name));
      if (k === 'hasAttribute') return (name) => attrs.has(name);
      return node();
    },
    set: () => true,
  },
);
globalThis.document = new Proxy(node(), { get: (t, k) => (k === 'documentElement' ? root : node()) });
const store = new Map();
const writes = [];
globalThis.localStorage = {
  getItem: (k) => (store.has(k) ? store.get(k) : null),
  setItem: (k, v) => {
    writes.push(k);
    store.set(k, String(v));
  },
  removeItem: (k) => {
    writes.push(k);
    store.delete(k);
  },
};

test('compact resolves this browser, then Me, then Everyone, then off', async () => {
  const { resolve } = await import('../prefs.js');
  assert.equal(resolve('compact', null, null, null), 'off');
  assert.equal(resolve('compact', null, null, { compact: true }), 'on');
  assert.equal(resolve('compact', null, { compact: 'off' }, { compact: true }), 'off');
  assert.equal(resolve('compact', 'on', { compact: 'off' }, { compact: false }), 'on');
});

test('the toggle writes only this browser, the explicit opposite, and History follows', async () => {
  const { toggleCompact, compactOn, emit } = await import('./all.mjs');
  // Everyone has it on: a toggle must still turn it off, so it can't write '' and follow Everyone.
  emit('helloCfg', { deviceDefaults: { compact: true } });
  assert.equal(compactOn(), true);
  assert.equal(attrs.has('data-compact'), true);
  writes.length = 0;
  toggleCompact();
  assert.equal(compactOn(), false);
  assert.equal(store.get('sode-compact'), 'off');
  assert.equal(attrs.has('data-compact'), false);
  assert.deepEqual(writes, ['sode-compact']);
  toggleCompact();
  assert.equal(store.get('sode-compact'), 'on');
  assert.equal(attrs.has('data-compact'), true);
  assert.deepEqual(writes, ['sode-compact', 'sode-compact']);
});

test('desktop.css hides only the desktop History under data-compact', () => {
  const css = readFileSync(join(web, 'desktop.css'), 'utf8');
  assert.match(css, /:root\[data-layout="desktop"\]\[data-compact\][^{]*#patch-view > \.sode__history \{\s*display: none;/);
});

// Seam: Alt+Shift+C is bound once, and nothing else takes Alt+Shift+C or lists it.
test('Alt+Shift+C collides with no other binding or shortcut-list entry', () => {
  const hits = [];
  for (const f of readdirSync(web).filter((f) => f.endsWith('.js'))) {
    const src = readFileSync(join(web, f), 'utf8');
    for (const m of src.matchAll(/'alt\+shift\+c'|KeyC/g)) hits.push(`${f}: ${m[0]}`);
  }
  assert.deepEqual(hits, ["desktopkeys.js: 'alt+shift+c'"]);
  const html = readFileSync(join(web, 'index.html'), 'utf8');
  const list = html.slice(html.indexOf('id="keys-dialog"'));
  assert.equal([...list.matchAll(/<kbd[^>]*>(?:Alt|⌥)\+?(?:Shift|⇧)\+?c<\/kbd>/gi)].length, 1);
  assert.equal([...list.matchAll(/data-mac="⌥⇧c"/g)].length, 1);
});
