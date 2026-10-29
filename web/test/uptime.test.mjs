import test from 'node:test';
import assert from 'node:assert/strict';
import { slots, summary, drops, slotFor, slotText, rangeFor, presetId, PRESETS, MAX_SLOTS } from '../uptime.js';

const MIN = 60_000, HOUR = 60 * MIN, DAY = 24 * HOUR;
const colours = (d) => slots(d).map((s) => s.colour);
// 4 hours in 1 hour slots, "now" at the end.
const base = (over) => ({ start: 'up', trans: [], counts: [], from: 0, to: 4 * HOUR, slot: HOUR, now: 4 * HOUR, ...over });

test('up all the time is green', () => {
  assert.deepEqual(colours(base()), ['ok', 'ok', 'ok', 'ok']);
});

test('down for a whole slot is red, part of one is yellow', () => {
  const d = base({ trans: [{ t: HOUR + 30 * MIN, s: 'down' }, { t: 3 * HOUR, s: 'up' }] });
  // 0 ok · 1 half down · 2 down · 3 up
  assert.deepEqual(colours(d), ['ok', 'warn', 'error', 'ok']);
  assert.equal(slots(d)[1].upMs, 30 * MIN);
});

test('failed or slow messages make an up slot yellow', () => {
  const d = base({ counts: [{ i: 1, bad: 2, slow: 0 }, { i: 2, bad: 0, slow: 1 }] });
  assert.deepEqual(colours(d), ['ok', 'warn', 'warn', 'ok']);
});

test('nothing recorded yet is grey; the record starting mid-slot counts from there', () => {
  const d = base({ start: '', trans: [{ t: HOUR + 30 * MIN, s: 'up' }] });
  assert.deepEqual(colours(d), ['none', 'ok', 'ok', 'ok']);
  assert.equal(slots(d)[1].knownMs, 30 * MIN);
});

test('time to come is not drawn as down', () => {
  const d = base({ now: 2 * HOUR + 10 * MIN, trans: [{ t: 2 * HOUR + 5 * MIN, s: 'down' }] });
  assert.deepEqual(colours(d), ['ok', 'ok', 'warn', 'future']);
  assert.equal(slots(d)[2].knownMs, 10 * MIN);
});

test('state carries across slots with no change in them', () => {
  const d = base({ start: 'down', trans: [{ t: 3 * HOUR, s: 'up' }] });
  assert.deepEqual(colours(d), ['error', 'error', 'error', 'ok']);
});

test('Sode being off counts as down', () => {
  const d = base({ trans: [{ t: HOUR, s: 'off' }, { t: 3 * HOUR, s: 'down' }, { t: 3 * HOUR + 1, s: 'up' }] });
  assert.deepEqual(colours(d).slice(0, 3), ['ok', 'error', 'error']);
});

test('summary: rounds down, counts drops, and always has text', () => {
  assert.equal(summary(base()), '100% up · 0 drops');
  const d = base({ trans: [{ t: HOUR, s: 'down' }, { t: HOUR + 36 * 1000, s: 'up' }], counts: [{ i: 0, bad: 3, slow: 2 }] });
  assert.equal(summary(d), '99.7% up · 1 drop · 3 failed · 2 slow');
  assert.equal(summary(base({ start: '' })), 'No record for this time');
});

test('drops counts up-to-not-up changes only', () => {
  const d = base({ trans: [{ t: 1, s: 'down' }, { t: 2, s: 'off' }, { t: 3, s: 'up' }, { t: 4, s: 'down' }] });
  assert.equal(drops(d), 2);
});

test('slot length: 24 h is 15 min slots, 7 days 1 h, and rows never pass MAX_SLOTS', () => {
  assert.equal(slotFor(0, DAY), 15 * MIN);
  assert.equal(slotFor(0, 7 * DAY), HOUR);
  for (const days of [0.001, 0.5, 3, 30, 200, 400, 5000]) {
    const slot = slotFor(0, days * DAY);
    assert.ok(Math.ceil((days * DAY) / slot) <= MAX_SLOTS, `${days} days → ${slot}`);
  }
});

test('every preset fits in MAX_SLOTS, and the day and week keep their slots', () => {
  for (const p of PRESETS) assert.ok(Math.ceil(p.span / slotFor(0, p.span)) <= MAX_SLOTS, p.id);
  assert.equal(slotFor(0, HOUR), MIN);
});

test('range: the preset back from now, or the times chosen', () => {
  const now = 100 * DAY;
  assert.deepEqual(rangeFor('7d', null, null, now), { custom: false, from: now - 7 * DAY, to: now });
  assert.deepEqual(rangeFor('nonsense', null, null, now), { custom: false, from: now - DAY, to: now });
  assert.deepEqual(rangeFor('1h', now - 3 * DAY, null, now), { custom: true, from: now - 3 * DAY, to: now });
  assert.deepEqual(rangeFor('1h', null, now - DAY, now), { custom: true, from: now - 8 * DAY, to: now - DAY });
  assert.equal(rangeFor('1h', now, now - DAY, now), null);
  assert.equal(presetId('30d'), '30d');
  assert.equal(presetId(null), '24h');
});

test('slot text says why, with the time', () => {
  const fmt = (t) => `t${t / MIN}`;
  const [ok, warn] = slots(base({ trans: [{ t: HOUR + 30 * MIN, s: 'down' }], counts: [{ i: 1, bad: 1, slow: 0 }] }));
  assert.equal(slotText(ok, fmt), 't0 – t60: router up');
  assert.match(slotText(warn, fmt), /router up 50% of the time, 1 message failed or needed a retry/);
});
