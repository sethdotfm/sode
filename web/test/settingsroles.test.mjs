// Settings role tags: who edits each section and whether lower roles see it.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { SECTION_ROLES, sectionView, approvalWho } from '../settingsrolerules.js';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');
const html = readFileSync(join(web, 'index.html'), 'utf8');
const view = html.slice(html.indexOf('id="settings-view"'), html.indexOf('</form>', html.indexOf('id="settings-view"')));
const slugs = [...new Set([...view.matchAll(/<fieldset class="sode__group[^"]*"[^>]*\bdata-section="([^"]+)"/g)].map((m) => m[1]))];

test('every Settings section has a role tag, and no tag names a missing section', () => {
  assert.deepEqual(Object.keys(SECTION_ROLES).sort(), [...slugs].sort());
});

test('the tags are the agreed table', () => {
  const by = {};
  for (const [slug, t] of Object.entries(SECTION_ROLES)) (by[`${t.edit} ${t.lower}`] ||= []).push(slug);
  assert.deepEqual(by, {
    'anyone shown': ['this-browser', 'my-user'],
    'trusted shown': ['everyone-defaults', 'branding', 'history'],
    'administrator shown': ['level-colors', 'confirm', 'routers'],
    'administrator hidden': ['tie-lines', 'protects', 'locks', 'users', 'api', 'panels', 'proxy', 'export-import', 'run-setup-again'],
  });
});

const views = (role, phone = false) => Object.fromEntries(slugs.map((s) => [s, sectionView(s, role, phone)]));
const pick = (v, want) =>
  Object.entries(v)
    .filter(([, x]) => x === want)
    .map(([s]) => s);

test('a Standard user sees Shown sections read-only and does not see Access or Server', () => {
  const v = views('standard');
  assert.deepEqual(pick(v, 'edit'), ['this-browser', 'my-user']);
  assert.deepEqual(pick(v, 'read'), ['everyone-defaults', 'branding', 'history', 'level-colors', 'confirm', 'routers']);
  assert.deepEqual(pick(v, 'hidden'), ['tie-lines', 'protects', 'locks', 'users', 'api', 'panels', 'proxy', 'export-import', 'run-setup-again']);
});

test('a Trusted user edits Everyone’s defaults, Branding and History, and reads the Administrator Shown sections', () => {
  const v = views('trusted');
  assert.deepEqual(pick(v, 'edit'), ['this-browser', 'my-user', 'everyone-defaults', 'branding', 'history']);
  assert.deepEqual(pick(v, 'read'), ['level-colors', 'confirm', 'routers']);
  assert.deepEqual(pick(v, 'hidden'), ['tie-lines', 'protects', 'locks', 'users', 'api', 'panels', 'proxy', 'export-import', 'run-setup-again']);
});

test('an Administrator edits every section', () => {
  assert.deepEqual(pick(views('administrator'), 'edit'), slugs);
});

test('no Role (a held Lock) or an unknown one is no more than a Standard user', () => {
  assert.deepEqual(views(''), views('standard'));
  assert.deepEqual(views('owner'), views('standard'));
});

test('the phone shows only the Preferences sections, whatever the Role', () => {
  for (const role of ['standard', 'trusted', 'administrator']) {
    const v = views(role, true);
    assert.deepEqual(pick(v, 'edit'), ['this-browser', 'my-user'], role);
    assert.equal(pick(v, 'read').length, 0, role);
  }
});

test('an unknown section is hidden', () => {
  assert.equal(sectionView('no-such-section', 'administrator'), 'hidden');
});

test('the Approval names the highest Role its sections need', () => {
  assert.equal(approvalWho(['branding', 'history']), 'a Trusted user or an Administrator');
  assert.equal(approvalWho(['branding', 'confirm']), 'an Administrator');
});
