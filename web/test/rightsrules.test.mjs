import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  allowed,
  planOutside,
  listText,
  parseList,
  parseNames,
  draftOf,
  rightsOf,
  entryKeys,
  pickValid,
  rightsTab,
  accessReq,
  newEntry,
} from '../rightsrules.js';

test('allowed: a missing list means all, an empty one means none', () => {
  assert.equal(allowed(null, 'dsts', 4), true);
  assert.equal(allowed({}, 'dsts', 4), true);
  assert.equal(allowed({ dsts: [] }, 'dsts', 4), false);
  assert.equal(allowed({ dsts: [4] }, 'dsts', 4), true);
  assert.equal(allowed({ dsts: [4] }, 'dsts', 5), false);
  assert.equal(allowed({ sources: [1] }, 'dsts', 5), true);
});

test('listText and parseList turn router numbers into shown numbers and back', () => {
  assert.equal(listText([0, 1, 2, 6], 1), '1-3, 7');
  assert.equal(listText([], 1), '');
  assert.deepEqual(parseList('1-3, 7', 1), { list: [0, 1, 2, 6] });
  assert.deepEqual(parseList('3 1 1', 0), { list: [1, 3] });
  assert.ok(parseList('abc', 0).error);
  assert.ok(parseList('5-2', 0).error);
  assert.ok(parseList('0', 1).error, 'below the first shown number');
});

test('parseNames takes one preset name per line', () => {
  assert.deepEqual(parseNames(' Wall, A \n\nNews\nNews '), ['Wall, A', 'News']);
});

test('the editor keeps All and Only these apart, so an empty list is not All', () => {
  const none = draftOf({ dsts: [] }, 1);
  assert.equal(none.dsts.all, false);
  assert.deepEqual(rightsOf(none, 1, 4), { rights: { dsts: [] } });
  const all = draftOf({}, 1);
  assert.equal(all.dsts.all, true);
  assert.deepEqual(rightsOf(all, 1, 4), { rights: {} });
});

test('rightsOf builds the wire shape and reports the first bad list', () => {
  const d = draftOf({ sources: [0, 1], levels: [1, 2], presets: ['Wall'], visibility: 'hidden' }, 1);
  assert.deepEqual(rightsOf(d, 1, 4), { rights: { sources: [{ n: 0 }, { n: 1 }], levels: [1, 2], presets: ['Wall'], visibility: 'hidden' } });
  d.levels.text = '1, 9';
  assert.match(rightsOf(d, 1, 4).error, /level/i);
  d.levels.text = '1';
  d.sources.text = 'x';
  assert.match(rightsOf(d, 1, 4).error, /source/i);
});

test('planOutside names the first route the rights refuse, with shown numbers', () => {
  const g = (src, levels) => ({ src: { n: src }, levels });
  const plan = { verb: 'patch', items: [{ dst: { n: 9 }, groups: [g(3, [1])] }] };
  assert.equal(planOutside(null, plan, 1), '');
  assert.equal(planOutside({ sources: [3], dsts: [9], levels: [1] }, plan, 1), '');
  assert.equal(planOutside({ dsts: [8] }, plan, 1), 'destination 10 is outside your rights');
  assert.equal(planOutside({ sources: [2] }, plan, 1), 'source 4 is outside your rights');
  assert.equal(planOutside({ levels: [2] }, plan, 1), 'level 1 is outside your rights');
});

test('planOutside skips the Before source of an undo and checks a preset by id', () => {
  const undo = { verb: 'undo', items: [{ dst: { n: 9 }, undo: { id: 1 }, groups: [{ src: { n: 3 }, levels: [1] }] }] };
  assert.equal(planOutside({ sources: [] }, undo, 0), '');
  assert.equal(planOutside({ dsts: [] }, undo, 0), 'destination 9 is outside your rights');
  const preset = { verb: 'preset', preset: 'Wall', presetId: 'p-wall', items: [] };
  assert.equal(planOutside({ presets: ['p-news'] }, preset, 0), 'preset "Wall" is outside your rights');
  assert.equal(planOutside({ presets: ['p-wall'] }, preset, 0), '');
});

