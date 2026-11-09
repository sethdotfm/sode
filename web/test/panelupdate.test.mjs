// When an open panel applies a changed revision (panelupdate.js), with no DOM and no clock.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createUpdate, noticeRev, snooze, updateStep, applied, FORCE_MS, SNOOZE_MS } from '../panelupdate.js';

const idle = { viewed: false, busy: false };
const viewing = { viewed: true, busy: false };

test('an unchanged revision asks for nothing', () => {
  const u = noticeRev(createUpdate(3), 3, 1000);
  assert.deepEqual(updateStep(u, idle, 1000), { step: 'none' });
});

test('a changed revision applies at once when no destination is viewed', () => {
  const u = noticeRev(createUpdate(3), 4, 1000);
  assert.equal(updateStep(u, idle, 1000).step, 'apply');
});

test('a viewed destination holds the update and shows the banner until the 10 seconds pass', () => {
  const u = noticeRev(createUpdate(3), 4, 1000);
  const wait = updateStep(u, viewing, 1000 + FORCE_MS - 1);
  assert.equal(wait.step, 'banner');
  assert.equal(wait.leftMs, 1);
  assert.equal(updateStep(u, viewing, 1000 + FORCE_MS).step, 'apply');
});

test('the 10 seconds count from the first notice, not from a later poll of a newer revision', () => {
  let u = noticeRev(createUpdate(3), 4, 1000);
  u = noticeRev(u, 5, 8000);
  assert.equal(updateStep(u, viewing, 1000 + FORCE_MS).step, 'apply');
});

test('a take in flight is never interrupted, even past the 10 seconds', () => {
  const u = noticeRev(createUpdate(3), 4, 1000);
  assert.equal(updateStep(u, { viewed: true, busy: true }, 99000).step, 'banner');
  assert.equal(updateStep(u, { viewed: false, busy: true }, 99000).step, 'banner');
});

test('snooze holds the update for 60 seconds even when idle, then 10 more if a destination is viewed', () => {
  let u = noticeRev(createUpdate(3), 4, 1000);
  u = snooze(u, 2000);
  assert.deepEqual(updateStep(u, idle, 2000 + SNOOZE_MS - 1), { step: 'snoozed' });
  assert.equal(updateStep(u, idle, 2000 + SNOOZE_MS).step, 'apply');
  assert.equal(updateStep(u, viewing, 2000 + SNOOZE_MS).step, 'banner');
  assert.equal(updateStep(u, viewing, 2000 + SNOOZE_MS + FORCE_MS).step, 'apply');
});

test('applying records the revision and the next change starts over', () => {
  let u = noticeRev(createUpdate(3), 4, 1000);
  u = applied(u, 4);
  assert.equal(updateStep(u, idle, 5000).step, 'none');
  u = noticeRev(u, 5, 6000);
  assert.equal(updateStep(u, viewing, 6001).step, 'banner');
});

test('a panel with no revision yet (an older server) never updates', () => {
  const u = noticeRev(createUpdate(undefined), undefined, 1000);
  assert.equal(updateStep(u, idle, 1000).step, 'none');
});
