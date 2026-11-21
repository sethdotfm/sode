import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  CATEGORIES,
  iconsBroughtBy,
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
  OWN_CATEGORIES,
  ADMIN_ONLY,
  newOwnPick,
  ownState,
  ownSize,
  ownBody,
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
    { name: 'bookmarks', size: 60, items: [{ id: 'bookmarks', name: 'Everyone bookmarks', size: 60 }] },
    { name: 'panels', size: 50, items: [{ id: 'p-1', name: 'Desk', kind: 'panel', size: 50, icons: ['ic-1'] }] },
    {
      name: 'artwork',
      size: 70,
      items: [
        { id: 'ic-1', name: 'Star', kind: 'icon', size: 30 },
        { id: 'ic-2', name: 'Moon', kind: 'icon', size: 40 },
      ],
    },
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

test('a picked panel brings the Icon library icons it uses', () => {
  const p = newPick(list);
  for (const [c] of CATEGORIES) setCat(list, p, c, false);
  assert.equal(iconsBroughtBy(list, p).size, 0);
  assert.equal(catSize(list, p, 'artwork'), 0);
  p.items.panels.add('p-1');
  assert.equal(iconsBroughtBy(list, p).get('ic-1').name, 'Desk');
  assert.equal(catSize(list, p, 'artwork'), 30);
  assert.deepEqual(exportBody(list, p, {}).categories, ['panels']);
});

test('own export: everything ticked; the body sends only own categories, presets and filter sets by id', () => {
  const own = {
    categories: [
      { name: 'profile', items: [{ id: 'profile', name: 'Profile', size: 20 }] },
      {
        name: 'presets',
        items: [
          { id: 'p1', name: 'Show', size: 40 },
          { id: 'p2', name: 'Rehearsal', size: 30 },
        ],
      },
      { name: 'filterSets', items: [] },
    ],
  };
  const p = newOwnPick(own);
  assert.equal(ownState(own, p, 'presets'), 'all');
  assert.equal(ownSize(own, p, 'presets'), 70);
  assert.deepEqual(ownBody(p), { categories: ['profile', 'presets'], pick: { presets: ['p1', 'p2'] } });
  p.items.presets.delete('p2');
  p.items.profile.clear();
  assert.equal(ownState(own, p, 'presets'), 'some');
  assert.deepEqual(ownBody(p), { categories: ['presets'], pick: { presets: ['p1'] } });
  assert.deepEqual(
    OWN_CATEGORIES.map(([c]) => c),
    ['profile', 'presets', 'filterSets'],
  );
  assert.deepEqual(
    ADMIN_ONLY.map(([c]) => c),
    CATEGORIES.map(([c]) => c),
  );
});

test('Everyone bookmarks is one item, sent by its id', () => {
  const p = newPick(list);
  for (const [c] of CATEGORIES) setCat(list, p, c, false);
  p.items.bookmarks.add('bookmarks');
  assert.equal(catState(list, p, 'bookmarks'), 'all');
  assert.equal(catSize(list, p, 'bookmarks'), 60);
  assert.deepEqual(exportBody(list, p, {}), { categories: ['bookmarks'], pick: { bookmarks: ['bookmarks'] }, sections: [], includePins: true });
});
