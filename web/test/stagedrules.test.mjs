// Staged Settings edits across Step in and Step out: each keeps its author, and Save skips the ones in sections the
// saver's Role cannot see.
import test from 'node:test';
import assert from 'node:assert/strict';
import { noteAuthors, splitSeen, unseenNotes, sentKeys, namesAuthors } from '../stagedrules.js';
import { sectionView } from '../settingsrolerules.js';

const row = (key, now, sub = null) => ({ key, sub, was: '', now });

test('a new or changed edit belongs to whoever made it; an unchanged one keeps its author', () => {
  let marks = noteAuthors(new Map(), [row('brandText', 'Truck 4')], 'Ana');
  marks = noteAuthors(marks, [row('brandText', 'Truck 4'), row('locks', true, 'main')], 'Bo');
  const by = (r) => marks.get(`${r.key}/${r.sub}`)?.by;
  assert.equal(by(row('brandText')), 'Ana');
  assert.equal(by(row('locks', null, 'main')), 'Bo');

  marks = noteAuthors(marks, [row('brandText', 'Truck 5'), row('locks', true, 'main')], 'Bo');
  assert.equal(by(row('brandText')), 'Bo');
});

test('an edit that went back to the value as loaded is dropped', () => {
  let marks = noteAuthors(new Map(), [row('brandText', 'Truck 4')], 'Ana');
  marks = noteAuthors(marks, [], 'Ana');
  assert.equal(marks.size, 0);
});

test('Save skips edits in sections the saver cannot see and counts them by author', () => {
  // Bo, an Administrator, stepped in and staged Locks and Users edits; Ana, a Trusted user, staged Branding.
  const rows = [row('brandText', 'Truck 4'), row('locks', true, 'main'), row('autoLockMinutes', 5), row('user/u2/role', 'trusted')];
  const marks = noteAuthors(new Map(), rows.slice(0, 1), 'Ana');
  const all = noteAuthors(marks, rows, 'Bo');
  const by = (r) => all.get(`${r.key}/${r.sub}`).by;
  const slug = { brandText: 'branding', locks: 'locks', autoLockMinutes: 'locks', 'user/u2/role': 'users' };
  const seen = (r) => sectionView(slug[r.key], 'trusted') !== 'hidden';

  const { shown, unseen } = splitSeen(rows, seen);
  assert.deepEqual(
    shown.map((r) => r.key),
    ['brandText'],
  );
  assert.deepEqual(sentKeys(rows, seen), ['brandText']);
  assert.deepEqual(unseenNotes(unseen, by), ['3 changes by Bo that your Role cannot see']);
});

test('the count names each author apart, one change in the singular', () => {
  const rows = [row('locks', true, 'main'), row('panels', true)];
  const by = (r) => (r.key === 'locks' ? 'Bo' : 'Cy');
  assert.deepEqual(unseenNotes(rows, by), ['1 change by Bo that your Role cannot see', '1 change by Cy that your Role cannot see']);
});

test('the review names authors only when an edit is not the saver’s own', () => {
  const rows = [row('brandText', 'x'), row('confirmEnter', true)];
  assert.equal(
    namesAuthors(rows, () => 'Ana', 'Ana'),
    false,
  );
  assert.equal(
    namesAuthors(rows, (r) => (r.key === 'brandText' ? 'Ana' : 'Bo'), 'Ana'),
    true,
  );
  assert.equal(
    namesAuthors(rows, () => 'Ana', 'Bo'),
    true,
  );
});
