import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');
const read = (f) => readFileSync(join(web, f), 'utf8');

const html = read('index.html');
const view = html.slice(html.indexOf('id="settings-view"'), html.indexOf('</form>', html.indexOf('id="settings-view"')));
const attr = (attrs, name) => attrs.match(new RegExp(`\\b${name}="([^"]*)"`))?.[1];
const groups = [...view.matchAll(/<fieldset class="sode__group[^"]*"([^>]*)>\s*<legend>([^<]+)<\/legend>([\s\S]*?)<\/fieldset>/g)].map((m) => ({
  attrs: m[1],
  title: m[2],
  slug: attr(m[1], 'data-section'),
  topic: attr(m[1], 'data-topic'),
  router: attr(m[1], 'data-router'),
  body: m[3],
}));
const group = (title) => groups.find((g) => g.title === title);
// Topic bands and router sub-headings, in page order.
const heads = [...view.matchAll(/<(h3|h4) class="(sode__topic|sode__router)"([^>]*)>([\s\S]*?)<\/h[34]>/g)].map((m) => ({
  kind: m[2] === 'sode__topic' ? 'topic' : 'router',
  id: attr(m[3], m[2] === 'sode__topic' ? 'data-topic' : 'data-router'),
  title: m[4].replace(/<[^>]+>/g, '').trim(),
}));

test('five topics, each with a band, in the agreed order', () => {
  assert.deepEqual(
    heads.filter((h) => h.kind === 'topic').map((h) => [h.id, h.title]),
    [
      ['preferences', 'Preferences'],
      ['look-and-feel', 'Look and feel'],
      ['routers', 'Routers'],
      ['access', 'Access'],
      ['server', 'Server'],
    ],
  );
});

test('each topic holds its sections in order', () => {
  const byTopic = {};
  for (const g of groups) (byTopic[g.topic] ||= []).push(g.slug);
  assert.deepEqual(byTopic, {
    preferences: ['this-browser', 'my-user'],
    'look-and-feel': ['everyone-defaults', 'branding', 'history', 'level-colors', 'confirm'],
    routers: ['routers', 'router-levels', 'router-matrix', 'router-names', 'takes', 'protects'],
    access: ['locks', 'users', 'api', 'panels'],
    server: ['proxy', 'export-import', 'run-setup-again'],
  });
  const order = [...view.matchAll(/data-topic="([^"]+)"/g)].map((m) => m[1]);
  const firsts = [...new Set(order)];
  assert.deepEqual(firsts, ['preferences', 'look-and-feel', 'routers', 'access', 'server']);
  for (let i = 1; i < order.length; i++) assert.ok(firsts.indexOf(order[i]) >= firsts.indexOf(order[i - 1]), `section out of its topic at ${i}`);
});

test('the router block has a sub-heading and holds the per-router sections', () => {
  const routers = heads.filter((h) => h.kind === 'router');
  assert.equal(routers.length, 1);
  const inBlock = groups.filter((g) => g.router === routers[0].id).map((g) => g.slug);
  assert.deepEqual(inBlock, ['router-matrix', 'router-names', 'takes', 'protects']);
  const ids = (slug) => [...groups.find((g) => g.slug === slug).body.matchAll(/\bid="(cfg-[a-z0-9-]+)"/g)].map((m) => m[1]);
  for (const id of ['cfg-router', 'cfg-routername', 'cfg-routershow', 'cfg-matrix', 'cfg-levels', 'cfg-offset', 'cfg-lvlnum', 'cfg-numoff'])
    assert.ok(ids('router-matrix').includes(id), id);
  for (const id of ['cfg-namerefresh', 'cfg-namelen', 'cfg-namereq']) assert.ok(ids('router-names').includes(id), id);
  assert.deepEqual(ids('takes'), ['cfg-default', 'cfg-resync', 'cfg-firemode', 'cfg-salvogroup']);
  assert.deepEqual(ids('protects'), ['cfg-protectdevice']);
  assert.deepEqual(ids('confirm'), ['cfg-lockout', 'cfg-dismiss', 'cfg-enter']);
});

test('the Locks section holds the page locks and auto-lock', () => {
  const g = group('Locks');
  for (const id of ['cfg-lock-main', 'cfg-lock-log', 'cfg-lock-panel', 'cfg-auto-lock', 'cfg-pin-waits', 'cfg-guest-role'])
    assert.match(g.body, new RegExp(`id="${id}"`));
});

test('the Users section holds Manage users, New user defaults (saved with the form) and the profile list', () => {
  const g = group('Users');
  assert.match(g.attrs, /id="users-group"/);
  assert.doesNotMatch(g.attrs, /data-apart/);
  for (const id of ['users-manage', 'cfg-new-persistent', 'cfg-new-no-auto-lock', 'users-people']) assert.match(g.body, new RegExp(`id="${id}"`));
});

test('the API section holds its fields and no Save of its own: they save with the one Save', () => {
  const g = group('API');
  assert.doesNotMatch(g.attrs, /data-apart/);
  for (const id of ['api-guest', 'api-scopes', 'api-log-reads', 'api-rate', 'api-cap']) {
    assert.match(g.body, new RegExp(`id="${id}"`));
  }
  assert.doesNotMatch(g.body, /id="api-save"/);
  assert.doesNotMatch(g.body, /id="api-enabled"/, 'the API is always on');
});

test('one Export & Import section opens the Export & Import dialog', () => {
  const g = group('Export & Import');
  assert.match(g.body, /id="xi-open"/);
  assert.doesNotMatch(g.body, /config-file|transfer-open/);
  assert.equal(groups.filter((x) => /export|backup/i.test(x.title)).length, 1);
});

test('no module adds a Settings section of its own', () => {
  const adders = readdirSync(web).filter((f) => f.endsWith('.js') && /el\(\s*'fieldset',\s*'sode__group'/.test(read(f)));
  assert.deepEqual(adders, []);
});

test('Preferences labels are the PREFS labels', async () => {
  const { PREFS, PREF_LABELS } = await import('../prefs.js');
  const found = [...html.matchAll(/<label for="([\w-]+)">([^<]+)<\/label>\s*<span class="pb-select-wrap"><select id="\1"[^>]*data-g?pref="(\w+)"/g)];
  assert.equal(found.length, 2 * Object.keys(PREFS).length);
  for (const [, , text, key] of found) assert.equal(text, PREF_LABELS[key], key);
});
