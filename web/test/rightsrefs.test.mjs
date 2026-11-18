import { test } from 'node:test';
import assert from 'node:assert/strict';
import { allowed, planOutside, draftOf, rightsOf } from '../rightsrules.js';
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
