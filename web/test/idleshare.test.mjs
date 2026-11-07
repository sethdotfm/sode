import test from 'node:test';
import assert from 'node:assert/strict';
import { seenActive, shouldShare, createIdleShare } from '../idleshare.js';

test('seenActive: activity in another tab only moves the timer forward', () => {
  assert.equal(seenActive(1000, 5000), 5000);
  assert.equal(seenActive(5000, 1000), 5000);
  assert.equal(seenActive(5000, 'x'), 5000);
  assert.equal(seenActive(5000, undefined), 5000);
});

test('shouldShare: at most one activity message a second', () => {
  assert.equal(shouldShare(0, 999), false);
  assert.equal(shouldShare(0, 1000), true);
  assert.equal(shouldShare(5000, 5500), false);
});

const fakeChannel = () => ({
  sent: [],
  postMessage(m) {
    this.sent.push(m);
  },
  onmessage: null,
});

test('touch posts to the channel at most once a second and moves lastActive', () => {
  let t = 10_000;
  const ch = fakeChannel();
  const idle = createIdleShare(ch, () => t);
  assert.equal(idle.lastActive(), 10_000);
  idle.touch();
  t = 10_500;
  idle.touch();
  assert.equal(idle.lastActive(), 10_500);
  t = 11_100;
  idle.touch();
  assert.deepEqual(ch.sent, [10_000, 11_100]);
});

test('a message from another tab moves lastActive forward and never back', () => {
  const ch = fakeChannel();
  const idle = createIdleShare(ch, () => 1000);
  ch.onmessage({ data: 4000 });
  assert.equal(idle.lastActive(), 4000);
  ch.onmessage({ data: 2000 });
  ch.onmessage({ data: 'junk' });
  assert.equal(idle.lastActive(), 4000);
});

test('without a channel touch only moves lastActive', () => {
  let t = 1;
  const idle = createIdleShare(null, () => t);
  t = 99;
  idle.touch();
  assert.equal(idle.lastActive(), 99);
});
