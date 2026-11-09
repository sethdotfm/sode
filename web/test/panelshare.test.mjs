// What the Share modal's People tab offers and sends. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { actionsFor, shareDraft, pickList, toggleUser, shareBody, shareChanged } from '../panelsrules.js';

const P = (extra = {}) => ({
  id: 'p-1',
  name: 'Wall',
  owner: 'Ana',
  acts: 'session',
  share: { mode: 'me' },
  rows: 4,
  cols: 8,
  cells: [],
  canEdit: true,
  ...extra,
});

test('only a panel the caller may change offers Share', () => {
  assert.deepEqual(actionsFor(P()), ['launch', 'edit', 'share', 'push', 'delete']);
  assert.deepEqual(actionsFor(P({ canEdit: false })), ['launch', 'copy']);
});

test('the draft starts from the stored share and never aliases it', () => {
  const share = { mode: 'named', users: ['Bo'] };
  const d = shareDraft(share);
  d.users.push('Cy');
  assert.deepEqual(share.users, ['Bo']);
  assert.deepEqual(shareDraft(undefined), { mode: 'me', users: [] });
  assert.deepEqual(shareDraft({ mode: 'everyone' }), { mode: 'everyone', users: [] });
});

test('the picker lists everyone but the owner, ticks the named, and keeps a named user who has left the list', () => {
  const people = [{ name: 'Ana' }, { name: 'Bo' }, { name: 'Cy' }];
  assert.deepEqual(pickList(people, 'ana', ['cy']), [
    { name: 'Bo', on: false },
    { name: 'Cy', on: true },
  ]);
  assert.deepEqual(pickList(people, 'Ana', ['Dee']), [
    { name: 'Bo', on: false },
    { name: 'Cy', on: false },
    { name: 'Dee', on: true },
  ]);
});

test('ticking adds a name once and unticking drops it, ignoring case', () => {
  assert.deepEqual(toggleUser(['Bo'], 'Cy', true), ['Bo', 'Cy']);
  assert.deepEqual(toggleUser(['Bo'], 'bo', true), ['Bo']);
  assert.deepEqual(toggleUser(['Bo', 'Cy'], 'BO', false), ['Cy']);
});

test('the save body is the panel with the new share, minus what the server owns', () => {
  const p = P({ clientRights: { visibility: 'hidden' }, art: [{ id: 'a' }], future: 7, slug: 'wall' });
  const b = shareBody(p, { mode: 'named', users: ['Bo'] });
  assert.deepEqual(b.share, { mode: 'named', users: ['Bo'] });
  assert.equal('canEdit' in b, false);
  assert.equal('clientRights' in b, false);
  assert.equal(b.name, 'Wall');
  assert.equal(b.future, 7);
  assert.equal(b.slug, 'wall'); // a PUT without it would clear the short name
  assert.deepEqual(p.share, { mode: 'me' });
});

test('names are sent only for Named users', () => {
  assert.deepEqual(shareBody(P(), { mode: 'everyone', users: ['Bo'] }).share, { mode: 'everyone' });
  assert.deepEqual(shareBody(P(), { mode: 'me', users: ['Bo'] }).share, { mode: 'me' });
});

test('a draft counts as changed only when who may use the panel differs', () => {
  assert.equal(shareChanged({ mode: 'me' }, { mode: 'me', users: [] }), false);
  assert.equal(shareChanged({ mode: 'named', users: ['Bo', 'Cy'] }, { mode: 'named', users: ['cy', 'bo'] }), false);
  assert.equal(shareChanged({ mode: 'named', users: ['Bo'] }, { mode: 'named', users: [] }), true);
  assert.equal(shareChanged({ mode: 'everyone' }, { mode: 'me', users: [] }), true);
  assert.equal(shareChanged({ mode: 'me', users: [] }, { mode: 'named', users: [] }), true);
});
