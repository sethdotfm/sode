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
  guessCategories,
  paletteScope,
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

const keys = (items) => items.map((x) => (x.type === 'cat' ? `> ${x.id}` : x.row?.key || `# ${x.label}`));
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
    { scope: 'all', id: 'All', name: 'All' },
    { scope: 'global', id: 'c1', name: 'Cameras' },
  ],
  panels: [{ id: 'x1', name: 'Wall', type: 'grid' }],
  settings: [
    { section: 'this-browser', sectionTitle: 'This browser', topic: 'Preferences', id: 'cfg-theme', label: 'Theme' },
    { section: 'branding', sectionTitle: 'Branding', topic: 'Look and feel', id: 'cfg-brand', label: 'Logo text' },
    { section: 'users', sectionTitle: 'Users', topic: 'Access', id: 'users-manage', label: 'Manage users', button: 'users-manage' },
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
  assert.equal(rowByKey(rows, 'filterSet:global:c1').label, 'Cameras', 'a filter set row key holds its hidden id');
  assert.equal(rowByKey(rows, 'setting:users/users-manage').kind, 'settingsButton');
  assert.ok(rowByKey(rows, 'action:reread').home);
  assert.ok(!rowByKey(rows, 'action:lock').home);
  assert.equal(new Set(rows.map((r) => r.key)).size, rows.length, 'keys are unique');
  for (const r of rows)
    assert.ok(
      CATEGORIES.some((c) => c.id === r.cat),
      r.key,
    );
});

test('catalogue: Home on every page, in the empty Palette on every page but Home', () => {
  const here = rowByKey(catalogue(src()), 'page:home');
  assert.deepEqual([here.here, here.home], [true, true], 'on Home it runs there, as the Home key');
  assert.ok(!paletteView(catalogue(src())).some((x) => x.row?.key === 'page:home'));
  assert.equal(keys(paletteView(catalogue(src()), { query: 'home' }))[0], 'page:home');
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
  const users = rowByKey(rows, 'setting:users/users-manage');
  assert.equal(users.access.ok, false);
  assert.deepEqual(users.access.offer, ['stepIn']);

  const admin = catalogue(src({ who: who({ role: 'administrator' }) }));
  assert.ok(
    admin.every((r) => r.access.ok),
    'an Administrator with all Rights gets no grey row',
  );

  // Step in needs another user with a PIN.
  const alone = catalogue(src({ who: who({ users: [{ name: 'Ana', len: 4 }] }) }));
  assert.deepEqual(rowByKey(alone, 'setting:users/users-manage').access.offer, []);
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
    paletteView(rows, { query: 'log', marks }).some((x) => x.type === 'head'),
    false,
  );
});

