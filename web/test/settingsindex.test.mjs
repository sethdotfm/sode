import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { DICT, buildIndex, find, hitWhere } from '../settingsindex.js';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');
const read = (f) => readFileSync(join(web, f), 'utf8');

// The sections as the page registers them: the settings view's fieldsets, with their slug and topic.
function sections() {
  const html = read('index.html');
  const view = html.slice(html.indexOf('id="settings-view"'));
  const topics = Object.fromEntries([...view.matchAll(/<h3 class="sode__topic"[^>]*data-topic="([^"]+)"[^>]*>([^<]+)<\/h3>/g)].map((m) => [m[1], m[2].trim()]));
  return [...view.matchAll(/<fieldset class="sode__group[^"]*"([^>]*)>\s*<legend>([^<]+)<\/legend>/g)].map((m) => {
    const topicId = m[1].match(/data-topic="([^"]+)"/)?.[1];
    return { id: m[1].match(/data-section="([^"]+)"/)?.[1], title: m[2], topicId, topic: topics[topicId] };
  });
}

const idx = () =>
  buildIndex(
    sections()
      .map((s) => ({ ...s, settings: [] }))
      .concat([
        {
          id: 'locks',
          title: 'Locks',
          settings: [{ id: 'cfg-pin-waits', label: 'Wait after wrong PINs' }],
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

test('every registered section has dictionary terms', () => {
  const secs = sections();
  assert.ok(secs.length >= 12, 'found the sections: ' + secs.map((s) => s.title).join(', '));
  for (const s of secs) {
    assert.ok(s.id, `${s.title} needs data-section`);
    assert.ok(s.topic, `${s.title} needs data-topic of a topic band`);
    assert.ok(DICT[s.id]?.length, `add DICT['${s.id}'] in settingsindex.js`);
  }
});

// my-default-* buttons come from devicesettings.js and open-log from about.js.
test('every DICT key names a section or control on the page', () => {
  const html = read('index.html');
  for (const k of Object.keys(DICT).filter((k) => !k.startsWith('my-default-') && k !== 'open-log')) {
    assert.match(html, new RegExp(`(id|data-find|data-section)="${k}"`), k);
  }
});

test('common words reach their section', () => {
  const cases = {
    dark: 'this-browser',
    clock: 'this-browser',
    ip: 'router-matrix',
    host: 'router-matrix',
    poll: 'router-names',
    lockout: 'confirm',
    'success banner': 'confirm',
    offset: 'router-matrix',
    undo: 'history',
    audit: 'proxy',
    proxy: 'proxy',
    brand: 'branding',
    border: 'branding',
    salvo: 'salvos',
  };
  for (const [q, sec] of Object.entries(cases)) {
    const hits = find(buildIndex([...sections(), { id: 'salvos', title: 'Salvos' }]), q);
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
      hits.some((h) => h.sectionId === 'users' || h.sectionId === 'locks'),
      q,
    );
  }
});

test('settings are found by label and by dictionary', () => {
  assert.equal(top('wrong pins').id, 'cfg-pin-waits');
  assert.equal(top('communication log').id, 'open-log');
  assert.equal(top('uptime').id, 'open-log');
  assert.equal(top('wipe').id, 'clear-history');
});

test('the router salvo group is found by its words', () => {
  const index = buildIndex([
    {
      id: 'router',
      title: 'Router',
      settings: [{ id: 'cfg-salvogroup', label: 'Router salvo group' }],
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
      settings: [{ id: 'cfg-firemode', label: 'Multi destination' }],
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
  assert.equal(top('pasword').sectionId === 'locks' || top('pasword').sectionId === 'users', true);
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

test('every result names its topic', () => {
  const index = buildIndex(sections().map((s) => ({ ...s, settings: s.id === 'confirm' ? [{ id: 'cfg-lockout', label: 'Confirm lockout (ms)' }] : [] })));
  const hit = find(index, 'confirm lockout')[0];
  assert.equal(hit.id, 'cfg-lockout');
  assert.equal(hit.topic, 'Look and feel');
  for (const [q, topic] of [
    ['proxy', 'Server'],
    ['dark', 'Preferences'],
    ['ip', 'Routers'],
    ['users', 'Access'],
  ]) {
    assert.equal(find(index, q)[0]?.topic, topic, q);
  }
});

test("a topic name finds that topic's sections", () => {
  const hits = find(buildIndex(sections()), 'look and feel');
  assert.equal(hits.length, 5);
  assert.ok(hits.every((h) => h.topic === 'Look and feel'));
});

test('hitWhere names the topic, and the router for a per-router section', () => {
  assert.equal(hitWhere({ kind: 'section', topic: 'Server', section: 'Proxy' }), 'Server');
  assert.equal(hitWhere({ kind: 'setting', topic: 'Server', section: 'Proxy' }), 'Server › Proxy');
  assert.equal(hitWhere({ kind: 'section', topic: 'Routers', router: 'Ultrix', section: 'Takes' }), 'Routers › Ultrix');
  assert.equal(hitWhere({ kind: 'setting', topic: 'Routers', router: 'Ultrix', section: 'Takes' }), 'Routers › Ultrix › Takes');
});
