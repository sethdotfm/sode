// Panel Name and Instance Name cells: kept by cellsIn without a number, offered by Type and Find, never pressed.
import test from 'node:test';
import assert from 'node:assert/strict';
import { cellsIn, nameCellText, namesFrom, SHOW_KINDS } from '../panelrules.js';
import { kindChoices, kindLabel } from '../editorrules.js';
import { findEntries, findPlan, applyFind } from '../findrules.js';
import { createGrid, select, cellAt, isUnassigned } from '../gridmodel.js';

const cell = (kind, extra = {}) => ({ r: 1, c: 1, rs: 1, cs: 1, kind, ...extra });

test('a name cell needs no number and is never unassigned', () => {
  for (const kind of ['panelName', 'instanceName']) {
    assert.equal(cellsIn({ rows: 1, cols: 1, cells: [cell(kind)] }).length, 1, kind);
    assert.equal(isUnassigned(cell(kind)), false, kind);
    assert.ok(SHOW_KINDS.includes(kind), kind);
  }
});

test('a name cell shows the name, with a caption as the line above it, and its kind words without one', () => {
  assert.deepEqual(nameCellText(cell('panelName'), { panelName: 'Lobby Wall', instance: 'Front desk' }), { caption: '', value: 'Lobby Wall' });
  assert.deepEqual(nameCellText(cell('instanceName'), { panelName: 'Lobby Wall', instance: 'Front desk' }), { caption: '', value: 'Front desk' });
  assert.deepEqual(nameCellText(cell('instanceName', { caption: 'This is' }), { instance: 'Front desk' }), { caption: 'This is', value: 'Front desk' });
  assert.equal(nameCellText(cell('instanceName'), { panelName: 'Lobby Wall' }).value, 'Instance Name');
  assert.equal(nameCellText(cell('panelName')).value, 'Panel Name');
});

test('Type and the router names give the kinds their words', () => {
  const kinds = kindChoices().map((x) => x.kind);
  assert.ok(kinds.includes('panelName') && kinds.includes('instanceName'));
  assert.equal(kindLabel('panelName'), 'Panel Name');
  assert.equal(kindLabel('instanceName'), 'Instance Name');
  assert.equal(namesFrom({}).of(cell('instanceName')), 'Instance Name');
});

test('Find offers both and places one as a cell with no number', () => {
  const entries = findEntries({ sources: [], destinations: [] }, { presets: [] }, null);
  const e = entries.find((x) => x.category === 'instanceName');
  assert.ok(e && !e.off);
  assert.ok(entries.some((x) => x.category === 'panelName'));
  const g = select(createGrid(1, 2), 1, 2);
  const out = applyFind(g, findPlan(g, [e]));
  const placed = cellAt(out, 1, 2);
  assert.equal(placed.kind, 'instanceName');
  assert.equal(placed.n, undefined);
});
