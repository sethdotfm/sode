// What the panel page reads from the URL and the API (panelrules.js), and the text it shows.
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  panelIdFrom,
  keyFromHash,
  kioskOf,
  openFailText,
  sessionNote,
  envFrom,
  namesFrom,
  cellsIn,
  errorText,
  offlineOf,
  currentLabel,
} from '../panelrules.js';
import { createState, currentOfDestination } from '../panelmodel.js';

test('the panel id is the last path segment of /panels/<id>, and the editor and list are not panels', () => {
  assert.equal(panelIdFrom('/panels/p-1a2b3c4d'), 'p-1a2b3c4d');
  assert.equal(panelIdFrom('/panels/p-1a2b3c4d/'), 'p-1a2b3c4d');
  assert.equal(panelIdFrom('/sode/panels/p-1a2b3c4d'), 'p-1a2b3c4d');
  assert.equal(panelIdFrom('/panels/editor'), '');
  assert.equal(panelIdFrom('/panels/'), '');
  assert.equal(panelIdFrom('/panels'), '');
});

test('a key in the URL fragment means the panel acts as its API client, no fragment means Session mode', () => {
  assert.equal(keyFromHash('#key=sode_abc123'), 'sode_abc123');
  assert.equal(keyFromHash('#x=1&key=sode_abc'), 'sode_abc');
  assert.equal(keyFromHash(''), '');
  assert.equal(keyFromHash('#key='), '');
  assert.equal(keyFromHash('#keys=sode_x'), '');
});

test('the tally and Protected answers become the model env, with only the router levels', () => {
  const e = envFrom(
    {
      levels: [
        { level: 1, label: 'V' },
        { level: 2, label: 'A' },
      ],
      routes: [
        { dst: 0, level: 1, src: 5 },
        { dst: 0, level: 2, src: -1 },
      ],
    },
    {
      protected: [
        { dst: 3, level: 1 },
        { dst: 3, level: 2 },
      ],
    },
  );
  assert.deepEqual(e.levels, [1, 2]);
  assert.equal(e.tally.get(0).get(1), 5);
  assert.equal(e.tally.get(0).get(2), -1);
  assert.deepEqual([...e.protectedDsts], [3]);
});

test('names are looked up by router number, with a fallback that shows the number the router screen shows', () => {
  const n = namesFrom({ numberOffset: 1, sources: [{ n: 0, name: 'CAM 1' }], destinations: [{ n: 2, name: 'MON 2' }] });
  assert.equal(n.source(0), 'CAM 1');
  assert.equal(n.destination(2), 'MON 2');
  assert.equal(n.source(9), 'Source 10');
  assert.equal(n.destination(9), 'Destination 10');
});

test('a cell shows its caption instead of the Router name', () => {
  const n = namesFrom({ numberOffset: 0, sources: [{ n: 0, name: 'CAM 1' }], destinations: [] });
  assert.equal(n.of({ kind: 'source', n: 0 }), 'CAM 1');
  assert.equal(n.of({ kind: 'source', n: 0, caption: 'Host cam' }), 'Host cam');
});

test('only whole cells inside the grid are drawn, and the first of two that overlap wins', () => {
  const panel = {
    rows: 2,
    cols: 3,
    cells: [
      { r: 1, c: 1, kind: 'source', n: 0 },
      { r: 1, c: 1, kind: 'destination', n: 1 },
      { r: 1, c: 2, cs: 2, kind: 'take' },
      { r: 2, c: 3, rs: 2, kind: 'take' },
      { r: 3, c: 1, kind: 'source', n: 4 },
      { r: 2, c: 1, kind: 'mystery' },
      { r: 0, c: 1, kind: 'source', n: 1 },
      { r: 2, c: 2, kind: 'source' },
      null,
    ],
  };
  assert.deepEqual(
    cellsIn(panel).map((c) => [c.kind, c.r, c.c, c.rs, c.cs]),
    [
      ['source', 1, 1, 1, 1],
      ['take', 1, 2, 1, 2],
    ],
  );
});

test('error text names the Source and Destination the way the cells do', () => {
  const n = namesFrom({ numberOffset: 0, sources: [{ n: 4, name: 'CAM 5' }], destinations: [{ n: 0, name: 'MON A' }] });
  assert.equal(errorText({ code: 'no-destination' }, n), 'Pick a destination first');
  assert.equal(errorText({ code: 'protected', dst: 0 }, n), 'MON A is protected');
  assert.equal(errorText({ code: 'no-common-levels', src: 4, dst: 0 }, n), 'CAM 5 and MON A have no level in common');
  assert.equal(
    errorText({ code: 'refused', reason: 'outside_rights', message: 'destination 1 is outside the rights' }, n),
    'destination 1 is outside the rights',
  );
  assert.equal(errorText(null, n), '');
});

test('a destination cell reads its own current source on the levels it and the panel share', () => {
  const e = envFrom(
    {
      levels: [{ level: 1 }, { level: 2 }],
      routes: [
        { dst: 1, level: 1, src: 6 },
        { dst: 1, level: 2, src: 7 },
      ],
    },
    { protected: [] },
  );
  assert.equal(currentOfDestination(createState(), e, { kind: 'destination', n: 1 }), -1);
  assert.equal(currentOfDestination(createState(), e, { kind: 'destination', n: 1, levels: [1] }), 6);
  assert.equal(currentOfDestination(createState({ levels: [2] }), e, { kind: 'destination', n: 1 }), 7);
});

test('a panel runs as a kiosk when its settings or its link say so', () => {
  assert.equal(kioskOf('', { settings: { kiosk: true } }), true);
  assert.equal(kioskOf('#key=sode_abc&kiosk=1', { settings: {} }), true);
  assert.equal(kioskOf('#kiosk=1', {}), true);
  assert.equal(kioskOf('#key=sode_abc', { settings: {} }), false);
  assert.equal(kioskOf('#kiosk=0', null), false);
  assert.equal(kioskOf('', null), false);
});

test('opening a panel fails with a reason that says what to do with the key', () => {
  assert.match(openFailText(401, true), /rotated or revoked/);
  assert.match(openFailText(404, true), /No panel/);
  assert.match(openFailText(500, false), /could not be opened/);
});

test('the page is offline when Sode does not answer or the router link is down, and says which', () => {
  assert.equal(offlineOf({ failed: false, routerUp: true }), null);
  assert.match(offlineOf({ failed: true, routerUp: true }).detail, /Sode/);
  const router = offlineOf({ failed: false, routerUp: false });
  assert.match(router.detail, /router/);
  assert.equal(router.title, 'Offline');
  // Both down: the page cannot know about the router, so it blames Sode.
  assert.match(offlineOf({ failed: true, routerUp: false }).detail, /Sode/);
});

test('the Current Source cell names the viewed destination, and a caption still wins', () => {
  assert.equal(currentLabel({ kind: 'current' }, 'CAM 1'), 'CAM 1 is currently:');
  assert.equal(currentLabel({ kind: 'current' }, ''), 'Currently');
  assert.equal(currentLabel({ kind: 'current', caption: 'Now on' }, 'CAM 1'), 'Now on');
});

test('sessionNote says something for a refused or ended key', () => {
  assert.match(sessionNote('refused', 'too many keys').text, /too many keys/);
  assert.equal(sessionNote('refused').retry, true);
  assert.match(sessionNote('ended').text, /Reload/);
  assert.equal(sessionNote('ended').retry, false);
  assert.equal(sessionNote('request').text, '');
});
