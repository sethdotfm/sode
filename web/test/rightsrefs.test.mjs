import { test } from 'node:test';
import assert from 'node:assert/strict';
import { allowed, planOutside, draftOf, rightsOf, newEntry, accessReq } from '../rightsrules.js';
import { rightsAllow, fixedDestsOf } from '../panelrules.js';

const ref = (n, name) => ({ router: 'u1', n, ...(name ? { name } : {}) });

test('Rights lists of router references allow the same numbers as bare numbers', () => {
  const r = { sources: [ref(3, 'CAM 4')], dsts: [] };
  for (const check of [allowed, rightsAllow]) {
    assert.equal(check(r, 'sources', 3), true);
    assert.equal(check(r, 'sources', 4), false);
    assert.equal(check(r, 'dsts', 0), false);
    assert.equal(check(r, 'levels', 1), true);
  }
  const g = (src, levels) => ({ src: { n: src }, levels });
  const plan = { verb: 'patch', items: [{ dst: { n: 9 }, groups: [g(3, [1])] }] };
  assert.equal(planOutside({ sources: [ref(3)], dsts: [ref(9)] }, plan, 1), '');
  assert.equal(planOutside({ dsts: [ref(8)] }, plan, 1), 'destination 10 is outside your rights');
});

test('a right names its router: A.5 allows A.5 and refuses B.5', () => {
  const r = { sources: [{ router: 'a', n: 5 }], dsts: [{ router: 'a', n: 5 }] };
  assert.equal(allowed(r, 'sources', 5, 'a'), true);
  assert.equal(allowed(r, 'sources', 5, 'b'), false);
  assert.equal(allowed(r, 'dsts', 5, 'b'), false);
  const uid = (end) => end.router || 'a';
  const plan = (router) => ({ verb: 'patch', items: [{ dst: { n: 5, router }, groups: [{ src: { n: 5, router }, levels: [1] }] }] });
  assert.equal(planOutside(r, plan(undefined), 1, String, uid), '');
  assert.equal(planOutside(r, plan('b'), 1, String, uid), 'destination 6 is outside your rights');
  assert.equal(planOutside({ sources: r.sources }, plan('b'), 1, String, uid), 'source 6 is outside your rights');
});

test('a preset is checked by id, so a rename keeps who can fire it', () => {
  const plan = { verb: 'preset', preset: 'Wall 2', presetId: 'p1', items: [] };
  assert.equal(planOutside({ presets: ['p1'] }, plan, 0), '');
  assert.equal(planOutside({ presets: ['p2'] }, plan, 0), 'preset "Wall 2" is outside your rights');
});

test('the editor shows preset names, sends ids, and keeps ids it cannot name', () => {
  const presets = [
    { id: 'p1', name: 'Wall' },
    { id: 'p2', name: 'News' },
  ];
  const d = draftOf({ sources: [ref(0), ref(1)], presets: ['p1', 'gone'] }, 1, presets);
  assert.equal(d.sources.text, '1-2');
  assert.equal(d.presets.text, 'Wall');
  d.presets.text = 'wall\nNews';
  assert.deepEqual(rightsOf(d, 1, 4, presets).rights.presets, ['p1', 'p2', 'gone']);
  d.presets.text = 'Nope';
  assert.match(rightsOf(d, 1, 4, presets).error, /Presets: no preset is named "Nope"/);
});

test('fixed destinations read as numbers from references or numbers', () => {
  const of = (fixedDests) => fixedDestsOf({ settings: { fixedDests } });
  assert.deepEqual(of([ref(12, 'MON 13'), 0, ref(40)]), [12, 0, 40]);
  assert.deepEqual(of([ref(2), 2]), [], 'a repeat reads as off');
  assert.deepEqual(of([{ router: 'u1' }]), []);
});

test("a user entry names that user's Me presets as well as Everyone presets", () => {
  const e = newEntry({ presets: ['me1'] }, 1, {}, [
    { id: 'p1', name: 'Wall' },
    { id: 'me1', name: 'Mine' },
  ]);
  assert.equal(e.draft.presets.text, 'Mine');
  e.dirty.add('rights');
  e.draft.presets.text = 'Mine\nWall';
  const r = accessReq({ entries: new Map([['user:1', e]]), users: [{ rid: 1, name: 'Ana' }], clients: [], off: 1, levels: 4 });
  assert.deepEqual(r.users[0].rights.presets, ['me1', 'p1']);
});
