// prepaint.js runs first on every page, so it clears v1 localStorage before anything reads it.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import vm from 'node:vm';

const src = readFileSync(join(dirname(fileURLToPath(import.meta.url)), '..', 'prepaint.js'), 'utf8');

function storage(entries) {
  const m = new Map(Object.entries(entries));
  return {
    m,
    get length() {
      return m.size;
    },
    key: (i) => [...m.keys()][i] ?? null,
    getItem: (k) => (m.has(k) ? m.get(k) : null),
    setItem: (k, v) => m.set(k, String(v)),
    removeItem: (k) => m.delete(k),
  };
}

function load(localStorage, only = 'theme') {
  const document = { documentElement: { dataset: {} }, currentScript: { dataset: { only } } };
  const sessionStorage = { getItem: () => null };
  vm.runInNewContext(src, { document, localStorage, sessionStorage, matchMedia: () => ({ matches: false }) });
  return document.documentElement.dataset;
}

test('a browser with v1 sode-* keys loses them on the first v2 load', () => {
  const ls = storage({
    'sode-device-defaults': '{"theme":"dark"}',
    'sode-theme': 'light',
    'sode-salvos': '[{"name":"Old"}]',
    'sode-session': 'v1-token',
    'other-app': 'kept',
  });
  const ds = load(ls);
  assert.deepEqual([...ls.m.keys()].sort(), ['other-app', 'sode-format']);
  assert.equal(ls.m.get('sode-format'), '2');
  assert.equal(ds.theme, undefined, 'v1 theme painted');
});

test('v2 keys stay on later loads', () => {
  const ls = storage({});
  load(ls);
  ls.setItem('sode-theme', 'dark');
  ls.setItem('sode-presets', '[]');
  const ds = load(ls);
  assert.deepEqual([...ls.m.keys()].sort(), ['sode-format', 'sode-presets', 'sode-theme']);
  assert.equal(ds.theme, 'dark');
});

test('storage that throws does not stop the page', () => {
  const bad = {
    get length() {
      throw new Error('denied');
    },
    key: () => {
      throw new Error('denied');
    },
    getItem: () => {
      throw new Error('denied');
    },
    setItem: () => {
      throw new Error('denied');
    },
    removeItem: () => {
      throw new Error('denied');
    },
  };
  assert.doesNotThrow(() => load(bad, undefined));
});
