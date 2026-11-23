import { test } from 'node:test';
import assert from 'node:assert/strict';
import { oneEdit, wordsMatch, searchWordHit, moreMatch, SEARCH_WORDS } from '../palettematch.js';
import { rankRows, countsAsUse, dropPick, dropUse, addUse, paletteView } from '../paletterules.js';
import { catalogue, rowByKey } from '../palettecatalogue.js';
import { hiddenCounts, hiddenLine, numberHint, footKeys, shiftEnterDoes } from '../palettehints.js';
import { keyLabel } from '../keys.js';

const users = [
  { name: 'Ana', len: 4 },
  { name: 'Bo', len: 4 },
];
const who = (more = {}) => ({ role: 'administrator', user: 'Ana', rights: null, locks: {}, users, ...more });
const guest = (more = {}) => who({ role: '', user: '', ...more });
const SETTINGS = [
  { section: 'this-browser', sectionTitle: 'This browser', topic: 'Preferences', id: 'cfg-theme', label: 'Theme' },
  { section: 'confirm', sectionTitle: 'Confirm', topic: 'Look and feel', id: 'cfg-dismiss', label: 'Success banner closes after (s)' },
  { section: 'router-matrix', sectionTitle: 'Router & matrix', topic: 'Routers', id: 'cfg-router', label: 'Address' },
  { section: 'router-matrix', sectionTitle: 'Router & matrix', topic: 'Routers', id: 'cfg-routername', label: 'Name' },
  { section: 'users', sectionTitle: 'Users', topic: 'Access', id: 'users-manage', label: 'Manage users', button: 'users-manage' },
  { section: 'history', sectionTitle: 'History', topic: 'Look and feel', id: 'clear-history', label: 'Clear History', button: 'clear-history' },
];
const src = (more = {}) => ({
  page: 'main',
  presets: [{ scope: 'global', id: 'p1', name: 'Studio A' }],
  filterSets: [{ scope: 'all', id: 'All', name: 'All' }],
  panels: [],
  settings: SETTINGS,
  who: who(),
  ...more,
});
const top = (rows, q) => rankRows(rows, q)[0]?.key;

test('oneEdit: one change, one more or less letter, or a swap', () => {
  assert.ok(oneEdit('sigm', 'sign'));
  assert.ok(oneEdit('lcok', 'lock'));
  assert.ok(oneEdit('adress', 'address'));
  assert.ok(oneEdit('locks', 'lock'));
  assert.ok(!oneEdit('lcko', 'lock'));
  assert.ok(!oneEdit('abc', 'abcde'));
});

test('typo: one edit for a word of 4 letters or more, after exact and prefix matches', () => {
  const rows = catalogue(src({ who: guest() }));
  assert.equal(top(rows, 'sigm in'), 'action:signIn');
  assert.equal(top(rows, 'lcok'), 'action:lock');
  assert.equal(top(rows, 'undi'), 'action:undo');
  assert.equal(wordsMatch(rowByKey(rows, 'action:lock'), 'lok'), null, 'a 3-letter word gets no typo');
  assert.deepEqual(wordsMatch(rowByKey(rows, 'action:lock'), 'lcok'), { typo: true });
  // "lock" itself: the Lock row first, a typo match ("Look") never before it.
  const look = { key: 'x', cat: 'settings', label: 'Look', hint: '', words: [], access: { ok: true } };
  assert.deepEqual(wordsMatch(look, 'lock'), { typo: true });
  const r = rankRows([look, ...rows], 'lock');
  assert.equal(r[0].key, 'action:lock');
  assert.equal(r.at(-1).key, 'x', 'the typo match comes last');
});

