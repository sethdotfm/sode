import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  BOOKMARKS_MAX,
  bookmarkProblem,
  cleanBookmarks,
  shownList,
  ownScope,
  canEditEveryone,
  addBookmark,
  fromRow,
  fromUrl,
  canNewTab,
  bookmarkView,
  paletteMarks,
  renameKey,
  moveBookmark,
} from '../bookmarkrules.js';

// The Go tests read the same cases (internal/config/bookmarks_test.go).
const cases = JSON.parse(readFileSync(new URL('./bookmark_cases.json', import.meta.url), 'utf8')).cases;

test('bookmarkProblem: the shared cases, with the server’s words', () => {
  for (const c of cases) assert.equal(bookmarkProblem(c.b), c.problem, JSON.stringify(c.b));
});

test('shownList: This browser, else Me, else Everyone; never merged', () => {
  const g = [{ key: 'page:log', label: 'Log' }];
  const u = [{ key: 'page:panels', label: 'Panels' }];
  const l = [{ url: 'https://x.test/', label: 'X' }];
  assert.deepEqual(shownList({ local: l, user: u, global: g, signedIn: true }), { scope: 'local', list: l });
  assert.deepEqual(shownList({ local: [], user: u, global: g, signedIn: true }), { scope: 'user', list: u });
  assert.deepEqual(shownList({ local: [], user: [], global: g, signedIn: true }), { scope: 'global', list: g });
  // A Guest has no Me list, whatever a stale profile holds.
  assert.deepEqual(shownList({ local: [], user: u, global: g, signedIn: false }), { scope: 'global', list: g });
  assert.deepEqual(shownList({}), { scope: 'global', list: [] });
});

test('ownScope and canEditEveryone', () => {
  assert.equal(ownScope(true), 'user');
  assert.equal(ownScope(false), 'local');
  assert.equal(canEditEveryone({ role: 'standard' }), false);
  assert.equal(canEditEveryone({ role: '' }), false);
  assert.equal(canEditEveryone({ role: 'trusted' }), true);
  assert.equal(canEditEveryone({ role: 'administrator' }), true);
  assert.equal(canEditEveryone({ role: '', noUsers: true }), true);
});

test('addBookmark: the cap, the same target and unsafe addresses', () => {
  let list = [];
  for (let i = 0; i < BOOKMARKS_MAX; i++) list = addBookmark(list, { key: `preset:global:p${i}`, label: `P${i}` }).list;
  assert.equal(list.length, 30);
  assert.match(addBookmark(list, { key: 'page:log', label: 'Log' }).err, /at most 30/);
  assert.match(addBookmark([{ key: 'page:log', label: 'Log' }], { key: 'page:log', label: 'Log' }).err, /already/);
  assert.equal(addBookmark([], fromUrl('javascript:alert(1)', 'X')).err, 'use a web address that starts with http:// or https://');
  assert.equal(addBookmark([], fromUrl('https://sode.local/panels/a#key=1', 'A')).err, 'a bookmark cannot hold a panel key');
});

test('fromRow and fromUrl: the defaults', () => {
  assert.deepEqual(fromRow({ key: 'page:log', label: 'Communication Log' }), { key: 'page:log', label: 'Communication Log', newTab: false, inPalette: true });
  assert.deepEqual(fromUrl('https://example.com/run'), { url: 'https://example.com/run', label: 'example.com', newTab: true, inPalette: true });
  assert.equal(fromUrl('https://example.com/', 'Rundown').label, 'Rundown');
});

test('cleanBookmarks drops bad entries and keeps 30', () => {
  const raw = [{ key: 'page:log', label: 'Log' }, { url: 'javascript:x', label: 'Bad' }, 'junk', { key: 'page:panels', label: 'P', inPalette: false }];
  assert.deepEqual(cleanBookmarks(raw), [
    { key: 'page:log', label: 'Log', newTab: false, inPalette: true },
    { key: 'page:panels', label: 'P', newTab: false, inPalette: false },
  ]);
  assert.deepEqual(cleanBookmarks('x'), []);
  assert.equal(cleanBookmarks(Array.from({ length: 40 }, (_, i) => ({ key: `page:p${i}`, label: 'X' }))).length, 30);
});

