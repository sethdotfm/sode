// When an open panel applies a changed revision (panelupdate.js), with no DOM and no clock.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createUpdate, noticeRev, snooze, updateStep, applied, bannerLine, COUNT_MS, SNOOZE_MS } from '../panelupdate.js';

const idle = { staged: false, busy: false };
const staging = { staged: true, busy: false };

test('an unchanged revision asks for nothing', () => {
  const u = noticeRev(createUpdate(3), 3, 1000);
  assert.deepEqual(updateStep(u, idle, 1000), { step: 'none' });
});

test('a changed revision never applies on the spot: the banner counts down first', () => {
  const u = noticeRev(createUpdate(3), 4, 1000);
  assert.deepEqual(updateStep(u, idle, 1000), { step: 'banner', leftMs: COUNT_MS, wait: '' });
  assert.equal(updateStep(u, idle, 1000 + COUNT_MS - 1).step, 'banner');
  assert.equal(updateStep(u, idle, 1000 + COUNT_MS).step, 'apply');
});

test('staged routes are never dropped: the banner waits past the countdown for a take or Clear', () => {
  const u = noticeRev(createUpdate(3), 4, 1000);
  assert.deepEqual(updateStep(u, staging, 1000 + COUNT_MS), { step: 'banner', leftMs: 0, wait: 'staged' });
  assert.deepEqual(updateStep(u, staging, 99000), { step: 'banner', leftMs: 0, wait: 'staged' });
  assert.equal(updateStep(u, idle, 99000).step, 'apply');
});

test('a take in flight is never interrupted', () => {
  const u = noticeRev(createUpdate(3), 4, 1000);
  assert.equal(updateStep(u, { staged: false, busy: true }, 99000).wait, 'busy');
  assert.equal(updateStep(u, { staged: true, busy: true }, 99000).wait, 'busy');
});

test('the countdown counts from the first notice, not from a later poll of a newer revision', () => {
  let u = noticeRev(createUpdate(3), 4, 1000);
  u = noticeRev(u, 5, 8000);
  assert.equal(updateStep(u, idle, 1000 + COUNT_MS).step, 'apply');
});

test('snooze hides the banner for 60 seconds, then the countdown starts again', () => {
  let u = noticeRev(createUpdate(3), 4, 1000);
  u = snooze(u, 2000);
  assert.deepEqual(updateStep(u, idle, 2000 + SNOOZE_MS - 1), { step: 'snoozed' });
  assert.deepEqual(updateStep(u, idle, 2000 + SNOOZE_MS), { step: 'banner', leftMs: COUNT_MS, wait: '' });
  assert.equal(updateStep(u, idle, 2000 + SNOOZE_MS + COUNT_MS).step, 'apply');
});

test('applying records the revision and the next change starts over', () => {
  let u = noticeRev(createUpdate(3), 4, 1000);
  u = applied(u, 4);
  assert.equal(updateStep(u, idle, 5000).step, 'none');
  u = noticeRev(u, 5, 6000);
  assert.equal(updateStep(u, idle, 6001).step, 'banner');
});

test('a panel with no revision yet (an older server) never updates', () => {
  const u = noticeRev(createUpdate(undefined), undefined, 1000);
  assert.equal(updateStep(u, idle, 1000).step, 'none');
});

test('the banner line says what the refresh waits for', () => {
  assert.equal(bannerLine({ leftMs: 2100, wait: '' }), 'Refreshing in 3 s.');
  assert.match(bannerLine({ leftMs: 0, wait: 'busy' }), /after the take/);
  assert.match(bannerLine({ leftMs: 0, wait: 'staged' }), /take or clear/);
});