test('across name and path: each query word in the name, the path or the words, one in the name', () => {
  const rows = catalogue(src());
  const addr = rowByKey(rows, 'setting:router-matrix/cfg-router');
  assert.deepEqual(wordsMatch(addr, 'router address'), { typo: false });
  assert.deepEqual(wordsMatch(addr, 'matrix addr'), { typo: false });
  assert.equal(wordsMatch(addr, 'router matrix'), null, 'the path alone does not list a whole section');
  assert.equal(top(rows, 'router address'), 'setting:router-matrix/cfg-router');
  assert.equal(top(rows, 'routers adress'), 'setting:router-matrix/cfg-router', 'a typo works across the path too');
});

test('search words: one table, a whole word or the start of one (3 letters or more)', () => {
  const rows = catalogue(src());
  assert.equal(top(rows, 'dismiss'), 'setting:confirm/cfg-dismiss');
  assert.equal(top(rows, 'toast'), 'setting:confirm/cfg-dismiss');
  assert.equal(top(rows, 'notification'), 'setting:confirm/cfg-dismiss');
  assert.equal(top(rows, 'add user'), 'setting:users/users-manage', 'the Users editor');
  assert.equal(top(rows, 'manage users'), 'setting:users/users-manage');
  assert.equal(top(rows, '?'), 'action:shortcuts');
  assert.equal(top(rows, 'undo'), 'action:undo');
  assert.equal(top(rows, 'ip'), 'setting:router-matrix/cfg-router');
  assert.ok(searchWordHit(rowByKey(rows, 'setting:confirm/cfg-dismiss'), 'dism'));
  assert.ok(!searchWordHit(rowByKey(rows, 'setting:confirm/cfg-dismiss'), 'di'), 'two letters must be the whole word');
  // Signed in, "login" is Sign out or Step in; signed out it is Sign in, and "unlock" too.
  const keys = rankRows(rows, 'login').map((r) => r.key);
  assert.ok(keys.includes('action:signOut'), keys.join());
  const out = catalogue(src({ who: guest() }));
  assert.equal(top(out, 'login'), 'action:signIn');
  assert.equal(top(out, 'unlock'), 'action:signIn');
  assert.equal(moreMatch(rowByKey(rows, 'action:lock'), 'zzzz'), null);
});

test('search words: each key names a row that the catalogue can make', () => {
  const rows = [...catalogue(src()), ...catalogue(src({ who: guest() }))];
  for (const e of SEARCH_WORDS)
    for (const k of e.keys) {
      if (k === 'action:stepIn') continue; // only while Step in is offered
      assert.ok(
        rows.some((r) => (k.endsWith('/') ? r.key.startsWith(k) : r.key === k)),
        k,
      );
    }
});

test('hidden matches: a Guest gets a count, and Enter signs in', () => {
  const rows = catalogue(src({ who: guest() }));
  const view = paletteView(rows, { query: 'users' });
  assert.ok(!view.some((it) => it.row?.key === 'setting:users/users-manage'), 'the grey row stays hidden');
  const c = hiddenCounts(rows, 'users');
  assert.deepEqual(c, { settings: 1 });
  assert.deepEqual(hiddenLine(c, { canSignIn: true }), { act: 'signIn', label: '1 setting needs sign-in. Enter: Sign in' });
  // Panels: no names come from the server, so they count when the query names panels.
  assert.deepEqual(hiddenCounts(rows, 'new panel', { panels: 3 }), { panels: 3 });
  assert.deepEqual(hiddenCounts(rows, 'studio', { panels: 3 }), {});
  const all = hiddenCounts(rows, 's', { panels: 3 });
  assert.ok(all.settings > 1, 'a category word counts the whole category');
  assert.equal(hiddenLine({ panels: 3, settings: 67 }, { canSignIn: true }).label, '3 panels and 67 settings need sign-in. Enter: Sign in');
  assert.deepEqual(hiddenLine({ settings: 2 }), { act: '', label: '2 settings need sign-in.' });
  assert.deepEqual(hiddenCounts(rows, ''), {});
  assert.equal(hiddenLine({}), null);
});

