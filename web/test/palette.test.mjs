import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  PALETTE_CHARS,
  PALETTE_OFF,
  PALETTE_VALUES,
  paletteCharProblem,
  paletteCharOf,
  opensPalette,
  namesStartingWith,
  startsNamesText,
  addPick,
  cleanPicks,
  CATEGORIES,
  paletteView,
  rankRows,
  stepIndex,
  firstIndex,
  pathText,
} from '../paletterules.js';
import { catalogue, pageOf, rowByKey, ACTIONS } from '../palettecatalogue.js';
import { PREFS } from '../prefs.js';

test('Palette character: one character that no name, number or patch box uses', () => {
  for (const c of PALETTE_CHARS) assert.equal(paletteCharProblem(c), '', c);
  for (const bad of ['a', 'Z', 'é', '7', '٣', ' ', '\t', '+', ',', '-', '.', '', '>>', '́', '​'])
    assert.notEqual(paletteCharProblem(bad), '', JSON.stringify(bad));
  assert.equal(paletteCharProblem('😀'), '', 'one code point, not one UTF-16 unit');
});

test('Palette character: the pref offers each character, then off', () => {
  assert.deepEqual(Object.keys(PREFS.paletteChar.values), [...PALETTE_CHARS, PALETTE_OFF]);
  assert.equal(PREFS.paletteChar.def, '>');
  assert.equal(PALETTE_VALUES['>'], '>');
  assert.equal(paletteCharOf('>'), '>');
  assert.equal(paletteCharOf(PALETTE_OFF), '');
  assert.equal(paletteCharOf('a'), '');
  assert.equal(paletteCharOf(undefined), '');
});

test('Palette character: Go keeps the same list', () => {
  const go = readFileSync(new URL('../../internal/config/palette.go', import.meta.url), 'utf8');
  assert.match(go, new RegExp(`PaletteChars\\s+= "${PALETTE_CHARS.replace(/[\\^$.*+?()[\]{}|]/g, '\\$&')}"`));
});

test('opensPalette: first in an empty box, no modifier', () => {
  const k = (value, key, mods = {}) => opensPalette({ value, key, ...mods }, '>');
  assert.equal(k('', '>'), true);
  assert.equal(k('A', '>'), false, 'after other text it is plain text');
  assert.equal(k('', '~'), false);
  assert.equal(k('', '>', { ctrlKey: true }), false);
  assert.equal(k('', '>', { altKey: true }), false);
  assert.equal(k('', '>', { metaKey: true }), false);
  assert.equal(opensPalette({ value: '', key: '>' }, ''), false, 'off');
});

test('the warning lists router names that start with the character', () => {
  const srcs = [{ name: '>CAM 1' }, { name: 'CAM 2' }];
  const dsts = [{ name: '>MON' }, { name: '>CAM 1' }];
  const names = namesStartingWith('>', srcs, dsts);
  assert.deepEqual(names, ['>CAM 1', '>MON']);
  assert.match(startsNamesText('>', names), /^2 router names start with >: >CAM 1, >MON\. To find them, type >> first\.$/);
  assert.equal(startsNamesText('>', []), '');
  assert.equal(startsNamesText('', names), '');
  assert.match(startsNamesText('>', ['a', 'b', 'c'], 2), /a, b and 1 more/);
});

test('last picks: newest first, five at most, no repeats', () => {
  let r = [];
  for (const k of ['a', 'b', 'c', 'd', 'e', 'f']) r = addPick(r, k);
  assert.deepEqual(r, ['f', 'e', 'd', 'c', 'b']);
  assert.deepEqual(addPick(r, 'd'), ['d', 'f', 'e', 'c', 'b']);
  assert.deepEqual(cleanPicks(['a', 1, '', 'a', null, 'b']), ['a', 'b']);
  assert.deepEqual(cleanPicks('x'), []);
});