test('Edit user rights jumps to the Rights tab with that row selected, even after a rename', () => {
  const rows = [
    { rid: 1, name: 'Ana' },
    { rid: 2, name: 'Bo' },
  ];
  assert.deepEqual(rightsTab(2), { tab: 'rights', pick: 'user:2' });
  const keys = entryKeys(rows, [{ rid: 7, name: 'PanelsBot' }]);
  assert.deepEqual(keys, ['user:1', 'user:2', 'client:7', 'guest']);
  assert.equal(pickValid(keys, 'user:2'), 'user:2');
  assert.equal(pickValid(keys, 'user:9'), 'user:1', 'a removed row falls back to the first entry');
});

const ent = (rights, extra, dirty = []) => {
  const e = newEntry(rights, 1, extra);
  for (const d of dirty) e.dirty.add(d);
  return e;
};

test('accessReq sends untouched rights as they were loaded, and nothing for a row that never loaded any', () => {
  const entries = new Map([
    ['user:1', ent({ dsts: [] }, { noAutoLock: true }, ['noAutoLock'])],
    ['client:5', ent({ sources: [2] })],
    ['guest', ent({ levels: [1] })],
  ]);
  const r = accessReq({
    entries,
    users: [
      { rid: 1, name: 'Ana' },
      { rid: 2, name: 'New' },
    ],
    clients: [
      { rid: 5, name: 'PanelsBot', scopes: ['take'] },
      { rid: 6, name: 'Fresh', scopes: [] },
    ],
    off: 1,
    levels: 4,
  });
  assert.deepEqual(r.users, [{ rights: { dsts: [] }, noAutoLock: true }, {}]);
  assert.deepEqual(r.apiClients, [
    { name: 'PanelsBot', scopes: ['take'], rights: { sources: [2] } },
    { name: 'Fresh', scopes: [], rights: {} },
  ]);
  assert.deepEqual(r.guestRights, { levels: [1] });
  assert.equal('guestScopes' in r, false, 'keyless scopes save with the API settings');
});

test('accessReq sends No auto-lock only once edited', () => {
  const entries = new Map([['user:1', ent({}, { noAutoLock: true })]]);
  const r = accessReq({ entries, users: [{ rid: 1, name: 'Ana' }], clients: [], off: 1, levels: 4 });
  assert.deepEqual(r.users, [{ rights: {} }]);
});

test('accessReq reads an edited draft, and names the user whose rights are wrong', () => {
  const edited = ent({}, {}, ['rights']);
  edited.draft.dsts = { all: false, text: '3, 5-6' };
  const entries = new Map([['user:1', edited]]);
  const ok = accessReq({ entries, users: [{ rid: 1, name: 'Ana' }], clients: [], off: 1, levels: 4 });
  assert.deepEqual(ok.users, [{ rights: { dsts: [{ n: 2 }, { n: 4 }, { n: 5 }] } }]);
  assert.equal('guestRights' in ok, false, 'no guest entry: the server keeps the saved one');
  edited.draft.dsts.text = 'x';
  assert.match(accessReq({ entries, users: [{ rid: 1, name: 'Ana' }], clients: [], off: 1, levels: 4 }).error, /^Ana: Destinations/);
});

test('the Protect right: unset stays unset, Yes and No go back as true and false', () => {
  assert.equal(draftOf({}, 1).protect, '');
  assert.deepEqual(rightsOf(draftOf({ protect: true }, 1), 1, 4), { rights: { protect: true } });
  assert.deepEqual(rightsOf(draftOf({ protect: false }, 1), 1, 4), { rights: { protect: false } });
  assert.deepEqual(rightsOf(draftOf({}, 1), 1, 4), { rights: {} });
});
