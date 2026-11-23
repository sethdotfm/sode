import test from 'node:test';
import assert from 'node:assert/strict';
import { gridParts } from '../gridpanelmode.js';
import { gridPanel } from './gridpanelfixture.mjs';

const instant = gridPanel({ settings: { mode: 'instant' } });

test('the panel page runs every part, in the mode its panel sets', () => {
  const p = gridParts({ ui: { name: {} }, embedded: false }, instant);
  assert.deepEqual(p, { panel: true, mode: 'instant', head: true, panelUndo: true, ownConfirm: true, keyList: true });
});

test('the Live Editor has its panel and its undo, but no deck header', () => {
  const p = gridParts({ ui: {}, embedded: false }, instant);
  assert.equal(p.head, false);
  assert.equal(p.panel, true);
  assert.equal(p.panelUndo, true);
  assert.equal(p.mode, 'instant');
});

test('an adapter with its own confirm has no panel: Take mode, no panel Undo, no deck header, no own confirm', () => {
  const p = gridParts({ ui: { name: {} }, confirm() {} }, instant);
  assert.deepEqual(p, { panel: false, mode: 'take', head: false, panelUndo: false, ownConfirm: false, keyList: false });
});

test('an adapter with no panel at all still runs in Take mode', () => {
  assert.equal(gridParts({ ui: {}, confirm() {} }, undefined).mode, 'take');
});