const OK = { ok: true, lock: false, why: '', offer: [] };
const rows = [
  { key: 'preset:global:a1', kind: 'preset', label: 'Morning', access: OK },
  { key: 'page:log', kind: 'page', label: 'Communication Log', access: { ...OK, lock: true } },
  { key: 'setting:locks/cfg-lock', kind: 'setting', label: 'Locks', access: { ok: false, lock: false, why: 'Outside your Role.', offer: ['approval'] } },
  { key: 'page:settings', kind: 'page', label: 'Settings', home: true, access: OK },
];

test('bookmarkView: ok, Missing, No access and Sign in', () => {
  assert.equal(bookmarkView({ key: 'preset:global:a1', label: 'Old name' }, rows).label, 'Morning');
  assert.equal(bookmarkView({ key: 'preset:global:gone', label: 'Gone' }, rows).state, 'missing');
  assert.equal(bookmarkView({ key: 'preset:global:gone', label: 'Gone' }, rows).label, 'Gone');
  assert.equal(bookmarkView({ key: 'setting:locks/cfg-lock', label: 'Locks' }, rows).state, 'noAccess');
  assert.equal(bookmarkView({ key: 'page:log', label: 'Log' }, rows).state, 'signIn');
  assert.equal(bookmarkView({ url: 'https://x.test/', label: 'X' }, rows).state, 'ok');
  // Before the catalogue is read, nothing is Missing yet.
  assert.equal(bookmarkView({ key: 'preset:global:gone', label: 'Gone' }, null).state, 'ok');
});

test('canNewTab: pages, panels and web addresses only', () => {
  assert.equal(canNewTab({ url: 'https://x.test/' }, null), true);
  assert.equal(canNewTab({ key: 'page:log' }, rows[1]), true);
  assert.equal(canNewTab({ key: 'page:settings' }, rows[3]), false);
  assert.equal(canNewTab({ key: 'preset:global:a1' }, rows[0]), false);
});

test('canNewTab: a page that runs on Home has no new tab, whatever its key', () => {
  const page = { key: 'page:other', kind: 'page', label: 'Other', home: true, access: OK };
  assert.equal(canNewTab({ key: page.key }, page), false);
  assert.equal(canNewTab({ key: page.key }, { ...page, home: false }), true);
});

test('paletteMarks: ticked bookmarks as rows, in list order, Missing ones left out', () => {
  const list = [
    { url: 'https://x.test/', label: 'X', newTab: true, inPalette: true },
    { key: 'preset:global:a1', label: 'Morning', inPalette: true },
    { key: 'page:log', label: 'Log', inPalette: false },
    { key: 'preset:global:gone', label: 'Gone', inPalette: true },
  ];
  const m = paletteMarks(list, rows);
  assert.deepEqual(
    m.map((r) => r.key),
    ['url:https://x.test/', 'preset:global:a1'],
  );
  assert.equal(m[0].kind, 'url');
  assert.deepEqual(m[0].target, { url: 'https://x.test/', newTab: true });
});

test('renameKey: only the renamed key moves', () => {
  const list = [
    { key: 'filterSet:local:Cams', label: 'Cams' },
    { key: 'filterSet:global:Cams', label: 'Cams' },
  ];
  assert.deepEqual(renameKey(list, 'filterSet:local:Cams', 'filterSet:local:Cameras', 'Cameras'), [
    { key: 'filterSet:local:Cameras', label: 'Cameras' },
    { key: 'filterSet:global:Cams', label: 'Cams' },
  ]);
});

test('moveBookmark: by one place or before another row; null when nothing moves', () => {
  const l = ['a', 'b', 'c'].map((key) => ({ key }));
  const keys = (x) => x.map((b) => b.key).join('');
  assert.equal(keys(moveBookmark(l, 0, { by: 1 })), 'bac');
  assert.equal(keys(moveBookmark(l, 2, { before: 0 })), 'cab');
  assert.equal(keys(moveBookmark(l, 0, { before: 3 })), 'bca');
  assert.equal(moveBookmark(l, 0, { by: -1 }), null);
  assert.equal(moveBookmark(l, 0, { before: 1 }), null);
  assert.equal(moveBookmark(l, 5, { by: 1 }), null);
});
