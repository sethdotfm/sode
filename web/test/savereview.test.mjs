import test from 'node:test';
import assert from 'node:assert/strict';
import {
  diffRows,
  groupRows,
  saveLabel,
  unsavedWords,
  refusalNote,
  showValue,
  reconnects,
  reconnectTargets,
  reconnectQuestion,
  sectionOfKey,
  critical,
} from '../savereviewrules.js';
import { routerDisplay } from '../routerlistrules.js';

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
  assert.equal(saveLabel(0, 2), 'Nothing to save');
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
  assert.equal(refusalNote({ reason: 'Role' }, showValue), 'Not saved. Your Role is not allowed to change this.');
  assert.equal(refusalNote({ reason: 'needs Approval' }, showValue), 'Not saved. This needs Approval.');
  assert.equal(refusalNote({ reason: 'needs PIN recheck' }, showValue), 'Not saved. This needs a PIN recheck.');
  assert.equal(refusalNote({ reason: 'invalid', err: 'levels must be 1 to 256' }, showValue), 'Not saved. Levels must be 1 to 256.');
});

test('router connection fields reconnect; others do not', () => {
  for (const a of ['address', 'matrix', 'levels', 'levelWireOffset', 'nameLength', 'nameRequests']) assert.ok(reconnects(`router/ua/${a}`), a);
  for (const k of ['router/ua/title', 'router/ua/fireMode', 'routerShow', 'brandText', 'locks', 'routers/order']) assert.ok(!reconnects(k), k);
  // A new order reconnects no router and is not Critical.
  assert.ok(!critical('routers/order'));
  // The flat keys of the first router are gone: router/<uid>/<attribute> names every router.
  for (const k of ['router', 'matrix', 'levels']) assert.ok(!reconnects(k), k);
});

test('every router key is in the Routers section, the first router too', () => {
  for (const k of [
    'router/ua/address',
    'router/ub/levels',
    'router/ua/fireMode',
    'router/ua/protectDevice',
    'router/new:1/add',
    'router/ub/remove',
    'routerShow',
  ])
    assert.equal(sectionOfKey(k), 'routers', k);
});

const ROUTERS = [
  { uid: 'ua', prefix: 'A', title: 'Studio A', address: '10.0.0.1:8910' },
  { uid: 'ub', prefix: 'B', title: 'Stage 2', address: '10.0.0.2:8910' },
  { uid: 'uc', prefix: 'C', title: '', address: '10.0.0.3:8910' },
];
const NEW = { ua: '10.0.0.11:8910', ub: '10.0.0.12:8910', uc: '10.0.0.3:8910' };

test('the reconnect question names the router that reconnects and its new address, not the first router', () => {
  const t = reconnectTargets(['router/ub/address', 'brandText'], ROUTERS, routerDisplay, (uid) => NEW[uid]);
  assert.deepEqual(t, [{ uid: 'ub', name: 'B Stage 2', address: '10.0.0.12:8910' }]);
  assert.deepEqual(reconnectQuestion(t, true), { title: 'Save and reconnect B Stage 2 to 10.0.0.12:8910?', lines: [] });
});

test('the reconnect question names each router when two or more reconnect, in list order', () => {
  const keys = ['router/uc/matrix', 'router/ua/address', 'router/ua/levels', 'router/ub/address'];
  const t = reconnectTargets(keys, ROUTERS, routerDisplay, (uid) => NEW[uid]);
  assert.deepEqual(
    t.map((x) => x.uid),
    ['ua', 'ub', 'uc'],
  );
  assert.deepEqual(reconnectQuestion(t, true), {
    title: 'Save and reconnect 3 routers?',
    lines: ['A Studio A to 10.0.0.11:8910', 'B Stage 2 to 10.0.0.12:8910', 'Router C to 10.0.0.3:8910'],
  });
});

test('with one router the reconnect question gives only the address', () => {
  const t = reconnectTargets(['router/ua/address'], [ROUTERS[0]], routerDisplay, () => '10.0.0.9:8910');
  assert.deepEqual(reconnectQuestion(t, false), { title: 'Save and reconnect to 10.0.0.9:8910?', lines: [] });
  assert.deepEqual(reconnectQuestion([{ uid: '', name: 'Router A', address: '' }], false).title, 'Save and reconnect?');
});

test('unsavedWords: the Exit warning names each section with changes and says what to do', () => {
  assert.equal(unsavedWords([]), '');
  assert.equal(unsavedWords(['Routers']), 'Routers has unsaved changes. Save, or press Discard and exit.');
  assert.equal(unsavedWords(['Routers', 'Locks']), 'Routers and Locks have unsaved changes. Save, or press Discard and exit.');
  assert.equal(unsavedWords(['Server', 'Routers', 'Locks']), 'Server, Routers and Locks have unsaved changes. Save, or press Discard and exit.');
});
