// The Settings sections are in index.html, so each later topic owns its sections in the page HTML.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');
const read = (f) => readFileSync(join(web, f), 'utf8');

const html = read('index.html');
const view = html.slice(html.indexOf('id="settings-view"'), html.indexOf('</form>', html.indexOf('id="settings-view"')));
const groups = [...view.matchAll(/<fieldset class="sode__group[^"]*"([^>]*)>\s*<legend>([^<]+)<\/legend>([\s\S]*?)<\/fieldset>/g)].map((m) => ({
  attrs: m[1],
  title: m[2],
  body: m[3],
}));
const group = (title) => groups.find((g) => g.title === title);

test('Users and API come right after Access, in that order', () => {
  const titles = groups.map((g) => g.title);
  const at = titles.indexOf('Access');
  assert.deepEqual(titles.slice(at, at + 3), ['Access', 'Users', 'API']);
});

test('the Users section holds Manage users and the profile list, saved apart from the form', () => {
  const g = group('Users');
  assert.match(g.attrs, /id="users-group"/);
  assert.match(g.attrs, /data-apart/);
  for (const id of ['users-manage', 'users-people']) assert.match(g.body, new RegExp(`id="${id}"`));
});

test('the API section holds its fields and its own Save, saved apart from the form', () => {
  const g = group('API');
  assert.match(g.attrs, /data-apart/);
  for (const id of ['api-enabled', 'api-guest', 'api-log-reads', 'api-rate', 'api-cap', 'api-save', 'api-note', 'api-err']) {
    assert.match(g.body, new RegExp(`id="${id}"`));
  }
});

test('no module adds a Settings section of its own', () => {
  const adders = readdirSync(web).filter((f) => f.endsWith('.js') && /el\(\s*'fieldset',\s*'sode__group'/.test(read(f)));
  assert.deepEqual(adders, []);
});