test('paletteView: a category, Settings sections, and a search inside a level', () => {
  const rows = catalogue(src({ who: who({ role: 'administrator' }) }));
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

test('rankRows: grey rows come after every usable row, and the cursor starts on a usable one', () => {
  const off = (key, label) => ({ key, label, access: { ok: false, why: 'Outside your Role' } });
  const on = (key, label) => ({ key, label, access: { ok: true } });
  const rows = [off('a', 'Record'), on('b', 'Page border color'), on('c', 'Recall preset')];
  assert.deepEqual(
    rankRows(rows, 'rec').map((r) => r.key),
    ['c', 'a'],
    'the letters in order must start a word: "rec" does not find "Page border color"',
  );
  assert.deepEqual(
    rankRows(rows, '').map((r) => r.key),
    ['b', 'c', 'a'],
  );
  const items = [{ type: 'head' }, { type: 'row', row: rows[0] }, { type: 'row', row: rows[1] }];
  assert.equal(firstIndex(items), 2);
  assert.equal(firstIndex([{ type: 'row', row: rows[0] }]), 0);
});

test('guessCategories: the first word names a category by prefix, letters in order or one typo', () => {
  const table = {
    p: ['panels'],
    pan: ['panels'],
    panel: ['panels'],
    panels: ['panels'],
    pnl: ['panels'],
    pnls: ['panels'],
    panle: ['panels'],
    pannel: ['panels'],
    pre: ['presets'],
    ps: ['presets'],
    presset: ['presets'],
    pg: ['pages'],
    pgs: ['pages'],
    page: ['pages'],
    a: ['actions'],
    act: ['actions'],
    acton: ['actions'],
    f: ['filterSets'],
    fs: ['filterSets'],
    fltr: ['filterSets'],
    filter: ['filterSets'],
    s: ['settings'],
    set: ['settings'],
    sett: ['settings'],
    stg: ['settings'],
    setings: ['settings'],
    cfg: ['settings'],
    pa: ['pages', 'panels'],
    x: [],
    c: [],
    studio: [],
    wall: [],
  };
  for (const [w, want] of Object.entries(table)) assert.deepEqual(guessCategories(w), want, w);
});

const searchSrc = (more = {}) =>
  src({
    presets: [{ scope: 'global', id: 'p1', name: 'P 1' }],
    panels: [
      { id: 'x1', name: 'New Panel', type: 'buttons' },
      { id: 'x2', name: 'Wall', type: 'grid' },
    ],
    settings: [
      { section: 'panels', sectionTitle: 'Panels', topic: 'Access', id: 'cfg-np', label: 'New-panel Authentication mode' },
      { section: 'this-browser', sectionTitle: 'This browser', topic: 'Preferences', id: 'cfg-theme', label: 'Theme' },
    ],
    who: who({ role: 'administrator' }),
    ...more,
  });

test('paletteScope: one clear category, never a tie, never over a name that starts with the query', () => {
  const rows = catalogue(searchSrc());
  assert.deepEqual(paletteScope('pnl new', rows), { cat: 'panels', word: 'pnl', rest: 'new' });
  assert.deepEqual(paletteScope('p', rows), { cat: 'panels', word: 'p', rest: '' });
  assert.equal(paletteScope('pa new', rows), null, 'a tie does not scope');
  assert.equal(paletteScope('p 1', rows), null, 'the preset P 1 starts with the query');
  assert.equal(paletteScope('studio', rows), null);
  assert.equal(pathText([], rows, 'set new'), 'Settings first');
  assert.equal(pathText([], rows, 'new panel'), '');
});

test('search: user rows beat settings, and a scope lists its category first', () => {
  const rows = catalogue(searchSrc());
  const v = (query) => keys(paletteView(rows, { query }));
  assert.equal(v('new panel')[0], 'panel:x1', 'the panel New Panel beats the New-panel setting');
  assert.deepEqual(v('pnl new'), ['panel:x1']);
  assert.deepEqual(v('p').slice(0, 4), ['> panels', 'panel:x1', 'panel:x2', '# Other matches'], 'the category first, so Enter opens it');
  assert.deepEqual(v('panel').slice(0, 4), ['> panels', 'panel:x1', 'panel:x2', '# Other matches']);
  assert.deepEqual(v('panels').slice(0, 2), ['> panels', 'page:panels'], 'a page with the word as its full label comes next');
  assert.deepEqual(v('settings').slice(0, 2), ['> settings', 'page:settings']);
  assert.deepEqual(v('set new'), ['setting:panels/cfg-np']);
  assert.equal(v('p 1')[0], 'preset:global:p1', 'a real name that starts with the category word');
  assert.deepEqual(v('pnl zzz'), ['# No match in Panels']);
  // A tie lists the rows of both categories first.
  const pa = v('pa');
  assert.deepEqual(pa.slice(0, pa.indexOf('# Other matches')).sort(), [
    '> pages',
    '> panels',
    'page:editor',
    'page:home',
    'page:log',
    'page:panels',
    'page:settings',
    'panel:x1',
    'panel:x2',
  ]);
  // A label match beats a match of the words only.
  const word = { key: 'a', cat: 'panels', label: 'Zed', words: ['alpha'], access: { ok: true } };
  const label = { key: 'b', cat: 'settings', label: 'Mode alpha', words: [], access: { ok: true } };
  assert.deepEqual(
    rankRows([word, label], 'alpha').map((r) => r.key),
    ['b', 'a'],
  );
  assert.equal(v('theme')[0], 'action:theme', 'the action beats the setting with the same word');
  assert.equal(v('theme')[1], 'setting:this-browser/cfg-theme');
});

test('search: grey rows show only as bookmarks with Show in Palette', () => {
  const rows = catalogue(src({ who: who({ rights: { presets: ['p2'] } }) }));
  const studio = rowByKey(rows, 'preset:global:p1');
  assert.equal(studio.access.ok, false);
  assert.ok(!keys(paletteView(rows, { query: 'studio' })).includes(studio.key));
  assert.ok(!keys(paletteView(rows, { path: ['presets'] })).includes(studio.key));
  assert.equal(paletteView(rows).find((x) => x.id === 'presets').count, 1);
  assert.ok(!keys(paletteView(rows, { last: [studio.key] })).includes(studio.key), 'a grey last pick is gone');
  assert.ok(keys(paletteView(rows, { marks: [studio] })).includes(studio.key));
  assert.equal(keys(paletteView(rows, { query: 'studio', marks: [studio] }))[0], studio.key);
  assert.deepEqual(
    paletteView(rows, { path: ['settings'] }).map((x) => x.id),
    ['this-browser'],
    'a Settings section with only grey rows is gone',
  );
});
