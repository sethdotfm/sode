import test from 'node:test';
import assert from 'node:assert/strict';
import { ROWS_MARK, copyRows, rowsText, readRows, pasteRows, pasteRowsNote, pickAfterRemove } from '../edgeclip.js';
import { CLIP_MARK } from '../gridclip.js';
import { edgeSnap, sameEdges } from '../gridundo.js';
import { namesReply } from './gridpanelfixture.mjs';

const router = namesReply({
  sources: [
    [0, 'CAM 1'],
    [1, 'CAM 2'],
    [2, 'VT 1'],
    [3, 'VT 2'],
    [4, 'GFX'],
  ],
}).sources;

const ART = 'icon:camera';
const UPLOAD = 'a-1b2c3d4e';

test('a Grid list snapshot holds both edges, and two copies of one edge compare equal', () => {
  const sources = { members: [2, 0], items: { 0: { pinned: true, art: ART } } };
  const destinations = { members: 'all', hide: [1] };
  const a = edgeSnap({ sources, destinations, settings: { x: 1 } });
  assert.deepEqual(a, { edges: { sources, destinations } });
  const b = edgeSnap({ sources: { items: { 0: { art: ART, pinned: true } }, members: [2, 0] }, destinations: { hide: [1], members: 'all' } });
  assert.equal(sameEdges(a, b), true);
  assert.equal(sameEdges(a, edgeSnap({ sources: { ...sources, members: [0, 2] }, destinations })), false);
  assert.equal(sameEdges(a, edgeSnap({ sources: { ...sources, items: {} }, destinations })), false);
  assert.equal(sameEdges(a, edgeSnap({ sources, destinations: { members: 'all' } })), false);
  // A step keeps only the lists its edit named.
  const one = edgeSnap({ sources, destinations }, ['sources']);
  assert.deepEqual(one, { edges: { sources } });
  assert.equal(sameEdges(one, edgeSnap({ sources, destinations: { members: [] } }, ['sources'])), true);
});

test('a copy holds the picked rows in list order with item number, pin and artwork, and names its axis', () => {
  const edge = { members: [3, 1, 0], items: { 1: { pinned: true, art: ART, artB: '2', hideText: true }, 0: { art: UPLOAD } } };
  const b = copyRows(edge, 'sources', [1, 0], 'p1');
  assert.equal(b.sode, ROWS_MARK);
  assert.notEqual(ROWS_MARK, CLIP_MARK);
  assert.equal(b.axis, 'sources');
  assert.equal(b.panel, 'p1');
  assert.deepEqual(b.rows, [
    { n: 1, pinned: true, art: ART, artB: '2', hideText: true },
    { n: 0, art: UPLOAD },
  ]);
  assert.equal(copyRows(edge, 'sources', [], 'p1'), null);
});

test('readRows takes only a well formed block, and a Buttons cell block is not one', () => {
  const b = copyRows({ members: [1, 0] }, 'destinations', [1, 0], '');
  assert.deepEqual(readRows(rowsText(b)), b);
  assert.equal(readRows('hello'), null);
  assert.equal(readRows(JSON.stringify({ sode: CLIP_MARK, version: 1, panel: '', rows: 1, cols: 1, cells: [] })), null);
  assert.equal(readRows(JSON.stringify({ ...b, axis: 'levels' })), null);
  assert.equal(readRows(JSON.stringify({ ...b, rows: [{ n: -1 }] })), null);
  assert.equal(readRows(JSON.stringify({ ...b, rows: [{ n: 1 }, { n: 1 }] })), null);
  assert.equal(readRows(JSON.stringify({ ...b, rows: [{ n: 1, art: 'javascript:x' }] })), null);
  // Unknown fields are dropped.
  assert.deepEqual(readRows(JSON.stringify({ ...b, rows: [{ n: 1, extra: 1 }] })).rows, [{ n: 1 }]);
});

const block = (axis, rows, panel = 'p1') => ({ sode: ROWS_MARK, version: 1, panel, axis, rows });
const opts = { axis: 'sources', rights: null, kind: 'sources', panel: 'p1' };

