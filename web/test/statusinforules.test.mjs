import { test } from 'node:test';
import assert from 'node:assert/strict';
import { logReadable, spanText, routerUpSince } from '../statusinforules.js';

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
