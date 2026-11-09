// A panel's refusals before arming (Rights, Protected, levels) and its Result display. The server
// enforces the same rules; these must stay in step with it (internal/app/rights.go).
import test from 'node:test';
import assert from 'node:assert/strict';
import { createState, pressDestination, pressSource, pressPreset, armed, finishTake, resultShown, errorOf, RESULT_MS } from '../panelmodel.js';
import { envFrom, namesFrom, errorText } from '../panelrules.js';

const env = (over = {}) => ({ levels: [1, 2], tally: new Map(), protectedDsts: new Set(), ...over });
const T0 = 1000;
const rights = (r) => env({ rights: r });
const press = (e, d, s, settings) =>
  pressSource(pressDestination(createState(settings), { kind: 'destination', n: d }, e, T0), { kind: 'source', n: s }, e, T0);

test('a Destination outside the Rights refuses the Source press and never arms', () => {
  const e = rights({ dsts: [1] });
  const st = press(e, 0, 4);
  assert.deepEqual(errorOf(st, T0), { code: 'outside-rights', what: 'destination', src: 4, dst: 0, quiet: false, at: T0 });
  assert.equal(armed(st, e), false);
  assert.equal(st.staged.length, 0);
});

test('a Source outside the Rights is refused, and an empty list allows nothing', () => {
  assert.equal(errorOf(press(rights({ sources: [3] }), 0, 4), T0).what, 'source');
  assert.equal(errorOf(press(rights({ sources: [] }), 0, 4), T0).code, 'outside-rights');
  assert.equal(press(rights({ sources: [4], dsts: [0] }), 0, 4).staged.length, 1);
});

test('hidden Visibility marks the refusal quiet', () => {
  assert.equal(errorOf(press(rights({ dsts: [], visibility: 'hidden' }), 0, 4), T0).quiet, true);
});

test('a Protected Destination is refused ahead of the Rights', () => {
  const st = press(env({ protectedDsts: new Set([0]), rights: { dsts: [] } }), 0, 4);
  assert.equal(errorOf(st, T0).code, 'protected');
});

test('a take sends only the levels the Rights allow, and none left is no common level', () => {
  assert.deepEqual(press(rights({ levels: [2] }), 0, 4).staged[0].levels, [2]);
  assert.equal(errorOf(press(rights({ levels: [3] }), 0, 4), T0).code, 'no-common-levels');
});

test('a Preset outside the Rights, or holding a route outside them, is refused whole', () => {
  const presets = new Map([['me:A', [{ src: 4, dsts: [0, 1], levels: [] }]]]);
  const cell = { kind: 'preset', scope: 'me', name: 'A' };
  const run = (r) => pressPreset(createState(), cell, env({ presets, rights: r }), T0);
  assert.equal(errorOf(run({ presets: ['B'] }), T0).code, 'outside-rights');
  assert.equal(errorOf(run({ dsts: [0] }), T0).dst, 1);
  assert.equal(run({ dsts: [0] }).staged.length, 0);
  assert.equal(run({ presets: ['A'] }).staged.length, 2);
});

test('a Result shows as an icon, with its word only when the panel asks for text', () => {
  const done = finishTake(createState(), { results: [{ dst: 0, result: 'already' }] }, T0);
  assert.deepEqual(resultShown(done, 0, T0), { result: 'already', word: 'Already', text: '' });
  const texted = finishTake(createState({ resultText: true }), { results: [{ dst: 0, result: 'failed' }] }, T0);
  assert.deepEqual(resultShown(texted, 0, T0), { result: 'failed', word: 'Failed', text: 'Failed' });
  assert.equal(resultShown(texted, 0, T0 + RESULT_MS + 1), null);
});

test('the outside-rights error names the cell, and nothing of numbers when quiet', () => {
  const n = namesFrom({ numberOffset: 0, sources: [{ n: 4, name: 'CAM 5' }], destinations: [{ n: 0, name: 'MON A' }] });
  const err = (what, quiet) => errorText({ code: 'outside-rights', what, dst: 0, src: 4, quiet }, n);
  assert.equal(err('destination', false), "MON A is outside this panel's rights");
  assert.equal(err('source', false), "CAM 5 is outside this panel's rights");
  assert.equal(err('source', true), "That is outside this panel's rights");
});

test('the rights the server sent reach the env, and none means everything', () => {
  assert.equal(envFrom({}, {}).rights, null);
  assert.deepEqual(envFrom({}, {}, {}, { dsts: [1] }).rights, { dsts: [1] });
});
