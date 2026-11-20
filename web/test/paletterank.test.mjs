// The Palette's ranking and keys, with the queries of a keyboard review. They check loose matches, settings over
// actions, danger rows, twin settings, the empty Palette and the category word.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { paletteView, firstIndex, keyMove, jumpIndex, stepIndex, emptyBackspace, isDanger, dropTwins } from '../paletterules.js';
import { catalogue, rowByKey } from '../palettecatalogue.js';

const users = [
  { name: 'Ana', len: 4 },
  { name: 'Bo', len: 4 },
];
const set = (section, sectionTitle, topic, id, label, button = '') => ({ section, sectionTitle, topic, id, label, button });
const pref = (id, label) => [
  set('this-browser', 'This browser', 'Preferences', `cfg-${id}`, label),
  set('everyone-defaults', "Everyone's defaults", 'Look and feel', `gcfg-${id}`, label),
];
const settings = [
  ...pref('theme', 'Theme'),
  ...pref('set-stack', 'Stacked filters behavior'),
  ...pref('stack-gap', 'Stack quick changes to one destination'),
  ...pref('palette-char', 'Palette character'),
  set('this-browser', 'This browser', 'Preferences', 'my-default-theme', 'Save Theme as my default', 'my-default-theme'),
  set('this-browser', 'This browser', 'Preferences', 'my-default-clock', 'Save Clock as my default', 'my-default-clock'),
  set('my-user', 'My user', 'Preferences', 'account-keys', 'My API keys', 'account-keys'),
  set('my-user', 'My user', 'Preferences', 'account-transfer', 'Export & Import', 'account-transfer'),
  set('history', 'History', 'Look and feel', 'cfg-hidden', 'Hidden destinations in History'),
  set('history', 'History', 'Look and feel', 'clear-history', 'Clear History', 'clear-history'),
  set('takes', 'Takes', 'Routers', 'cfg-salvogroup', 'Router salvo group'),
  set('locks', 'Locks', 'Access', 'cfg-lock-main', 'Main page'),
  set('api', 'API', 'Access', 'api-log-reads', 'Log API reads'),
];
const who = (more = {}) => ({ role: 'administrator', user: 'Ana', rights: null, locks: {}, users, phone: false, stepped: false, ...more });
const src = (more = {}) => ({
  page: 'main',
  presets: [
    { scope: 'global', id: 'p1', name: 'Studio A cams' },
    { scope: 'global', id: 'p2', name: 'Sign language feed' },
  ],
  filterSets: [{ scope: 'all', id: 'All', name: 'All' }],
  panels: [{ id: 'x1', name: 'New Panel', type: 'grid' }],
  settings,
  who: who(),
  ...more,
});
const keys = (items) => items.map((x) => (x.type === 'cat' ? `> ${x.id}` : x.row?.key || `# ${x.label}`));
// The row that Enter picks first, or '' for none.
const enterRow = (items, empty = false) => {
  const i = firstIndex(items, { empty });
  return i < 0 ? '' : keys(items)[i];
};
const rows = catalogue(src());
const find = (query, r = rows) => paletteView(r, { query });

test('a loose match is never the row that Enter picks first', () => {
  for (const q of ['sing in', 'salvo x', 'ip', 'patch x', 'lgo']) {
    const items = find(q);
    const first = enterRow(items);
    if (first) assert.ok(!items[firstIndex(items)].weak, q);
    for (const it of items) if (it.weak) assert.ok(keys(items).indexOf('# Other matches') < items.indexOf(it), `${q}: a weak row is under the heading`);
  }
  assert.equal(enterRow(find('sing in')), '', '"sing in" has only loose matches: nothing is picked');
  assert.equal(enterRow(find('ip')), '', '"ip" is not Export & Import');
  assert.equal(enterRow(find('salvo')), 'setting:takes/cfg-salvogroup', 'a whole word beats letters in order');
  assert.equal(enterRow(find('patch')), 'page:home', 'patch is a word of Home');
});

test('signed in, sign in and log in offer Sign out and Step in', () => {
  for (const q of ['sign in', 'login', 'log in']) {
    const k = keys(find(q));
    assert.equal(enterRow(find(q)), 'action:signOut', q);
    assert.ok(k.includes('action:stepIn'), `${q}: Step in`);
    assert.ok(!k.slice(0, 2).includes('setting:this-browser/cfg-stack-gap'), `${q}: not the loose Stack quick changes`);
  }
  assert.equal(enterRow(find('sign in', catalogue(src({ who: who({ user: '' }) })))), 'action:signIn', 'a Guest gets Sign in');
});

test('an action or a page beats a setting with the same word', () => {
  const table = {
    theme: 'action:theme',
    keys: 'action:shortcuts',
    router: 'action:reread',
    log: 'page:log',
    main: 'page:home',
    history: 'action:filters',
    home: 'page:home',
    reread: 'action:reread',
    'my api keys': 'setting:my-user/account-keys',
    studio: 'preset:global:p1',
  };
  for (const [q, want] of Object.entries(table)) assert.equal(enterRow(find(q)), want, q);
});

