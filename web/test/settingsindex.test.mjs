import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { DICT, buildIndex, find, slug } from '../settingsindex.js';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');
const read = (f) => readFileSync(join(web, f), 'utf8');

// The sections as the page registers them: the settings view's own fieldsets
// plus the ones modules add (users.js).
function registered() {
  const html = read('index.html');
  const view = html.slice(html.indexOf('id="settings-view"'));
  const titles = [...view.matchAll(/<fieldset class="sode__group[^"]*"[^>]*>\s*<legend>([^<]+)<\/legend>/g)].map((m) => m[1]);
  const lines = read('users.js') + read('settings.js');
  for (const m of lines.matchAll(/el\('legend', null, '([^']+)'\)/g)) titles.push(m[1]);
  return [...new Set(titles)];
}

const idx = () =>
  buildIndex(
    registered()
      .map((t) => ({ id: slug(t), title: t, settings: [] }))
      .concat([
        {
          id: 'access',
          title: 'Access',
          settings: [
            { id: 'cfg-pin', label: 'Advanced Settings PIN' },
            { id: 'cfg-pin-tries', label: 'Lock after wrong PINs' },
          ],
        },
        {
          id: 'history',
          title: 'History',
          settings: [
            { id: 'open-log', label: 'Open the Communication Log' },
            { id: 'clear-history', label: 'Clear history' },
          ],
        },
      ]),
  );
const top = (q) => find(idx(), q)[0];

test('sections appear in the agreed order', () => {
  const html = read('index.html');
  const view = html.slice(html.indexOf('id="settings-view"'));
  const titles = [...view.matchAll(/<fieldset class="sode__group[^"]*"[^>]*>\s*<legend>([^<]+)<\/legend>/g)].map((m) => m[1]);
  // users.js inserts Users right after Access.
  titles.splice(titles.indexOf('Access') + 1, 0, 'Users');
  assert.deepEqual(titles, [
    'This browser',
    'My user',
    'Global defaults',
    'Router & matrix',
    'Takes',
    'Router Source & Destination Names',
    'Level colors',
    'Branding',
    'Access',
    'Users',
    'History',
    'Proxy',
    'Export & import',
    'Server backup',
  ]);
  assert.match(view, /<fieldset class="sode__group"[^>]*data-toc="Router names"[^>]*>\s*<legend>Router Source/);
});

test('every registered section has dictionary terms', () => {
  const secs = registered();
  assert.ok(secs.length >= 12, 'found the sections: ' + secs.join(', '));
  for (const t of secs) assert.ok(DICT[slug(t)]?.length, `add DICT['${slug(t)}'] in settingsindex.js`);
});

test('common words reach their section', () => {
  const cases = {
    dark: 'this-browser',
    clock: 'this-browser',
    ip: 'router-matrix',
    host: 'router-matrix',
    poll: 'router-source-destination-names',
    offset: 'router-matrix',
    undo: 'history',
    audit: 'proxy',
    proxy: 'proxy',
    brand: 'branding',
    border: 'branding',
    salvo: 'salvos',
  };
  for (const [q, sec] of Object.entries(cases)) {
    const hits = find(buildIndex([...registered().map((t) => ({ id: slug(t), title: t })), { id: 'salvos', title: 'Salvos' }]), q);
    assert.ok(
      hits.some((h) => h.sectionId === sec),
      `${q} -> ${sec}`,
    );
  }
});

test('password, pin, lock, idle and auto-lock land on Users or Access', () => {
  for (const q of ['password', 'pin', 'lock', 'timeout', 'idle', 'auto-lock']) {
    const hits = find(idx(), q);
    assert.ok(
      hits.some((h) => h.sectionId === 'users' || h.sectionId === 'access'),
      q,
    );
  }
});

test('settings are found by label and by dictionary', () => {
  assert.equal(top('wrong pins').id, 'cfg-pin-tries');
  assert.equal(top('communication log').id, 'open-log');
  assert.equal(top('uptime').id, 'open-log');
  assert.equal(top('wipe').id, 'clear-history');
});

test('the router salvo group is found by its words', () => {
  const index = buildIndex([
    {
      id: 'router',
      title: 'Router',
      settings: [{ id: 'cfg-salvogroup', label: 'Router salvo group (advanced)' }],
    },
  ]);
  for (const q of ['salvo group', 'native', 'group number', 'crosspoint list']) {
    assert.ok(
      find(index, q).some((h) => h.id === 'cfg-salvogroup'),
      q,
    );
  }
});

test('the fire mode setting is found by its words', () => {
  assert.ok(DICT['cfg-firemode']?.length, "add DICT['cfg-firemode'] in settingsindex.js");
  const index = buildIndex([
    {
      id: 'router',
      title: 'Router',
      settings: [{ id: 'cfg-firemode', label: 'Multi destination behavior' }],
    },
  ]);
  for (const q of ['native', 'fire mode', 'router salvo', 'preset']) {
    assert.ok(
      find(index, q).some((h) => h.id === 'cfg-firemode'),
      q,
    );
  }
});

test('typos still match', () => {
  assert.equal(top('pasword').sectionId === 'access' || top('pasword').sectionId === 'users', true);
  assert.equal(top('histroy').sectionId, 'history');
  assert.equal(top('comunication log').id, 'open-log');
});

test('ordering: exact label beats a dictionary hit; section before its settings on a tie', () => {
  const index = buildIndex(
    [
      { id: 'a', title: 'Alpha', settings: [{ id: 'x', label: 'Colour' }] },
      { id: 'b', title: 'Beta', settings: [] },
    ],
    { b: ['colour'], x: [] },
  );
  assert.equal(find(index, 'colour')[0].id, 'x');
  const two = buildIndex([{ id: 'users', title: 'Users', settings: [{ id: 'u1', label: 'Users thing' }] }]);
  assert.equal(find(two, 'users')[0].id, 'users');
});

test('unknown ids in the dictionary are ignored; unknown queries find nothing', () => {
  const index = buildIndex([{ id: 'names', title: 'Names', settings: [] }]);
  assert.deepEqual(find(index, 'salvo').length, 0);
  assert.equal(find(index, 'zzzqq').length, 0);
  assert.equal(find(index, '   ').length, 0);
});

test('all words of a query must match', () => {
  assert.ok(find(idx(), 'auto lock').some((h) => h.sectionId === 'users'));
  assert.equal(find(idx(), 'dark banana').length, 0);
});

test('this device still finds the This browser section', () => {
  const secs = [{ id: 'this-browser', title: 'This browser', settings: [{ id: 'cfg-theme', label: 'Theme' }] }];
  assert.equal(find(buildIndex(secs), 'this device')[0]?.sectionId, 'this-browser');
});

test('account words reach the Me block inside This browser', () => {
  const secs = [
    {
      id: 'this-browser',
      title: 'This browser',
      settings: [
        { id: 'account', label: 'Me' },
        { id: 'account-pin', label: 'Change my PIN' },
        { id: 'account-logout', label: 'Sign out' },
      ],
    },
  ];
  for (const [q, id] of [
    ['account', 'account'],
    ['sign out', 'account-logout'],
    ['logout', 'account-logout'],
    ['change pin', 'account-pin'],
  ]) {
    assert.ok(
      find(buildIndex(secs), q).some((h) => h.id === id),
      q,
    );
  }
});
