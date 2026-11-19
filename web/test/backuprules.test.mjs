import test from 'node:test';
import assert from 'node:assert/strict';
import { offersBackup, lastBackupText } from '../backuprules.js';

const pinRow = { key: 'pinWait', critical: true };
const brand = { key: 'brandText', critical: false };
const isCritical = (r) => r.critical;
const admin = 'administrator';
const guestDown = { text: 'Guest role goes down' };

test('a Critical save offers a backup', () => {
  assert.equal(offersBackup({ saves: [brand, pinRow], isCritical, interruptions: [], role: admin }), true);
});

test('a save with Interruptions offers a backup, also when nothing ends', () => {
  assert.equal(offersBackup({ saves: [brand], isCritical, interruptions: [guestDown], role: admin }), true);
});

test('when Sode did not send the Interruptions, the backup is offered', () => {
  assert.equal(offersBackup({ saves: [brand], isCritical, interruptions: null, role: admin }), true);
});

test('a plain save offers no backup', () => {
  assert.equal(offersBackup({ saves: [brand], isCritical, interruptions: [], role: admin }), false);
});

test('a Critical change that stays unsaved offers no backup', () => {
  assert.equal(offersBackup({ saves: [], isCritical, interruptions: [], role: admin }), false);
});

test('only an Administrator can download a backup, so only an Administrator is offered one', () => {
  assert.equal(offersBackup({ saves: [pinRow], isCritical, interruptions: [guestDown], role: 'trusted' }), false);
  assert.equal(offersBackup({ saves: [pinRow], isCritical, interruptions: [guestDown], role: '' }), false);
});

test('Last backup says when, or never', () => {
  const now = Date.parse('2026-11-18T12:00:00Z');
  assert.equal(lastBackupText(null, now), 'Last backup: never in this browser');
  assert.equal(lastBackupText('2026-11-18T09:00:00Z', now), 'Last backup: 3 hours ago in this browser');
  assert.equal(lastBackupText('not a time', now), 'Last backup: never in this browser');
});
