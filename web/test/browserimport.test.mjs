import { test } from 'node:test';
import assert from 'node:assert/strict';
import { browserBookmarks, browserPins, browserPrefs, mergeByName } from '../browserimport.js';
import { choicesFor } from '../importrules.js';

test('only known preferences with known values come from a file', () => {
  assert.deepEqual(browserPrefs({ theme: 'dark', clock: 'sundial', layout: 3, shoeSize: '9', presets: [] }), { theme: 'dark' });
  assert.deepEqual(browserPrefs(null), {});
});

test('Add import updates by name and keeps the rest; Replace keeps only the file', () => {
  const local = [{ name: 'Cams' }, { name: 'Mons', x: 1 }];
  const file = [{ name: 'mons', x: 2 }, { name: 'New' }];
  assert.deepEqual(mergeByName(local, file, false), [{ name: 'Cams' }, { name: 'mons', x: 2 }, { name: 'New' }]);
  assert.deepEqual(mergeByName(local, file, true), file);
});

test('This browser bookmarks are one list: Add takes a list, Replace takes even an empty one, unsafe ones drop', () => {
  const file = {
    bookmarks: [
      { url: 'https://example.com/rota', label: 'Rota', newTab: true },
      { url: 'https://example.com/#key=abc', label: 'Panel' },
      { key: 'preset:local:p-1', label: 'Show', inPalette: false },
    ],
  };
  const want = [
    { url: 'https://example.com/rota', label: 'Rota', newTab: true },
    { key: 'preset:local:p-1', label: 'Show', newTab: false },
  ];
  assert.deepEqual(browserBookmarks(file, false), want);
  assert.deepEqual(browserBookmarks(file, true), want);
  assert.equal(browserBookmarks({ bookmarks: [] }, false), null);
  assert.deepEqual(browserBookmarks({ bookmarks: [] }, true), []);
  assert.equal(browserBookmarks({ theme: 'dark' }, true), null);
});

test('This browser Palette pins are one list, as its bookmarks: bad keys and repeats drop', () => {
  const file = { palettePins: ['page:log', 'https://example.com/', 'page:log', 'preset:local:p-1', 7] };
  assert.deepEqual(browserPins(file, false), ['page:log', 'preset:local:p-1']);
  assert.equal(browserPins({ palettePins: [] }, false), null);
  assert.deepEqual(browserPins({ palettePins: [] }, true), []);
  assert.equal(browserPins({ theme: 'dark' }, true), null);
});

test('Everyone bookmarks is one list, so it has no Rename', () => {
  assert.deepEqual(
    choicesFor('bookmarks', {}).map(([v]) => v),
    ['new', 'current', 'skip'],
  );
});

test('Default artwork offers no Rename, since it is one per item', () => {
  assert.deepEqual(
    choicesFor('artwork', { kind: 'default' }).map(([v]) => v),
    ['new', 'current', 'skip'],
  );
  assert.equal(choicesFor('artwork', { kind: 'icon' }).length, 4);
});
