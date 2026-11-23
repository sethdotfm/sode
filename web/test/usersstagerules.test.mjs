import test from 'node:test';
import assert from 'node:assert/strict';
import { listedFields, stageFields, stageLabel, stageShow } from '../usersstagerules.js';
import { diffRows } from '../savereviewrules.js';

const listed = {
  users: [
    { id: 'u-1', name: 'Ana', len: 4, role: 'standard', persistent: false, noAutoLock: false, rights: {}, clients: ['Wall'] },
    { id: 'u-2', name: 'Ada', len: 4, role: 'administrator', persistent: true, noAutoLock: false, rights: {} },
  ],
  apiClients: [{ name: 'Desk', scopes: ['names'], rights: {} }],
  guestRights: {},
  maxPinLen: 8,
};

const row = (u, more = {}) => ({
  id: u.id,
  name: u.name,
  pin: '',
  noPin: !u.len,
  hadPin: !!u.len,
  role: u.role,
  persistent: !!u.persistent,
  noAutoLock: !!u.noAutoLock,
  rights: u.rights,
  ...more,
});
const state = (more = {}) => ({
  users: listed.users.map((u) => row(u)),
  clients: listed.apiClients.map((c) => ({ was: c.name, name: c.name, scopes: c.scopes, rights: c.rights })),
  guestRights: {},
  maxPinLen: 8,
  owners: {},
  ...more,
});
const keys = (more) => diffRows(listedFields(listed), stageFields(listed, state(more))).map((r) => r.key);

test('the dialog as loaded is no change', () => {
  assert.deepEqual(keys({}), []);
});

test('each changed attribute of a user is its own key', () => {
  const users = [row(listed.users[0], { name: 'Anna', role: 'trusted' }), row(listed.users[1])];
  assert.deepEqual(keys({ users }), ['user/u-1/name', 'user/u-1/role']);
});

test('a typed PIN sets it; No PIN on a user with a PIN removes it', () => {
  const users = [row(listed.users[0], { pin: '1357' }), row(listed.users[1], { noPin: true })];
  const now = stageFields(listed, state({ users }));
  assert.equal(now['user/u-1/pin'], '1357');
  assert.equal(now['user/u-2/pin'], '');
});

test('a new row is one add key; a removed user names the new owner of their panels', () => {
  const users = [
    row(listed.users[1]),
    { id: '', rid: 7, name: 'Cy', pin: '2468', noPin: false, hadPin: false, role: 'standard', persistent: true, noAutoLock: false, rights: {} },
  ];
  const now = stageFields(listed, state({ users, owners: { 'u-1': 'u-2' } }));
  assert.deepEqual(now['user/new:7/add'], { name: 'Cy', pin: '2468', role: 'standard', persistent: true, noAutoLock: false, rights: {} });
  assert.deepEqual(now['user/u-1/remove'], { owner: 'u-2' });
  assert.deepEqual(keys({ users, owners: { 'u-1': 'u-2' } }), ['user/u-1/remove', 'user/new:7/add']);
});

test('API clients: scopes change, a rename is a remove and an add, a missing one is removed', () => {
  assert.deepEqual(keys({ clients: [{ was: 'Desk', name: 'Desk', scopes: ['names', 'tally'], rights: {} }] }), ['client/Desk/scopes']);
  assert.deepEqual(keys({ clients: [{ was: 'Desk', name: 'Wall 2', scopes: ['names'], rights: {} }] }), ['client/Desk/remove', 'client/Wall 2/add']);
  assert.deepEqual(keys({ clients: [] }), ['client/Desk/remove']);
});

test('the review names a removed user with the API clients that go with them', () => {
  const names = { 'u-1': 'Ana' };
  const r = { key: 'user/u-1/remove', sub: null, was: false, now: { owner: '' } };
  assert.equal(stageLabel(r, names), 'Ana: Remove');
  assert.equal(stageShow(r, r.now, listed), 'Removed, with API client Wall');
  assert.equal(stageShow(r, r.was, listed), 'Kept');
});

test('labels and values of other keys', () => {
  const names = { 'u-1': 'Ana' };
  assert.equal(stageLabel({ key: 'user/u-1/persistent' }, names), 'Ana: Never sign out');
  assert.equal(stageLabel({ key: 'user/new:3/add' }, names, { name: 'Cy' }), 'New user Cy');
  assert.equal(stageLabel({ key: 'client/Desk/scopes' }, names), 'API client Desk: Scopes');
  assert.equal(stageShow({ key: 'user/u-1/pin' }, '1357'), 'New PIN');
  assert.equal(stageShow({ key: 'user/u-1/pin' }, ''), 'No PIN');
  assert.equal(stageShow({ key: 'user/u-1/rights' }, {}), 'All');
  assert.equal(stageShow({ key: 'user/u-1/rights' }, { sources: [{ n: 1 }, { n: 2 }], levels: [] }), '2 sources, no levels');
  assert.equal(stageShow({ key: 'user/u-1/role' }, 'trusted'), 'Trusted user');
  assert.equal(stageShow({ key: 'guestRights' }, { sources: [], tieLines: false }), 'no sources, no tie lines');
});
