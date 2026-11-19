// The one Save review's rules: the was/now diff, its grouping by topic and section, the Save button and the marks
// on refused fields.
import test from 'node:test';
import assert from 'node:assert/strict';
import { diffRows, groupRows, saveLabel, refusalNote, showValue, reconnects } from '../savereviewrules.js';

test('only changed fields make rows, with was and now', () => {
  const base = { brandText: 'Sode', confirmEnter: false, matrix: 0 };
  const now = { brandText: 'Truck 4', confirmEnter: false, matrix: 0 };
  assert.deepEqual(diffRows(base, now), [{ key: 'brandText', sub: null, was: 'Sode', now: 'Truck 4' }]);
});

test('an object field gives one row per changed key', () => {
  const base = { locks: { main: false, log: false, panel: false }, browserDefaults: { theme: 'dark', clock: '24' } };
  const now = { locks: { main: true, log: false, panel: true }, browserDefaults: { theme: 'dark', clock: '24' } };
  assert.deepEqual(diffRows(base, now), [
    { key: 'locks', sub: 'main', was: false, now: true },
    { key: 'locks', sub: 'panel', was: false, now: true },
  ]);
});

test('level colors give one row per changed level; a list field is one row', () => {
  const base = { levelColors: ['', '#112233'], trustedProxies: ['10.0.0.2'] };
  const now = { levelColors: ['#aabbcc', '#112233', ''], trustedProxies: ['10.0.0.2', '10.0.0.3'] };
  assert.deepEqual(diffRows(base, now), [
    { key: 'levelColors', sub: 0, was: '', now: '#aabbcc' },
    { key: 'trustedProxies', sub: null, was: ['10.0.0.2'], now: ['10.0.0.2', '10.0.0.3'] },
  ]);
});

test('a key the form does not read is not a change', () => {
  assert.deepEqual(diffRows({ hiddenDstsRouter: 'A' }, {}), []);
});

test('rows group by topic, then section, in page order', () => {
  const place = {
    brandText: { topic: 'Look and feel', section: 'Branding', order: 2 },
    confirmEnter: { topic: 'Look and feel', section: 'Confirm and results', order: 4 },
    router: { topic: 'Routers', section: 'Router & matrix', order: 7 },
    brandBorder: { topic: 'Look and feel', section: 'Branding', order: 2 },
  };
  const rows = ['router', 'confirmEnter', 'brandText', 'brandBorder'].map((key) => ({ key, sub: null, was: 1, now: 2 }));
  const out = groupRows(rows, (r) => place[r.key]);
  assert.deepEqual(
    out.map((t) => [t.topic, t.sections.map((s) => [s.section, s.rows.map((r) => r.key)])]),
    [
      [
        'Look and feel',
        [
          ['Branding', ['brandText', 'brandBorder']],
          ['Confirm and results', ['confirmEnter']],
        ],
      ],
      ['Routers', [['Router & matrix', ['router']]]],
    ],
  );
});

test('the Save button names what saves and what stays', () => {
  assert.equal(saveLabel(1, 0), 'Save 1 change');
  assert.equal(saveLabel(3, 0), 'Save 3 changes');
  assert.equal(saveLabel(2, 2), 'Save 2, keep 2 unsaved');
  assert.equal(saveLabel(0, 2), 'Nothing can save');
});

test('values show as people read them', () => {
  assert.equal(showValue(true), 'On');
  assert.equal(showValue(false), 'Off');
  assert.equal(showValue(''), 'Blank');
  assert.equal(showValue([]), 'Blank');
  assert.equal(showValue(['a', 'b']), 'a, b');
  assert.equal(showValue(30), '30');
});

test('a refused field says why; a conflict shows the saved value', () => {
  assert.equal(refusalNote({ reason: 'conflict', value: 'Truck 9' }, showValue), 'Not saved. Someone else saved "Truck 9" after you opened Settings.');
  assert.equal(refusalNote({ reason: 'Role' }, showValue), 'Not saved. Your Role cannot change this.');
  assert.equal(refusalNote({ reason: 'needs Approval' }, showValue), 'Not saved. This needs Approval.');
  assert.equal(refusalNote({ reason: 'needs PIN recheck' }, showValue), 'Not saved. This needs a PIN recheck.');
  assert.equal(refusalNote({ reason: 'invalid', err: 'levels must be 1 to 256' }, showValue), 'Not saved. Levels must be 1 to 256.');
});

test('router connection fields reconnect; others do not', () => {
  for (const k of ['router', 'matrix', 'levels', 'levelWireOffset', 'nameLength', 'nameRequests']) assert.ok(reconnects(k), k);
  for (const k of ['routerName', 'brandText', 'locks']) assert.ok(!reconnects(k), k);
});
