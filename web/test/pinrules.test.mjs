import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { PINS_MAX, pinsProblem, cleanPins, addPin, removePin, togglePin, movePin, pinRows, missingRow } from '../pinrules.js';

// The Go tests read the same cases (internal/config/palette_test.go).
const cases = JSON.parse(readFileSync(new URL('./pin_cases.json', import.meta.url), 'utf8')).cases;

const OK = { ok: true, lock: false, why: '', offer: [] };
const rows = [
  { key: 'preset:global:a1', kind: 'preset', label: 'Morning show', access: OK },
  { key: 'page:log', kind: 'page', label: 'Communication Log', access: OK },
];

test('pinsProblem: the shared cases, with the server’s words', () => {
  for (const c of cases) assert.equal(pinsProblem(c.pins), c.problem, JSON.stringify(c.pins).slice(0, 80));
});

test('cleanPins: a stored list as the page uses it', () => {
  assert.deepEqual(cleanPins(null), []);
  assert.deepEqual(cleanPins({ key: 'page:log' }), []);
  assert.deepEqual(cleanPins([' page:log ', 'page:log', 'https://x.test/', 5, 'preset:global:a1']), ['page:log', 'preset:global:a1']);
  const many = Array.from({ length: 30 }, (_, i) => `preset:global:p${i}`);
  assert.equal(cleanPins(many).length, PINS_MAX);
});

test('addPin and togglePin: one key pins and unpins; the 25th is refused', () => {
  let list = [];
  for (let i = 0; i < PINS_MAX; i++) list = addPin(list, `preset:global:p${i}`).list;
  assert.equal(list.length, 24);
  assert.equal(addPin(list, 'page:log').err, 'a list holds at most 24 Palette pins');
  assert.equal(togglePin(list, 'page:log').err, 'a list holds at most 24 Palette pins');
  assert.deepEqual(addPin(['page:log'], 'page:log'), { list: ['page:log'] });
  assert.deepEqual(togglePin(['page:log'], 'preset:global:a1'), { list: ['page:log', 'preset:global:a1'], pinned: true });
  assert.deepEqual(togglePin(['page:log', 'preset:global:a1'], 'page:log'), { list: ['preset:global:a1'], pinned: false });
  assert.deepEqual(removePin(['page:log'], 'page:panels'), ['page:log']);
  assert.match(addPin([], 'url:https://x.test/').err, /does not know/);
});

test('movePin: by Alt+arrows or by a drag, never off the list', () => {
  const list = ['a:1', 'a:2', 'a:3'];
  assert.deepEqual(movePin(list, 0, { by: 1 }), ['a:2', 'a:1', 'a:3']);
  assert.equal(movePin(list, 0, { by: -1 }), null);
  assert.deepEqual(movePin(list, 0, { before: 3 }), ['a:2', 'a:3', 'a:1']);
  assert.deepEqual(movePin(list, 2, { before: 0 }), ['a:3', 'a:1', 'a:2']);
  assert.deepEqual(list, ['a:1', 'a:2', 'a:3']);
});

test('pinRows: the saved order; a gone row is a grey Missing row once the rows are read', () => {
  const list = ['page:log', 'preset:global:gone', 'preset:global:a1'];
  const r = pinRows(list, rows);
  assert.deepEqual(
    r.map((x) => x.key),
    list,
  );
  assert.equal(r[0], rows[1], 'a pin is the catalogue row itself');
  assert.equal(r[1].missing, true);
  assert.equal(r[1].access.ok, false);
  assert.equal(r[1].label, 'Preset');
  assert.deepEqual(
    pinRows(list, rows, { ready: false }).map((x) => x.key),
    ['page:log', 'preset:global:a1'],
  );
  assert.equal(missingRow('filterSet:user:f1').label, 'Filter set');
});
