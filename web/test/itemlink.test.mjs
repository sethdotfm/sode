import test from 'node:test';
import assert from 'node:assert/strict';
import { presetLink, panelLink, readItemLink, linkInstall, itemLinkStep, OTHER_INSTALL } from '../itemlink.js';

const HERE = '61398d9320ce4125';
const open = { enforced: false, locked: false };
const presets = [
  { scope: 'global', id: 'p1', name: 'Show' },
  { scope: 'user', id: 'p2', name: 'Mine' },
  { scope: 'local', id: 'p3', name: 'Browser' },
];
const ready = { install: HERE, access: open, pageLocked: false, namesIn: true, profileWait: false, presets };

test('a preset Item link reads back to its preset and install, and the rest of the address stays', () => {
  const url = presetLink('https://sode.test/', 'p1', HERE);
  assert.equal(url, `https://sode.test/#preset=p1&install=${HERE}`);
  const r = readItemLink('#keep=1&preset=p1&install=' + HERE);
  assert.deepEqual({ preset: r.preset, install: r.install, hash: r.hash }, { preset: 'p1', install: HERE, hash: '#keep=1' });
  assert.equal(readItemLink('#top'), null);
  assert.equal(readItemLink(''), null);
});

test('a panel Item link adds the install to the panel link, beside any other fragment part', () => {
  assert.equal(panelLink('https://sode.test/panels/studio-a', HERE), `https://sode.test/panels/studio-a#install=${HERE}`);
  assert.equal(panelLink('https://sode.test/panels/x#kiosk=1', HERE), `https://sode.test/panels/x#kiosk=1&install=${HERE}`);
  assert.equal(linkInstall(`#install=${HERE}`), HERE);
  assert.equal(linkInstall('#key=abc'), '');
});

test('a link from another install never opens, and says so', () => {
  const link = readItemLink('#preset=p1&install=0000000000000000');
  assert.deepEqual(itemLinkStep(link, ready), { step: 'other' });
  assert.match(OTHER_INSTALL, /another Sode install/);
  assert.deepEqual(itemLinkStep(readItemLink('#preset=p1'), ready), { step: 'other' });
});

test('a link from this install opens its Everyone or Me preset', () => {
  assert.deepEqual(itemLinkStep(readItemLink(`#preset=p1&install=${HERE}`), ready), { step: 'open', preset: presets[0] });
  assert.deepEqual(itemLinkStep(readItemLink(`#preset=p2&install=${HERE}`), ready), { step: 'open', preset: presets[1] });
});

test('a This browser preset or a preset this user cannot see is missing', () => {
  assert.deepEqual(itemLinkStep(readItemLink(`#preset=p3&install=${HERE}`), ready), { step: 'missing' });
  assert.deepEqual(itemLinkStep(readItemLink(`#preset=gone&install=${HERE}`), ready), { step: 'missing' });
});

test('the link waits for the install, the lock state, the names, an uncovered page and the profile', () => {
  const link = readItemLink(`#preset=p2&install=${HERE}`);
  const wait = { step: 'wait' };
  assert.deepEqual(itemLinkStep(link, { ...ready, install: '' }), wait);
  assert.deepEqual(itemLinkStep(link, { ...ready, access: null }), wait);
  assert.deepEqual(itemLinkStep(link, { ...ready, namesIn: false }), wait);
  assert.deepEqual(itemLinkStep(link, { ...ready, access: { enforced: true, locked: true } }), wait);
  assert.deepEqual(itemLinkStep(link, { ...ready, access: { enforced: false, locked: true, held: true } }), wait);
  assert.deepEqual(itemLinkStep(link, { ...ready, pageLocked: true }), wait);
  // a stale hand-lock marker while a PIN lock is enforced and unlocked does not hold it
  assert.equal(itemLinkStep(link, { ...ready, access: { enforced: true, locked: false }, pageLocked: true }).step, 'open');
  // Me presets come after sign-in: a missing one waits for the profile, a found one does not
  assert.deepEqual(itemLinkStep(readItemLink(`#preset=gone&install=${HERE}`), { ...ready, profileWait: true }), wait);
  assert.equal(itemLinkStep(readItemLink(`#preset=p1&install=${HERE}`), { ...ready, profileWait: true }).step, 'open');
  // another install is told once the page is uncovered, so the message is seen; the profile does not matter
  assert.deepEqual(itemLinkStep(readItemLink('#preset=p1&install=x'), { ...ready, pageLocked: true }), wait);
  assert.deepEqual(itemLinkStep(readItemLink('#preset=p1&install=x'), { ...ready, profileWait: true }), { step: 'other' });
});