const who = (more = {}) => ({
  role: 'standard',
  user: 'Ana',
  rights: null,
  locks: {},
  users: [
    { name: 'Ana', len: 4 },
    { name: 'Bo', len: 4 },
  ],
  ...more,
});
const src = (more = {}) => ({
  page: 'main',
  presets: [
    { scope: 'global', id: 'p1', name: 'Studio A' },
    { scope: 'local', id: 'p2', name: 'Desk' },
  ],
  filterSets: [
    { scope: 'all', name: 'All' },
    { scope: 'global', name: 'Cameras' },
  ],
  panels: [{ id: 'x1', name: 'Wall', type: 'grid' }],
  settings: [
    { section: 'this-browser', sectionTitle: 'This browser', topic: 'Preferences', id: 'cfg-theme', label: 'Theme' },
    { section: 'branding', sectionTitle: 'Branding', topic: 'Look and feel', id: 'cfg-brand', label: 'Logo text' },
    { section: 'users', sectionTitle: 'Users', topic: 'Access', id: 'users-open', label: 'Manage users', button: true },
  ],
  who: who(),
  ...more,
});

test('pageOf: the page under the app root', () => {
  assert.equal(pageOf(''), 'main');
  assert.equal(pageOf('/'), 'main');
  assert.equal(pageOf('log'), 'log');
  assert.equal(pageOf('panels'), 'panels');
  assert.equal(pageOf('panels/editor'), 'editor');
  assert.equal(pageOf('panels/abc'), 'panel');
});

test('catalogue: every kind, keyed by kind and target', () => {
  const rows = catalogue(src());
  const kinds = new Set(rows.map((r) => r.kind));
  for (const k of ['action', 'preset', 'page', 'panel', 'filterSet', 'setting', 'settingsButton']) assert.ok(kinds.has(k), k);
  assert.equal(rowByKey(rows, 'preset:global:p1').label, 'Studio A');
  assert.equal(rowByKey(rows, 'preset:local:p2').hint, 'This browser');
  assert.equal(rowByKey(rows, 'panel:x1').target.href, 'panels/x1');
  assert.equal(rowByKey(rows, 'filterSet:all:All').label, 'All (no filter set)');
  assert.equal(rowByKey(rows, 'setting:users/users-open').kind, 'settingsButton');
  assert.ok(rowByKey(rows, 'action:reread').home);
  assert.ok(!rowByKey(rows, 'action:lock').home);
  assert.equal(new Set(rows.map((r) => r.key)).size, rows.length, 'keys are unique');
  for (const r of rows)
    assert.ok(
      CATEGORIES.some((c) => c.id === r.cat),
      r.key,
    );
});

test('catalogue: Home on every page but Home', () => {
  assert.equal(rowByKey(catalogue(src()), 'page:home'), null);
  for (const page of ['log', 'panels', 'editor', 'panel']) {
    const rows = catalogue(src({ page }));
    assert.equal(rowByKey(rows, 'page:home').target.href, './', page);
    assert.equal(paletteView(rows)[0].row.key, 'page:home', page);
  }
});

test('catalogue: grey rows outside Rights or Role offer Step in or Approval', () => {
  const rows = catalogue(src({ who: who({ rights: { presets: ['p2'] } }) }));
  const studio = rowByKey(rows, 'preset:global:p1');
  assert.equal(studio.access.ok, false);
  assert.match(studio.access.why, /^Outside your Rights/);
  assert.deepEqual(studio.access.offer, ['stepIn']);
  assert.equal(rowByKey(rows, 'preset:local:p2').access.ok, true);

  assert.equal(rowByKey(rows, 'setting:this-browser/cfg-theme').access.ok, true);
  const brand = rowByKey(rows, 'setting:branding/cfg-brand');
  assert.equal(brand.access.ok, false);
  assert.deepEqual(brand.access.offer, ['approval', 'stepIn']);
  const users = rowByKey(rows, 'setting:users/users-open');
  assert.equal(users.access.ok, false);
  assert.deepEqual(users.access.offer, ['stepIn']);

  const admin = catalogue(src({ who: who({ role: 'administrator' }) }));
  assert.ok(
    admin.every((r) => r.access.ok),
    'an Administrator with all Rights gets no grey row',
  );

  // Step in needs another user with a PIN.
  const alone = catalogue(src({ who: who({ users: [{ name: 'Ana', len: 4 }] }) }));
  assert.deepEqual(rowByKey(alone, 'setting:users/users-open').access.offer, []);
  assert.equal(rowByKey(alone, 'action:stepIn'), null);
});

