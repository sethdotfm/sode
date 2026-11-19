import { test } from 'node:test';
import assert from 'node:assert/strict';
import { browserPrefs, mergeByName } from '../browserimport.js';
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

test('Default artwork offers no Rename, since it is one per item', () => {
  assert.deepEqual(
    choicesFor('artwork', { kind: 'default' }).map(([v]) => v),
    ['new', 'current', 'skip'],
  );
  assert.equal(choicesFor('artwork', { kind: 'icon' }).length, 4);
});
