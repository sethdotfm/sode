import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { choicesFor, hasChoice, newPick, importReq, criticalNames, setCat, catState, catSize, reviewCounts, missingText, brokenItems } from '../importrules.js';

// A same-install file as POST api/import answers it: everything preselected Use new.
const check = () => ({
  sameInstall: true,
  categories: ['settings', 'routers', 'users', 'presets', 'panels', 'browser'],
  plan: {
    sameInstall: true,
    critical: true,
    categories: {
      settings: {
        items: [
          { key: 'branding', name: 'Branding', clash: 'Branding', choice: 'new', size: 40 },
          { key: 'locks', name: 'Locks', same: true, size: 20 },
        ],
      },
      routers: { items: [{ key: 'r1', name: 'Studio A', clash: 'Studio A', choice: 'new', size: 300 }] },
      users: { items: [{ key: 'u1', name: 'Bo', clash: 'Bo', choice: 'new', size: 9 }] },
      presets: {
        items: [
          { key: 'p1', name: 'Show', clash: 'Show live', choice: 'new', size: 100 },
          { key: 'p2', name: 'Open', size: 50, broken: [{ kind: 'source', ref: { router: 'x', n: 2, name: 'CAM 3' } }] },
        ],
      },
      panels: { refused: 'the router names are not read yet', items: [{ key: 'd1', name: 'Desk', clash: 'Desk', choice: 'new', size: 70 }] },
    },
  },
});

test('Rename is offered only where the server can rename', () => {
  assert.deepEqual(
    choicesFor('presets').map(([, t]) => t),
    ['Use new', 'Use current', 'Skip', 'Rename'],
  );
  assert.deepEqual(
    choicesFor('settings').map(([v]) => v),
    ['new', 'current', 'skip'],
  );
  assert.deepEqual(
    choicesFor('routers').map(([v]) => v),
    ['new', 'current', 'skip'],
  );
});

test('only a live clash in a category that imports asks for a choice', () => {
  const c = check();
  const cats = c.plan.categories;
  assert.equal(hasChoice(cats.presets, cats.presets.items[0]), true);
  assert.equal(hasChoice(cats.presets, cats.presets.items[1]), false, 'no clash');
  assert.equal(hasChoice(cats.settings, cats.settings.items[1]), false, 'equal to the live item');
  assert.equal(hasChoice(cats.panels, cats.panels.items[0]), false, 'refused category');
  assert.equal(hasChoice(cats.users, { key: 'u', name: 'X', clash: 'X', choice: 'new', refused: 'no' }), false, 'refused item');
});

test('everything is ticked but Routers and This browser, with the preselected choices', () => {
  const pick = newPick(check());
  assert.deepEqual(
    Object.keys(pick.items).filter((c) => pick.items[c].size),
    ['settings', 'users', 'presets'],
  );
  assert.deepEqual([...pick.items.presets], ['p1', 'p2']);
  assert.equal(pick.choices.presets.p1, 'new');
  assert.equal(catState(check(), pick, 'routers'), 'none');
  assert.equal(catState(check(), pick, 'panels'), 'none', 'a refused category cannot be ticked');
});

test('the request names the ticked categories, items, sections and choices', () => {
  const c = check();
  const pick = newPick(c);
  pick.items.presets.delete('p2');
  pick.choices.presets.p1 = 'rename';
  pick.items.settings.delete('locks');
  setCat(c, pick, 'users', false);
  assert.deepEqual(importReq(pick), {
    mode: 'add',
    categories: ['settings', 'presets'],
    sections: ['branding'],
    pick: { presets: ['p1'] },
    choices: { settings: { branding: 'new' }, presets: { p1: 'rename' } },
  });
  // A category with nothing ticked is not sent: an empty pick would mean everything to the server.
  setCat(c, pick, 'settings', false);
  pick.items.presets.clear();
  assert.deepEqual(importReq(pick).categories, []);
});

test('Replace import sends no choices, and Keep current Administrators only when it is off', () => {
  const c = check();
  const pick = newPick(c);
  pick.mode = 'replace';
  assert.deepEqual(importReq(pick).choices, {});
  assert.equal(importReq(pick).mode, 'replace');
  assert.equal('keepAdmins' in importReq(pick), false);
  pick.keepAdmins = false;
  assert.equal(importReq(pick).keepAdmins, false);
  assert.equal(hasChoice(c.plan.categories.presets, c.plan.categories.presets.items[0], pick), false);
});

test('a Replace plan counts what it removes', () => {
  const plan = {
    categories: {
      presets: {
        items: [
          { key: 'p1', name: 'Show', clash: 'Show', choice: 'new' },
          { key: 'p9', name: 'Late', remove: true },
        ],
      },
    },
  };
  assert.deepEqual(reviewCounts(plan), [{ cat: 'presets', text: '1 updated, 1 removed' }]);
});

test('Use new on a clash makes the import Critical; other choices do not', () => {
  const c = check();
  const pick = newPick(c);
  assert.deepEqual(criticalNames(c, pick), ['Branding', 'Bo', 'Show']);
  pick.choices.presets.p1 = 'rename';
  pick.choices.settings.branding = 'current';
  pick.choices.users.u1 = 'skip';
  assert.deepEqual(criticalNames(c, pick), []);
  pick.choices.presets.p1 = 'new';
  pick.items.presets.delete('p1');
  assert.deepEqual(criticalNames(c, pick), [], 'an unticked item writes nothing');
  setCat(c, pick, 'routers', true);
  assert.deepEqual(criticalNames(c, pick), ['Studio A'], 'a ticked router');
});

test('category size counts the ticked items', () => {
  const c = check();
  const pick = newPick(c);
  assert.equal(catSize(c, pick, 'presets'), 150);
  pick.items.presets.delete('p1');
  assert.equal(catSize(c, pick, 'presets'), 50);
  assert.equal(catSize(c, pick, 'routers'), 0);
});

test('the review counts what each category does', () => {
  const plan = check().plan;
  plan.categories.presets.items[0].choice = 'rename';
  plan.categories.presets.items[0].as = 'Show 2';
  plan.categories.users.items[0].choice = 'skip';
  assert.deepEqual(reviewCounts(plan), [
    { cat: 'settings', text: '1 updated, 1 no change' },
    { cat: 'routers', text: '1 updated' },
    { cat: 'users', text: '1 kept' },
    { cat: 'presets', text: '2 added, 1 Broken' },
    { cat: 'panels', text: 'Not imported: the router names are not read yet' },
  ]);
});

test('Broken lines name what is missing', () => {
  const plan = check().plan;
  assert.equal(missingText({ kind: 'source', ref: { n: 2, name: 'CAM 3' } }, 1), 'source CAM 3 (3)');
  assert.equal(missingText({ kind: 'destination', ref: { n: 0, name: '' } }, 1), 'destination 1');
  assert.equal(missingText({ kind: 'level', ref: { n: 4 } }, 1), 'level 4');
  assert.equal(missingText({ kind: 'preset', ref: { n: 0, name: 'Show' } }, 1), 'preset Show');
  assert.equal(missingText({ kind: 'router', ref: { n: 0, router: 'abc' } }, 1), 'router abc');
  assert.deepEqual(
    brokenItems(plan).map((x) => x.name),
    ['Open'],
  );
});

test('the module is pure', () => {
  const src = readFileSync(new URL('../importrules.js', import.meta.url), 'utf8');
  assert.doesNotMatch(src, /^import /m);
});
