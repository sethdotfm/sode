import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { paletteView, paletteScope, rankRows, addUse, cleanUses, useBoost, USE_TOP, USES_MAX } from '../paletterules.js';
import { catalogue, rowByKey, BUTTON_ACTIONS, buttonMode } from '../palettecatalogue.js';

const users = [
  { name: 'Ana', len: 4 },
  { name: 'Bo', len: 4 },
];
const who = (more = {}) => ({ role: 'standard', user: 'Ana', rights: null, locks: {}, users, phone: false, stepped: false, ...more });
const myUser = (id, label) => ({ section: 'my-user', sectionTitle: 'My user', topic: 'Preferences', id, label, button: id });
const settings = [
  myUser('account-login', 'Sign in'),
  myUser('account-stepin', 'Step in'),
  myUser('account-stepout', 'Step out'),
  myUser('account-pin', 'Change my PIN'),
  myUser('account-keys', 'My API keys'),
  myUser('account-logout', 'Sign out'),
  myUser('lock-btn', 'Lock'),
  myUser('clear-history', 'Clear History'),
  { section: 'this-browser', sectionTitle: 'This browser', topic: 'Preferences', id: 'cfg-theme', label: 'Theme', button: '' },
];
const src = (more = {}) => ({ page: 'main', presets: [], filterSets: [], panels: [], settings, who: who(), ...more });
const keys = (items) => items.map((x) => x.row?.key || `# ${x.label}`);
const view = (rows, query, more = {}) => keys(paletteView(rows, { query, ...more }));

