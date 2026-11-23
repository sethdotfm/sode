import test from 'node:test';
import assert from 'node:assert/strict';
import { nameArt, nameArtKey } from '../gridpanelart.js';
import { defaultsBy } from '../panellook.js';
import { SPAN } from '../routerids.js';

const edge = { members: 'all', items: { 1: { art: 'icon:camera', artB: '2' }, 2: { artB: 'A' } } };
const defaults = defaultsBy([
  { kind: 'source', n: 2, art: 'icon:mic', artB: 'B', hideText: true },
  { kind: 'source', n: 3, art: 'icon:mic', artB: '9', hideText: true },
  { kind: 'destination', n: 3, art: 'icon:monitor' },
]);

test('an item shows its own artwork', () => {
  assert.deepEqual(nameArt(edge, 'source', 1, defaults), { a: { icon: 'camera' }, b: '2' });
});

test('an item with no artwork of its own shows its Default artwork, Hide text included', () => {
  assert.deepEqual(nameArt(edge, 'source', 3, defaults), { a: { icon: 'mic' }, b: '9', hide: true });
  assert.deepEqual(nameArt(edge, 'destination', 3, defaults), { a: { icon: 'monitor' }, b: '' });
});

test('own artwork wins whole: its Icon B alone hides the default Icon A', () => {
  assert.deepEqual(nameArt(edge, 'source', 2, defaults), { a: null, b: 'A', hide: true });
});

test('Hide text is the item own true or false, else the default, and needs artwork', () => {
  const own = { members: 'all', items: { 1: { art: 'icon:camera', hideText: true }, 3: { hideText: false }, 4: { hideText: true } } };
  assert.equal(nameArt(own, 'source', 1, defaults).hide, true);
  assert.equal('hide' in nameArt(own, 'source', 3, defaults), false);
  assert.equal(nameArt(own, 'source', 4, defaults), null);
  assert.equal('hide' in nameArt(edge, 'source', 1, defaults), false);
  assert.notEqual(nameArtKey({ a: null, b: '1', hide: true }), nameArtKey({ a: null, b: '1' }));
});

test('an item with neither gives null, whatever the edge or defaults are', () => {
  assert.equal(nameArt(edge, 'source', 7, defaults), null);
  assert.equal(nameArt(undefined, 'source', 7, undefined), null);
  assert.equal(nameArt({ members: 'all', items: { 7: { art: 'https://x/y.png' } } }, 'source', 7, defaults), null);
});

test('a default is for its kind: a destination number is not a source number', () => {
  assert.equal(nameArt({ members: 'all' }, 'destination', 2, defaults), null);
});

test('the key changes with the artwork and only then', () => {
  const k = nameArtKey(nameArt(edge, 'source', 1, defaults));
  assert.equal(k, nameArtKey(nameArt(edge, 'source', 1, defaults)));
  assert.notEqual(k, nameArtKey(nameArt(edge, 'source', 3, defaults)));
  assert.equal(nameArtKey(null), '');
});

test('a name read with a router shows the Default artwork of that router', () => {
  const two = defaultsBy(
    [
      { kind: 'source', router: 'ra', n: 6, artB: 'A' },
      { kind: 'source', router: 'rb', n: 6, artB: 'B' },
    ],
    'ra',
  );
  assert.equal(nameArt({ members: 'all' }, 'source', 6, two).b, 'A');
  // Router B's source 6 is item number SPAN + 6 (routerids.js).
  assert.equal(nameArt({ members: [SPAN + 6] }, 'source', SPAN + 6, two, () => 'rb').b, 'B');
});
