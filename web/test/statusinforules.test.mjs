import { test } from 'node:test';
import assert from 'node:assert/strict';
import { logReadable, spanText, routerUpSince, hoverRows, moreText, HOVER_ROWS, errText } from '../statusinforules.js';

test('logReadable follows the uptime rule: a Role, no Lock, and the log lock passed', () => {
  assert.equal(logReadable(null, {}), false);
  assert.equal(logReadable({ role: '' }, {}), false);
  assert.equal(logReadable({ role: 'standard' }, {}), true);
  assert.equal(logReadable({ role: 'standard' }, { log: true }), false);
  assert.equal(logReadable({ role: 'standard', user: 'Bo' }, { log: true }), true);
  assert.equal(logReadable({ role: 'administrator', user: 'Bo', held: true }, {}), false);
  assert.equal(logReadable({ role: 'administrator', user: 'Bo', locked: true }, {}), false);
});

test('spanText gives whole minutes from a minute on', () => {
  assert.equal(spanText(42), '42 seconds');
  assert.equal(spanText(125), '2 minutes');
  assert.equal(spanText(3 * 3600 + 5 * 60 + 9), '3 hours 5 minutes');
  assert.equal(spanText(2 * 86400 + 3600), '2 days 1 hour');
});

test('routerUpSince finds the last change to up', () => {
  const base = { from: 0, to: 100, now: 100 };
  assert.deepEqual(routerUpSince({ ...base, start: 'up', trans: [] }), { over: true });
  assert.deepEqual(
    routerUpSince({
      ...base,
      start: 'up',
      trans: [
        { t: 10, s: 'down' },
        { t: 40, s: 'up' },
      ],
    }),
    { since: 40 },
  );
  assert.deepEqual(
    routerUpSince({
      ...base,
      start: 'down',
      trans: [
        { t: 20, s: 'up' },
        { t: 30, s: 'up' },
      ],
    }),
    { since: 20 },
  );
  assert.equal(routerUpSince({ ...base, start: 'up', trans: [{ t: 50, s: 'down' }] }), null);
  assert.equal(routerUpSince({ ...base, start: '', trans: [] }), null);
  assert.equal(routerUpSince({ ...base, start: 'down', trans: [{ t: 120, s: 'up' }] }), null);
});

// A router as routerLinks gives it: state is the setDot state, lost ones are error.
const rt = (prefix, state) => ({ uid: prefix, prefix, title: '', addr: '', link: state === 'error' ? 'down' : 'up', state, err: '', never: false });

test('hoverRows shows every router up to HOVER_ROWS, lost routers first, config order inside each group', () => {
  assert.equal(HOVER_ROWS, 8);
  const list = [rt('A', 'ok'), rt('B', 'error'), rt('C', 'ok'), rt('D', 'warn'), rt('E', 'error')];
  const { rows, more } = hoverRows(list);
  assert.deepEqual(
    rows.map((x) => x.r.prefix),
    ['B', 'E', 'D', 'A', 'C'],
  );
  assert.equal(more, 0);
  assert.equal(rows[0].name, 'Router B');
  assert.equal(rows[0].word, 'Down');
});

test('hoverRows with exactly HOVER_ROWS routers shows them all, with no "And N more"', () => {
  const list = 'ABCDEFGH'.split('').map((p) => rt(p, 'ok'));
  const { rows, more } = hoverRows(list);
  assert.equal(rows.length, 8);
  assert.equal(more, 0);
});

test('hoverRows over the cap keeps the box at HOVER_ROWS lines: one row less, and more is never 1', () => {
  const nine = 'ABCDEFGHI'.split('').map((p) => rt(p, 'ok'));
  let r = hoverRows(nine);
  assert.equal(r.rows.length, 7);
  assert.equal(r.more, 2);
  const many = Array.from({ length: 24 }, (_, i) => rt(String.fromCharCode(65 + i), i >= 20 ? 'error' : 'ok'));
  r = hoverRows(many);
  assert.equal(r.rows.length, 7);
  assert.equal(r.more, 17);
  assert.deepEqual(
    r.rows.slice(0, 4).map((x) => x.r.prefix),
    ['U', 'V', 'W', 'X'],
    'the lost routers are never cut off before the ones that are up',
  );
  assert.equal(hoverRows(nine, 4).rows.length, 3);
  assert.equal(hoverRows(nine, 4).more, 6);
});

test('moreText points to the router list', () => {
  assert.equal(moreText(17), 'And 17 more. Click the Routers icon to see all.');
});

test('errText puts the reason first, so a cut line keeps it', () => {
  assert.equal(errText('dial tcp 127.0.0.1:18911: connect: connection refused'), 'connection refused (127.0.0.1:18911)');
  assert.equal(errText('dial tcp 10.0.0.5:8910: i/o timeout'), 'i/o timeout (10.0.0.5:8910)');
  assert.equal(errText('read tcp 10.0.0.2:51000->10.0.0.5:8910: read: connection reset by peer'), 'connection reset by peer (10.0.0.5:8910)');
  assert.equal(errText('dial tcp: lookup ultrix.local: no such host'), 'no such host (dial tcp: lookup ultrix.local)');
  assert.equal(errText('no answer'), 'no answer');
  assert.equal(errText(''), '');
});