test('buttonMode: every Settings button in index.html runs, opens Settings, or is an action', () => {
  const html = readFileSync(new URL('../index.html', import.meta.url), 'utf8');
  const finds = new Set([...html.matchAll(/data-find="([^"]+)"/g)].map((m) => m[1]));
  const buttons = [...html.matchAll(/<button\b[^>]*>/g)]
    .map((m) => ({ id: /\bid="([^"]+)"/.exec(m[0])?.[1], find: /data-find=/.test(m[0]) }))
    .filter((b) => b.id && (b.find || finds.has(b.id)))
    .map((b) => b.id);
  const want = {
    'account-login': 'action',
    'account-stepin': 'action',
    'account-stepout': 'action',
    'account-logout': 'action',
    'lock-btn': 'action',
    'account-pin': 'run',
    'account-keys': 'run',
    'account-clients': 'run',
    'account-profile': 'run',
    'account-copy': 'run',
    'account-transfer': 'run',
    'xi-open': 'run',
    'clear-history': 'settings',
    'protects-clear-all': 'settings',
    'cfg-apply-preset': 'settings',
    'users-manage': 'settings',
    'setup-again-open': 'settings',
    'router-add': 'settings',
    'router-names-reread': 'action',
  };
  assert.deepEqual(buttons.sort(), Object.keys(want).sort(), 'a new Settings button needs a decision in palettecatalogue.js');
  for (const id of buttons) assert.equal(buttonMode(id), want[id], id);
  assert.equal(buttonMode('my-default-theme'), 'settings', 'my default changes the value beside it');
  assert.equal(buttonMode('something-new'), 'locate', 'a button that no list names is only shown');
});

test('catalogue: a Settings button row carries its button id and mode', () => {
  const rows = catalogue(src());
  const pin = rowByKey(rows, 'setting:my-user/account-pin');
  assert.equal(pin.kind, 'settingsButton');
  assert.deepEqual([pin.target.button, pin.target.mode, pin.icon], ['account-pin', 'run', 'mouse-pointer-click']);
  assert.equal(pin.hint, 'Preferences › My user', 'the path stays as context');
  assert.equal(rowByKey(rows, 'setting:my-user/clear-history').target.mode, 'settings');
  assert.equal(rowByKey(rows, 'setting:this-browser/cfg-theme').kind, 'setting');
});

test('catalogue: Sign out only when signed in, Step out only when stepped in', () => {
  const signedIn = catalogue(src());
  assert.ok(rowByKey(signedIn, 'action:signOut'));
  assert.equal(rowByKey(signedIn, 'action:stepOut'), null);
  assert.equal(rowByKey(signedIn, 'action:signIn'), null);

  const guest = catalogue(src({ who: who({ user: '' }) }));
  assert.equal(rowByKey(guest, 'action:signOut'), null);
  assert.ok(rowByKey(guest, 'action:signIn'));

  const stepped = catalogue(src({ who: who({ user: 'Bo', stepped: true }) }));
  assert.ok(rowByKey(stepped, 'action:stepOut'));
  assert.equal(rowByKey(stepped, 'action:signOut'), null, 'Settings hides Sign out while stepped in too');
  assert.equal(rowByKey(stepped, 'action:stepIn'), null);
});

test('catalogue: an action and its Settings button show as one row', () => {
  for (const [w, query, action] of [
    [who(), 'sign out', 'action:signOut'],
    [who({ user: '' }), 'sign in', 'action:signIn'],
    [who({ user: 'Bo', stepped: true }), 'step out', 'action:stepOut'],
    [who(), 'step in', 'action:stepIn'],
    [who(), 'lock', 'action:lock'],
  ]) {
    const rows = catalogue(src({ who: w }));
    const found = view(rows, query);
    assert.equal(found[0], action, query);
    const label = rowByKey(rows, action).label;
    assert.equal(found.filter((k) => rowByKey(rows, k)?.label === label).length, 1, `one ${label}`);
  }
  // The button's key stays, so a bookmark of it keeps working: it is the action.
  const rows = catalogue(src());
  const alias = rowByKey(rows, 'setting:my-user/account-logout');
  assert.deepEqual([alias.kind, alias.target.id, alias.alias, alias.home], ['action', 'signOut', true, false]);
  assert.ok(keys(paletteView(rows, { query: 'sign out', pins: [alias] })).includes(alias.key), 'a Palette pin still shows');
  // Signed out, the Settings Sign out is not there, as in Settings.
  assert.equal(rowByKey(catalogue(src({ who: who({ user: '' }) })), 'setting:my-user/account-logout'), null);
  assert.equal(Object.keys(BUTTON_ACTIONS).length, 6);
});

test('catalogue: buttons for a signed-in user only, API keys only with a PIN', () => {
  const guest = catalogue(src({ page: 'log', who: who({ user: '' }) }));
  assert.equal(rowByKey(guest, 'setting:my-user/account-pin'), null);
  assert.equal(rowByKey(guest, 'setting:my-user/account-keys'), null);
  const noPin = catalogue(src({ who: who({ user: 'Cy', users: [...users, { name: 'Cy', len: 0 }] }) }));
  assert.ok(rowByKey(noPin, 'setting:my-user/account-pin'));
  assert.equal(rowByKey(noPin, 'setting:my-user/account-keys'), null);
});

test('paletteScope: a weak category guess loses to a row label', () => {
  const rows = catalogue(src());
  assert.equal(paletteScope('sig', rows), null, 'sig is Sign out, not settings');
  assert.equal(view(rows, 'sig')[0], 'action:signOut');
  assert.equal(view(catalogue(src({ who: who({ user: '' }) })), 'sig')[0], 'action:signIn');
  assert.equal(paletteScope('set', rows)?.cat, 'settings', 'a short word from the table');
  assert.equal(paletteScope('sett', rows)?.cat, 'settings', 'a prefix of the category name');
  assert.equal(paletteScope('stg', rows)?.cat, 'settings', 'no label starts with stg');
  const stg = catalogue(src({ presets: [{ scope: 'global', id: 's1', name: 'Stg feed' }] }));
  assert.equal(paletteScope('stg', stg), null, 'a label starts with stg');
  assert.equal(view(stg, 'stg')[0], 'preset:global:s1');
});

const row = (key, label, cat = 'settings') => ({ key, label, cat, words: [], hint: '', access: { ok: true } });

test('uses: a count and the last time, the newest kept', () => {
  let u = addUse({}, 'a', 1000);
  u = addUse(u, 'a', 2000);
  assert.deepEqual(u.a, { n: 2, at: 2000 });
  assert.deepEqual(cleanUses([1]), {});
  assert.deepEqual(cleanUses({ a: { n: 'x', at: 1 }, b: { n: 1, at: 1 } }), { b: { n: 1, at: 1 } });
  let many = {};
  for (let i = 0; i < USES_MAX + 5; i++) many = addUse(many, `k${i}`, i);
  assert.equal(Object.keys(many).length, USES_MAX);
  assert.ok(!many.k0 && many[`k${USES_MAX + 4}`]);
});

test('useBoost: bounded, more for more picks, less with age', () => {
  const now = 100 * 86_400_000;
  assert.equal(useBoost(undefined, now), 0);
  const one = useBoost({ n: 1, at: now }, now);
  const ten = useBoost({ n: 10, at: now }, now);
  assert.ok(one > 0 && ten > one);
  assert.equal(useBoost({ n: 10_000, at: now }, now), USE_TOP);
  assert.ok(useBoost({ n: 10, at: now - 14 * 86_400_000 }, now) < ten * 0.51, 'half after two weeks');
});

test('rankRows: use moves a row only among matches of its strength', () => {
  const now = Date.now();
  const rows = [row('a', 'Mode alfa'), row('b', 'Mode beta')];
  assert.deepEqual(
    rankRows(rows, 'mode').map((r) => r.key),
    ['a', 'b'],
  );
  assert.deepEqual(
    rankRows(rows, 'mode', 50, { uses: { b: { n: 3, at: now } }, now }).map((r) => r.key),
    ['b', 'a'],
    'a used row first among equal matches',
  );
  const strong = [row('used', 'Design notes'), row('x', 'Sig level')];
  assert.deepEqual(
    rankRows(strong, 'sig', 50, { uses: { used: { n: 10_000, at: now } }, now }).map((r) => r.key),
    ['x', 'used'],
    'a label that starts with the query beats a used row that only holds it',
  );
  const weak = [row('used', 'Stage grid'), row('x', 'Sgx mode')];
  assert.deepEqual(
    rankRows(weak, 'sg', 50, { uses: { used: { n: 10_000, at: now } }, now }).map((r) => r.key),
    ['x', 'used'],
    'a prefix beats a used letters-in-order match',
  );
  // The common actions come first among equal matches with no use at all.
  assert.deepEqual(
    rankRows([row('action:other', 'Lock'), row('action:lock', 'Lock')], 'lock').map((r) => r.key),
    ['action:lock', 'action:other'],
  );
});