test('catalogue: a locked page shows a lock for a Guest and is not grey', () => {
  const rows = catalogue(src({ page: 'log', who: who({ user: '', locks: { main: true, panel: true } }) }));
  const home = rowByKey(rows, 'page:home');
  assert.equal(home.access.ok, true);
  assert.equal(home.access.lock, true);
  assert.equal(rowByKey(rows, 'panel:x1').access.lock, true);
  assert.equal(rowByKey(rows, 'page:log').access.lock, false);
  assert.ok(rowByKey(rows, 'action:signIn'), 'a Guest gets Sign in');
  assert.equal(rowByKey(catalogue(src()), 'action:signIn'), null, 'a signed-in user does not');
});

test('catalogue: the phone has no desktop-only rows and only its own Settings', () => {
  const rows = catalogue(src({ who: who({ phone: true }) }));
  for (const a of ACTIONS.filter((x) => x.desktop)) assert.equal(rowByKey(rows, `action:${a.id}`), null, a.id);
  assert.equal(rowByKey(rows, 'page:editor'), null);
  assert.ok(rowByKey(rows, 'setting:this-browser/cfg-theme'));
  assert.equal(rowByKey(rows, 'setting:branding/cfg-brand'), null);
});

test('paletteView: empty query lists last picks, then the categories', () => {
  const rows = catalogue(src({ page: 'log' }));
  const v = paletteView(rows, { last: ['preset:global:p1', 'gone', 'page:home'] });
  assert.deepEqual(
    v.slice(0, 4).map((x) => x.row?.key || x.label),
    ['page:home', 'Last picks', 'preset:global:p1', 'Categories'],
  );
  assert.deepEqual(
    v.filter((x) => x.type === 'cat').map((x) => x.id),
    CATEGORIES.map((c) => c.id),
  );
  assert.equal(v.find((x) => x.id === 'presets').count, 2);
});

test('paletteView: ticked bookmarks come first, then the last picks without them', () => {
  const rows = catalogue(src({ page: 'log' }));
  const marks = [rowByKey(rows, 'page:panels'), rowByKey(rows, 'preset:global:p1')];
  const v = paletteView(rows, { last: ['preset:global:p1', 'page:log'], marks });
  assert.deepEqual(
    v.slice(0, 7).map((x) => x.row?.key || x.label),
    ['page:home', 'Bookmarks', 'page:panels', 'preset:global:p1', 'Last picks', 'page:log', 'Categories'],
  );
  // A query searches every row; the bookmarks have no heading of their own there.
  assert.equal(
    paletteView(rows, { query: 'panels', marks }).some((x) => x.type === 'head'),
    false,
  );
});

test('paletteView: a category, Settings sections, and a search inside a level', () => {
  const rows = catalogue(src());
  assert.deepEqual(
    paletteView(rows, { path: ['presets'] }).map((x) => x.row.key),
    ['preset:global:p1', 'preset:local:p2'],
  );
  const secs = paletteView(rows, { path: ['settings'] });
  assert.deepEqual(
    secs.map((x) => [x.type, x.id, x.count]),
    [
      ['cat', 'this-browser', 1],
      ['cat', 'branding', 1],
      ['cat', 'users', 1],
    ],
  );
  assert.deepEqual(
    paletteView(rows, { path: ['settings', 'branding'] }).map((x) => x.row.key),
    ['setting:branding/cfg-brand'],
  );
  assert.equal(pathText(['settings', 'branding'], rows), 'Settings › Branding');
  assert.deepEqual(
    paletteView(rows, { path: ['settings'], query: 'theme' }).map((x) => x.row.key),
    ['setting:this-browser/cfg-theme'],
  );
});

test('rankRows: the label beats the hint, and nothing matches nothing', () => {
  const rows = catalogue(src());
  assert.equal(rankRows(rows, 'studio')[0].key, 'preset:global:p1');
  assert.equal(rankRows(rows, 'reread')[0]?.key, 'action:reread');
  assert.deepEqual(rankRows(rows, 'zzzzqq'), []);
  // The doubled Palette character searches for the character itself.
  const named = catalogue(src({ presets: [{ scope: 'global', id: 'q', name: '>Wall' }] }));
  assert.equal(rankRows(named, '>')[0].key, 'preset:global:q');
});

test('stepIndex skips heading lines and wraps', () => {
  const items = [{ type: 'row' }, { type: 'head' }, { type: 'cat' }];
  assert.equal(firstIndex(items), 0);
  assert.equal(stepIndex(items, 0, 1), 2);
  assert.equal(stepIndex(items, 2, 1), 0);
  assert.equal(stepIndex(items, 0, -1), 2);
  assert.equal(firstIndex([{ type: 'head' }]), -1);
});
