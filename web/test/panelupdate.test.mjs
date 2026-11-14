// When an open panel applies a changed revision (panelupdate.js), with no DOM and no clock.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createUpdate, noticeRev, snooze, canSnooze, applyNow, updateStep, applied, bannerLine, COUNT_MS, SNOOZE_MS } from '../panelupdate.js';

const idle = { staged: false, busy: false };
const staging = { staged: true, busy: false };
const taking = { staged: true, busy: true };

test('an unchanged revision asks for nothing', () => {
  const u = noticeRev(createUpdate(3), 3, 1000);
  assert.deepEqual(updateStep(u, idle, 1000), { step: 'none' });
});

test('a changed revision never applies on the spot: the banner counts down first', () => {
  const u = noticeRev(createUpdate(3), 4, 1000);
  assert.deepEqual(updateStep(u, idle, 1000), { step: 'banner', leftMs: COUNT_MS, totalMs: COUNT_MS, deadline: 1000 + COUNT_MS, wait: '', drops: false });
  assert.equal(updateStep(u, idle, 1000 + COUNT_MS - 1).step, 'banner');
  assert.equal(updateStep(u, idle, 1000 + COUNT_MS).step, 'apply');
});

test('staged routes are never dropped by the countdown alone: the banner waits for a take or Clear', () => {
  const u = noticeRev(createUpdate(3), 4, 1000);
  assert.deepEqual(updateStep(u, staging, 1000 + COUNT_MS), {
    step: 'banner',
    leftMs: 0,
    totalMs: COUNT_MS,
    deadline: 1000 + COUNT_MS,
    wait: 'staged',
    drops: false,
  });
  assert.equal(updateStep(u, staging, 99000).wait, 'staged');
  assert.equal(updateStep(u, idle, 99000).step, 'apply');
});

test('a take in flight is never interrupted', () => {
  const u = noticeRev(createUpdate(3), 4, 1000);
  assert.equal(updateStep(u, { staged: false, busy: true }, 99000).wait, 'busy');
  assert.equal(updateStep(u, taking, 99000).wait, 'busy');
});

test('the countdown counts from the first notice, not from a later poll of a newer revision', () => {
  let u = noticeRev(createUpdate(3), 4, 1000);
  u = noticeRev(u, 5, 8000);
  assert.equal(updateStep(u, idle, 1000 + COUNT_MS).step, 'apply');
});

test('Snooze keeps the banner up and counts 60 seconds down, then applies even over staged routes', () => {
  let u = noticeRev(createUpdate(3), 4, 1000);
  u = snooze(u, 2000);
  assert.deepEqual(updateStep(u, staging, 2000), { step: 'banner', leftMs: SNOOZE_MS, totalMs: SNOOZE_MS, deadline: 2000 + SNOOZE_MS, wait: '', drops: true });
  assert.equal(updateStep(u, idle, 2000).drops, false);
  assert.equal(updateStep(u, staging, 2000 + SNOOZE_MS - 1).step, 'banner');
  assert.equal(updateStep(u, staging, 2000 + SNOOZE_MS).step, 'apply');
  assert.equal(updateStep(u, idle, 2000 + SNOOZE_MS).step, 'apply');
});

test('a page snoozes once: a second Snooze changes nothing and canSnooze goes false', () => {
  let u = noticeRev(createUpdate(3), 4, 1000);
  assert.equal(canSnooze(u), true);
  u = snooze(u, 2000);
  assert.equal(canSnooze(u), false);
  assert.deepEqual(snooze(u, 50000), u);
  assert.equal(updateStep(snooze(u, 50000), idle, 50000).leftMs, SNOOZE_MS - 48000);
  assert.equal(updateStep(u, idle, 2000 + SNOOZE_MS).step, 'apply');
});

test('Apply now stays allowed after a Snooze, and a newer revision noticed during it keeps the same countdown', () => {
  let u = snooze(noticeRev(createUpdate(3), 4, 1000), 2000);
  u = noticeRev(u, 5, 9000);
  assert.equal(updateStep(u, idle, 9000).leftMs, SNOOZE_MS - 7000);
  assert.equal(updateStep(applyNow(u, 9000), staging, 9000).step, 'apply');
});

test('Apply now applies at once, staged routes or not', () => {
  const u = applyNow(noticeRev(createUpdate(3), 4, 1000), 3000);
  assert.equal(updateStep(u, idle, 3000).step, 'apply');
  assert.equal(updateStep(u, staging, 3000).step, 'apply');
});

test('neither Snooze nor Apply now interrupts a take in flight; the copy applies once it answers', () => {
  const u = noticeRev(createUpdate(3), 4, 1000);
  assert.equal(updateStep(applyNow(u, 3000), taking, 3000).wait, 'busy');
  assert.equal(updateStep(snooze(u, 2000), taking, 2000 + SNOOZE_MS).wait, 'busy');
  assert.equal(updateStep(applyNow(u, 3000), staging, 3100).step, 'apply');
});

test('applying records the revision and the next change starts over, with one Snooze back', () => {
  let u = snooze(noticeRev(createUpdate(3), 4, 1000), 2000);
  u = applied(u, 4);
  assert.equal(canSnooze(u), true);
  assert.equal(updateStep(u, idle, 5000).step, 'none');
  u = noticeRev(u, 5, 6000);
  assert.equal(updateStep(u, idle, 6001).step, 'banner');
  assert.equal(updateStep(u, staging, 6000 + COUNT_MS).wait, 'staged');
});

test('a panel with no revision yet (an older server) never updates', () => {
  const u = noticeRev(createUpdate(undefined), undefined, 1000);
  assert.equal(updateStep(u, idle, 1000).step, 'none');
});

test('the banner line says what the refresh waits for, and when staged routes go with it', () => {
  assert.equal(bannerLine({ leftMs: 2100, wait: '', drops: false }), 'Refreshing in 3 s.');
  assert.equal(bannerLine({ leftMs: 42000, wait: '', drops: true }), 'Refreshing in 42 s, dropping the staged routes.');
  assert.match(bannerLine({ leftMs: 0, wait: 'busy' }), /after the take/);
  assert.match(bannerLine({ leftMs: 0, wait: 'staged' }), /take or clear/);
});