test('a danger row ranks last and Enter never picks it first', () => {
  const clear = rowByKey(rows, 'setting:history/clear-history');
  assert.ok(isDanger(clear));
  assert.ok(isDanger({ label: 'Delete the panel' }) && isDanger({ label: 'Reset to default' }));
  assert.ok(!isDanger({ label: 'Theme' }));
  const items = find('clear history');
  assert.equal(enterRow(items), '', 'only the danger row matches: nothing is picked');
  const k = keys(items);
  assert.ok(k.indexOf('setting:history/clear-history') > k.indexOf('# Other matches'));
  const hist = keys(find('history'));
  assert.equal(hist[hist.length - 1], 'setting:history/clear-history', 'last');
});

test('a setting shows one time, in This browser; its section still lists both', () => {
  const k = keys(find('theme'));
  assert.ok(k.includes('setting:this-browser/cfg-theme'));
  assert.ok(!k.includes('setting:everyone-defaults/gcfg-theme'));
  assert.ok(keys(paletteView(rows, { path: ['settings', 'everyone-defaults'] })).includes('setting:everyone-defaults/gcfg-theme'));
  assert.ok(!keys(paletteView(rows, { path: ['settings'], query: 'theme' })).includes('setting:everyone-defaults/gcfg-theme'));
  // A Standard user cannot change Everyone's defaults: the grey twin is gone anyway, and This browser stays.
  const std = catalogue(src({ who: who({ role: 'standard' }) }));
  assert.deepEqual(
    keys(find('theme', std)).filter((x) => x.startsWith('setting:') && !x.includes('my-default')),
    ['setting:this-browser/cfg-theme'],
  );
  // With no This browser row to change, the Everyone's defaults row stays.
  const only = dropTwins(rows.filter((r) => r.key !== 'setting:this-browser/cfg-theme'));
  assert.ok(rowByKey(only, 'setting:everyone-defaults/gcfg-theme'));
});

test('a my default button names its setting', () => {
  const k = keys(find('save'));
  assert.ok(k.includes('setting:this-browser/my-default-theme') && k.includes('setting:this-browser/my-default-clock'));
  assert.equal(new Set(find('save').map((x) => x.row?.label)).size, find('save').length, 'no two rows look the same');
});

test('the empty Palette: Enter picks nothing but a category', () => {
  const log = catalogue(src({ page: 'log' }));
  const marks = [rowByKey(log, 'preset:global:p1')];
  assert.equal(enterRow(paletteView(log, { marks }), true), '', 'not the bookmarked preset');
  assert.equal(enterRow(paletteView(log, { last: ['preset:global:p2'] }), true), '', 'not a last pick');
  assert.equal(enterRow(paletteView(rows), true), '> actions', 'only categories: the first one');
  assert.equal(enterRow(paletteView(rows, { path: ['presets'] }), true), '', 'inside Presets with no text: no preset');
  assert.equal(enterRow(paletteView(rows, { path: ['settings'] }), true), '> this-browser', 'a Settings section opens');
});

test('a category word alone puts the category first, so Enter opens it', () => {
  assert.equal(enterRow(find('pre')), '> presets', 'pre does not fire the first preset');
  assert.equal(enterRow(find('panels')), '> panels');
  assert.deepEqual(keys(find('settings')).slice(0, 2), ['> settings', 'page:settings'], 'and the Settings page right after');
  assert.equal(enterRow(find('pre studio')), 'preset:global:p1', 'with more text the rows of the category come first');
});

test('keyMove: Tab steps, PageUp and PageDown jump, Ctrl+Home and Ctrl+End go to the ends', () => {
  const items = [{ type: 'head' }, ...Array.from({ length: 30 }, () => ({ type: 'row' })), { type: 'head' }, { type: 'row' }];
  const k = (key, cur, mods = {}) => keyMove({ key, ...mods }, items, cur, 10);
  assert.equal(k('Tab', 1), 2);
  assert.equal(k('Tab', 1, { shiftKey: true }), 32, 'Shift+Tab wraps past the heading');
  assert.equal(k('Tab', -1), 1, 'from no row, Tab goes to the first');
  assert.equal(k('Tab', -1, { shiftKey: true }), 32, 'and Shift+Tab to the last');
  assert.equal(k('PageDown', 1), 11);
  assert.equal(k('PageDown', 28), 32, 'past the heading to the last row, no wrap');
  assert.equal(k('PageUp', 5), 1, 'stops at the first row');
  assert.equal(k('Home', 20, { ctrlKey: true }), 1);
  assert.equal(k('End', 2, { ctrlKey: true }), 32);
  assert.equal(k('Home', 20), null, 'plain Home moves in the text');
  assert.equal(k('End', 20), null);
  assert.equal(k('a', 2), null);
  assert.equal(k('Tab', 2, { altKey: true }), null);
  assert.equal(jumpIndex([{ type: 'head' }], 0, 10), -1);
  assert.equal(stepIndex(items, -1, -1), 32);
});

test('Backspace in an empty box: up one level, and nothing at the top', () => {
  assert.equal(emptyBackspace([]), 'none');
  assert.equal(emptyBackspace(['settings']), 'up');
  assert.equal(emptyBackspace(['settings', 'history']), 'up');
});