test('hidden matches: a signed-in user is told the Rights or Role, with Step in when offered', () => {
  const rows = catalogue(src({ who: who({ role: 'standard' }) }));
  const c = hiddenCounts(rows, 'router address');
  assert.deepEqual(c, { settings: 1 });
  assert.deepEqual(hiddenLine(c, { signedIn: true }), { act: '', label: '1 setting is outside your Rights or Role.' });
  assert.equal(hiddenLine(c, { signedIn: true, canStepIn: true }).act, 'stepIn');
  const pins = [rowByKey(rows, 'setting:router-matrix/cfg-router')];
  assert.deepEqual(hiddenCounts(rows, 'router address', { pins }), {}, 'a Palette pin shows, so it is not hidden');
});

test('number hint: digits only', () => {
  assert.match(numberHint('12', 'main'), /press Esc and type the number in Source/);
  assert.match(numberHint(' 7 ', 'log'), /open Home/);
  assert.equal(numberHint('12a', 'main'), '');
  assert.equal(numberHint('', 'main'), '');
});

test('key hint: only the keys that apply to the row and the level', () => {
  const rows = catalogue(src());
  const keys = (it, o) => footKeys(it, o).map(([k, w]) => `${k} ${w}`);
  const preset = { type: 'row', row: rowByKey(rows, 'preset:global:p1') };
  const theme = { type: 'row', row: rowByKey(rows, 'action:theme') };
  assert.ok(keys(preset).includes('⇧Enter fill the Patch boxes'));
  assert.ok(!keys(theme).some((k) => k.startsWith('⇧Enter')));
  assert.equal(shiftEnterDoes(rowByKey(rows, 'filterSet:all:All')), '');
  assert.ok(keys(theme).includes('Esc close'), 'the top with no text closes');
  assert.ok(keys(theme, { query: 'th' }).includes('Esc clear'));
  assert.ok(keys({ type: 'cat', id: 'settings' }, { path: ['settings'] }).includes('Esc back'));
  assert.ok(keys({ type: 'hidden', act: 'signIn' }).includes('Enter sign in'));
  assert.ok(keys(preset).some((k) => k.endsWith(' bookmark')));
  assert.ok(keys(preset).some((k) => k.endsWith(' pin')));
  assert.ok(!keys(preset, { pick: true }).some((k) => k.endsWith(' bookmark') || k.endsWith(' pin')));
  assert.ok(keys(preset, { pinned: true, bookmarked: true }).some((k) => k.endsWith(' unpin')));
  assert.ok(keys(preset, { pinned: true, bookmarked: true }).some((k) => k.endsWith(' remove bookmark')));
  const altMove = `${keyLabel('alt+up')}↓ move`;
  assert.ok(!keys(preset).includes(altMove), 'only a row in the Pinned section moves');
  assert.ok(keys({ ...preset, pinned: true }).includes(altMove));
  assert.ok(!keys({ ...preset, pinned: true }, { query: 'st' }).includes(altMove));
  // A grey pin can be unpinned; a Missing one has no Bookmark.
  const gone = { type: 'row', pinned: true, row: { key: 'preset:global:gone', kind: 'missing', missing: true, access: { ok: false } } };
  assert.deepEqual(
    keys(gone, { pinned: true }).filter((k) => !k.startsWith('↑↓') && !k.startsWith('Esc')),
    ['Enter why', `${keyLabel('alt+p')} unpin`, `${keyLabel('alt+up')}↓ move`],
  );
});

test('uses: a clear or reset row is no use, and a first pick left at once goes', () => {
  assert.equal(countsAsUse('setting:history/clear-history'), false);
  assert.equal(countsAsUse('action:theme'), true);
  assert.deepEqual(dropPick(['a', 'b'], 'a'), ['b']);
  const u = addUse(addUse({}, 'a', 1), 'b', 2);
  assert.deepEqual(Object.keys(dropUse(u, 'a')), ['b']);
});
