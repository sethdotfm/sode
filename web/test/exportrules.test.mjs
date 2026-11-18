import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  CATEGORIES,
  newPick,
  catState,
  setCat,
  catSize,
  exportBody,
  backupBody,
  routerBroughtBy,
  nothingPicked,
  sizeText,
  fileNameOf,
} from '../exportrules.js';

const list = {
  categories: [
    {
      name: 'settings',
      size: 30,
      sections: [
        { id: 'branding', size: 10 },
        { id: 'locks', size: 20 },
      ],
    },
    { name: 'routers', size: 500, items: [{ id: 'r1', name: 'Router A', size: 500 }] },
    { name: 'users', size: 7, items: [{ id: 'u-1', name: 'Ada', size: 7, detail: 'administrator' }] },
    {
      name: 'presets',
      size: 300,
      items: [
        { id: 'pr-1', name: 'Show', size: 100, router: true },
        { id: 'pr-2', name: 'Late', size: 100, router: true },
        { id: 'pr-3', name: 'Note', size: 100 },
      ],
    },
    { name: 'filterSets', size: 0, items: [] },
    { name: 'panels', size: 50, items: [{ id: 'p-1', name: 'Desk', kind: 'panel', size: 50 }] },
    { name: 'artwork', size: 0 },
    { name: 'apiClients', size: 0 },
    { name: 'browser', size: 0 },
  ],
  all: 900,
};

test('the categories are the server export categories in file order', () => {
  const go = readFileSync(new URL('../../internal/app/exportfile.go', import.meta.url), 'utf8');
  const block = go.slice(go.indexOf('var exportCategories'));
  const consts = Object.fromEntries([...go.matchAll(/(cat\w+)\s+= "(\w+)"/g)].map((m) => [m[1], m[2]]));
  const names = [...block.slice(0, block.indexOf('\n}\n')).matchAll(/\{(cat\w+), "/g)].map((m) => consts[m[1]]);
  assert.deepEqual(
    CATEGORIES.map(([c]) => c),
    names,
  );
});

test('a new pick ticks everything with Include PINs on', () => {
  const p = newPick(list);
  for (const [c] of CATEGORIES) if (!['filterSets', 'artwork', 'apiClients'].includes(c)) assert.equal(catState(list, p, c), 'all', c);
  assert.equal(p.pins, true);
});

test('two presets and one panel send only those ids, plus nothing else', () => {
  const p = newPick(list);
  for (const [c] of CATEGORIES) setCat(list, p, c, false);
  assert.ok(nothingPicked(list, p));
  p.items.presets.add('pr-1').add('pr-3');
  p.items.panels.add('p-1');
  assert.equal(catState(list, p, 'presets'), 'some');
  const body = exportBody(list, p, { prefs: {} });
  assert.deepEqual(body.categories, ['presets', 'panels']);
  assert.deepEqual(body.pick, { presets: ['pr-1', 'pr-3'], panels: ['p-1'] });
  assert.equal(body.browser, undefined);
  // The router goes with Show though Routers is not ticked.
  assert.equal(routerBroughtBy(list, p).name, 'Show');
  assert.equal(catSize(list, p, 'routers'), 500);
  assert.equal(catSize(list, p, 'presets'), 200);
});

test('Settings goes by section', () => {
  const p = newPick(list);
  p.sections.delete('locks');
  assert.equal(catState(list, p, 'settings'), 'some');
  assert.equal(catSize(list, p, 'settings'), 10);
  assert.deepEqual(exportBody(list, p, {}).sections, ['branding']);
});

test('Back up everything sends every category with PINs, whatever the ticks', () => {
  const b = backupBody({ theme: 'dark' });
  assert.deepEqual(
    b.categories,
    CATEGORIES.map(([c]) => c),
  );
  assert.equal(b.includePins, true);
  assert.equal(b.pick, undefined);
  assert.deepEqual(b.browser, { theme: 'dark' });
});

test('sizes and file names read short', () => {
  assert.equal(sizeText(512), '512 B');
  assert.equal(sizeText(1530), '1.5 kB');
  assert.equal(sizeText(25_400), '25 kB');
  assert.equal(sizeText(3_210_000), '3.2 MB');
  assert.equal(fileNameOf('attachment; filename="2026-11-18-Sode-Export.zip"', 'x'), '2026-11-18-Sode-Export.zip');
  assert.equal(fileNameOf(null, '2026-11-18'), '2026-11-18-Sode-Export.zip');
});
