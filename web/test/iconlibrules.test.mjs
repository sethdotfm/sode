// The Icon library's pure rules (iconlibrules.js): names and the Choose icon picker's sections.
import test from 'node:test';
import assert from 'node:assert/strict';
import { libNameProblem, libNameOf, pickerSections, LIB_NAME_MAX, LIB_TYPES } from '../iconlibrules.js';

test('a library icon name is 1 to 40 characters, counted as characters, not UTF-16 units', () => {
  assert.equal(LIB_NAME_MAX, 40);
  assert.equal(libNameProblem('Logo'), '');
  assert.match(libNameProblem(''), /name/);
  assert.match(libNameProblem('   '), /name/);
  assert.equal(libNameProblem('😀'.repeat(40)), '');
  assert.match(libNameProblem('x'.repeat(41)), /40/);
});

test('a file name gives the default icon name: no extension, trimmed and cut to 40', () => {
  assert.equal(libNameOf('Station logo.png'), 'Station logo');
  assert.equal(libNameOf('mark.SVG'), 'mark');
  assert.equal(libNameOf('noext'), 'noext');
  assert.equal([...libNameOf('é'.repeat(50) + '.png')].length, 40);
});

test('only PNG and SVG go into the library', () => {
  assert.deepEqual(LIB_TYPES, ['image/png', 'image/svg+xml']);
});

test('the picker shows Uploaded above the vendored icons, library icons by name, both filtered by the search', () => {
  const lib = [
    { id: '0000000b', name: 'beta' },
    { id: '0000000a', name: 'Alpha' },
  ];
  const s = pickerSections(lib, ['camera', 'mic'], '');
  assert.deepEqual(
    s.map((x) => x.id),
    ['uploaded', 'icons'],
  );
  assert.deepEqual(s[0].tiles, [
    { value: 'lib:0000000a', name: 'Alpha', id: '0000000a' },
    { value: 'lib:0000000b', name: 'beta', id: '0000000b' },
  ]);
  assert.deepEqual(s[1].tiles, [
    { value: 'icon:camera', name: 'camera' },
    { value: 'icon:mic', name: 'mic' },
  ]);
  const found = pickerSections(lib, ['camera', 'mic'], 'al');
  assert.deepEqual(
    found[0].tiles.map((t) => t.name),
    ['Alpha'],
  );
  assert.deepEqual(found[1].tiles, []);
  // The Uploaded section stays, empty, so its Upload button is always there.
  assert.deepEqual(pickerSections([], ['mic'], '')[0].tiles, []);
  assert.equal(pickerSections(null, [], '')[0].id, 'uploaded');
});