test('a paste goes after the cursor row, or at the end, and keeps pin and artwork', () => {
  const edge = { members: [0, 1, 2] };
  const at = pasteRows(edge, router, block('sources', [{ n: 4, art: ART }, { n: 3 }]), { ...opts, after: 0 });
  assert.deepEqual(at.edge.members, [0, 4, 3, 1, 2]);
  assert.deepEqual(at.edge.items, { 4: { art: ART } });
  assert.deepEqual(at.added, [4, 3]);
  const end = pasteRows(edge, router, block('sources', [{ n: 4, pinned: true }]), opts);
  // A pinned row goes up with the other pins, as a pin does.
  assert.deepEqual(end.edge.members, [4, 0, 1, 2]);
  assert.deepEqual(end.edge.items[4], { pinned: true });
});

test('a paste refuses a block of the other axis and a list that follows the router', () => {
  const edge = { members: [0] };
  const other = pasteRows(edge, router, block('destinations', [{ n: 3 }]), opts);
  assert.equal(other.refused, 'axis');
  assert.equal(other.edge, edge);
  assert.match(pasteRowsNote(other, 'source'), /Nothing pasted: these are destinations/);
  const all = { members: 'all' };
  const following = pasteRows(all, router, block('sources', [{ n: 3 }]), opts);
  assert.equal(following.refused, 'following');
  assert.equal(following.edge, all);
  assert.match(pasteRowsNote(following, 'source'), /Follow the router is on/);
});

test('a paste skips items already in the list and items outside the Rights, and keeps items the router does not report', () => {
  const edge = { members: [0, 1], items: { 0: { art: ART } } };
  const r = pasteRows(edge, router, block('sources', [{ n: 0 }, { n: 2 }, { n: 3 }, { n: 99 }]), { ...opts, rights: { sources: [0, 1, 2, 99] }, after: 1 });
  assert.deepEqual(r.edge.members, [0, 1, 2, 99]);
  // The row already there keeps its place and its own artwork.
  assert.deepEqual(r.edge.items, { 0: { art: ART } });
  assert.deepEqual(r.added, [2, 99]);
  assert.equal(r.have, 1);
  assert.equal(r.outside, 1);
  assert.equal(pasteRowsNote(r, 'source'), 'Pasted 2 sources. Skipped 1 source already in the list. Skipped 1 source outside your Rights.');
  const none = pasteRows(edge, router, block('sources', [{ n: 0 }]), opts);
  assert.equal(none.edge, edge);
  assert.equal(pasteRowsNote(none, 'source'), 'Nothing pasted. Skipped 1 source already in the list.');
});

test('an uploaded image stays with its panel; a library or vendored icon pastes anywhere', () => {
  const rows = [
    { n: 2, art: UPLOAD },
    { n: 3, art: ART },
  ];
  const same = pasteRows({ members: [] }, router, block('sources', rows, 'p1'), opts);
  assert.equal(same.edge.items[2].art, UPLOAD);
  assert.equal(same.lostArt, 0);
  const other = pasteRows({ members: [] }, router, block('sources', rows, 'p2'), opts);
  assert.equal(other.edge.items[2], undefined);
  assert.equal(other.edge.items[3].art, ART);
  assert.equal(other.lostArt, 1);
  assert.match(pasteRowsNote(other, 'source'), /lost its uploaded image/);
  // An unsaved panel has no images of its own.
  assert.equal(pasteRows({ members: [] }, router, block('sources', rows, ''), { ...opts, panel: '' }).lostArt, 1);
});

test('after a Remove the pick is the name before the removed ones, else the name after', () => {
  assert.equal(pickAfterRemove([5, 6, 7, 8], new Set([7, 8])), 6);
  assert.equal(pickAfterRemove([5, 6, 7, 8], new Set([6, 8])), 5);
  assert.equal(pickAfterRemove([5, 6, 7, 8], new Set([5, 6])), 7);
  assert.equal(pickAfterRemove([5], new Set([5])), null);
});
