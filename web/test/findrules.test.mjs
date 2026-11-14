// Find in the panel editor (findrules.js): what can be searched, who may see it, and how results rank. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { pairPlan, chipKinds, findEntries, pageEntries, rankFind, fillRun, findPlan, applyFind, afterFill, findScope } from '../findrules.js';
import { createGrid, select, selCells, cellAt } from '../gridmodel.js';

const names = {
  numberOffset: 1,
  sources: [
    { n: 0, name: 'Camera 1' },
    { n: 1, name: 'Camera 2' },
    { n: 2, name: 'Graphics' },
  ],
  destinations: [
    { n: 0, name: 'Monitor A' },
    { n: 1, name: 'Monitor B' },
  ],
};
const presets = {
  presets: [
    { scope: 'everyone', name: 'Show open' },
    { scope: 'me', name: 'Night' },
  ],
};

test('every Source, Destination, Preset, Take, Clear, Undo, Current, Panel Name, Instance Name and Page cell kind is searchable, with the number a person sees', () => {
  const all = findEntries(names, presets, null);
  assert.equal(all.length, 18);
  assert.deepEqual(
    rankFind(all, 'take')
      .map((e) => e.category)
      .slice(0, 1),
    ['take'],
  );
  assert.deepEqual(rankFind(all, 'clear')[0].category, 'clear');
  assert.deepEqual(rankFind(all, 'undo')[0].category, 'undo');
  const cam = all.find((e) => e.label === 'Camera 2');
  assert.deepEqual([cam.category, cam.n, cam.num, cam.off], ['source', 1, 2, false]);
  const p = all.find((e) => e.label === 'Night');
  assert.deepEqual([p.category, p.scope, p.n], ['preset', 'me', undefined]);
});

test('Read-only rights grey what is outside them and Hidden removes it', () => {
  const rights = { sources: [0], dsts: [1], presets: ['Night'] };
  const grey = findEntries(names, presets, rights);
  assert.equal(grey.length, 18);
  assert.deepEqual(
    grey.filter((e) => e.off).map((e) => e.label),
    ['Camera 2', 'Graphics', 'Monitor A', 'Show open'],
  );
  const gone = findEntries(names, presets, { ...rights, visibility: 'hidden' });
  assert.deepEqual(
    gone.map((e) => e.label),
    [
      'Camera 1',
      'Monitor B',
      'Night',
      'Take',
      'Clear',
      'Undo',
      'Current Source',
      'Panel Name',
      'Instance Name',
      'Page Name',
      'Info',
      'Go to Page',
      'Next Page',
      'Previous Page',
    ],
  );
  assert.ok(gone.every((e) => !e.off));
});

test('a kind with no list in the rights allows everything of that kind', () => {
  const r = findEntries(names, presets, { sources: [2], visibility: 'hidden' });
  assert.deepEqual(
    r.filter((e) => e.category === 'source').map((e) => e.label),
    ['Graphics'],
  );
  assert.equal(r.filter((e) => e.category === 'destination').length, 2);
});

test('ranking puts an exact name first, then a prefix, a word start, a substring and a loose match', () => {
  const list = [
    { category: 'source', key: 'a', label: 'Backup cam', num: 5, n: 4, off: false },
    { category: 'source', key: 'b', label: 'Cam', num: 9, n: 8, off: false },
    { category: 'source', key: 'c', label: 'Camera 1', num: 1, n: 0, off: false },
    { category: 'source', key: 'd', label: 'Webcam', num: 7, n: 6, off: false },
    { category: 'source', key: 'e', label: 'Charm', num: 3, n: 2, off: false },
    { category: 'source', key: 'f', label: 'Lens', num: 4, n: 3, off: false },
  ];
  assert.deepEqual(
    rankFind(list, 'cam').map((e) => e.key),
    ['b', 'c', 'a', 'd', 'e'],
  );
});

test('every word must match, in any order, and a number matches what a person sees', () => {
  const list = findEntries(names, presets, null);
  assert.deepEqual(
    rankFind(list, 'mon b').map((e) => e.label),
    ['Monitor B'],
  );
  assert.deepEqual(
    rankFind(list, '2')
      .map((e) => e.label)
      .slice(0, 2),
    ['Camera 2', 'Monitor B'],
  );
});

test('chips narrow to categories, and with no text results come in number order by category', () => {
  const list = findEntries(names, presets, null);
  assert.deepEqual(
    rankFind(list, '', ['destination']).map((e) => e.label),
    ['Monitor A', 'Monitor B'],
  );
  assert.deepEqual(
    rankFind(list, '').map((e) => e.label),
    [
      'Camera 1',
      'Camera 2',
      'Graphics',
      'Monitor A',
      'Monitor B',
      'Night',
      'Show open',
      'Take',
      'Clear',
      'Undo',
      'Current Source',
      'Panel Name',
      'Instance Name',
      'Page Name',
      'Info',
      'Go to Page',
      'Next Page',
      'Previous Page',
    ],
  );
});

const find = (label, entries) => entries.find((e) => e.label === label);
const sel = (r1, c1, r2, c2) => select(select(createGrid({ rows: 4, cols: 4 }), r1, c1), r2, c2, true);

test('choosing a result fills from it onward in number order, of its own category', () => {
  const all = findEntries(names, presets, null);
  assert.deepEqual(
    fillRun(all, find('Camera 2', all), null).map((e) => e.label),
    ['Camera 2', 'Graphics'],
  );
  assert.deepEqual(
    fillRun(all, find('Night', all), null).map((e) => e.label),
    ['Night', 'Show open'],
  );
});

test('a fill skips what the rights grey out, and a greyed start fills nothing', () => {
  const all = findEntries(names, presets, { sources: [0, 2] });
  assert.deepEqual(
    fillRun(all, find('Camera 1', all), null).map((e) => e.label),
    ['Camera 1', 'Graphics'],
  );
  assert.deepEqual(fillRun(all, find('Camera 2', all), null), []);
});

test('a picked set fills in the order picked, of the first pick category, without greyed entries', () => {
  const all = findEntries(names, presets, { sources: [0, 2] });
  const picks = [find('Graphics', all), find('Monitor A', all), find('Camera 2', all), find('Camera 1', all)];
  assert.deepEqual(
    fillRun(all, picks[0], picks).map((e) => e.label),
    ['Graphics', 'Camera 1'],
  );
});

test('the plan puts the run across the selection, and applying it matches the plan', () => {
  const all = findEntries(names, presets, null);
  const g = sel(1, 1, 2, 2);
  const run = fillRun(all, find('Camera 1', all), null);
  const plan = findPlan(g, run, 'across');
  assert.deepEqual(
    plan.slots.map((s) => [s.r, s.c, s.entry.label, s.replace]),
    [
      [1, 1, 'Camera 1', false],
      [1, 2, 'Camera 2', false],
      [2, 1, 'Graphics', false],
    ],
  );
  const done = applyFind(g, plan);
  assert.deepEqual(
    plan.slots.map((s) => ({ r: s.r, c: s.c, kind: cellAt(done, s.r, s.c).kind, n: cellAt(done, s.r, s.c).n })),
    [
      { r: 1, c: 1, kind: 'source', n: 0 },
      { r: 1, c: 2, kind: 'source', n: 1 },
      { r: 2, c: 1, kind: 'source', n: 2 },
    ],
  );
  assert.equal(cellAt(done, 2, 2), undefined);
});

test('the fill order is a parameter', () => {
  const all = findEntries(names, presets, null);
  const g = sel(1, 1, 2, 2);
  const plan = findPlan(g, fillRun(all, find('Camera 1', all), null), 'down');
  assert.deepEqual(
    plan.slots.map((s) => [s.r, s.c]),
    [
      [1, 1],
      [2, 1],
      [1, 2],
    ],
  );
});

test('a Preset cell holds scope and name, and replacing one of another kind drops its old item', () => {
  const all = findEntries(names, presets, null);
  let g = sel(1, 1, 1, 2);
  g = applyFind(g, findPlan(g, fillRun(all, find('Camera 1', all), null), 'across'));
  const done = applyFind(g, findPlan(g, fillRun(all, find('Night', all), null), 'across'));
  assert.deepEqual(
    selCells(done).map(({ r, c, kind, scope, name, n }) => ({ r, c, kind, scope, name, n })),
    [
      { r: 1, c: 1, kind: 'preset', scope: 'me', name: 'Night', n: undefined },
      { r: 1, c: 2, kind: 'preset', scope: 'everyone', name: 'Show open', n: undefined },
    ],
  );
});

test('after a click on one selected square the selection moves to the next square across', () => {
  const g = select(createGrid({ rows: 2, cols: 3 }), 1, 3);
  const next = afterFill(g);
  assert.deepEqual(next.sel, { anchor: { r: 2, c: 1 }, head: { r: 2, c: 1 } });
  const last = select(createGrid({ rows: 2, cols: 3 }), 2, 3);
  assert.equal(afterFill(last), last);
});

test('a result outside the rights ranks below one inside them that matches as well', () => {
  const list = findEntries(names, presets, { sources: [1] });
  assert.deepEqual(
    rankFind(list, 'camera').map((e) => e.label),
    ['Camera 2', 'Camera 1'],
  );
});

test('Find starts narrowed to the kind the selected squares were given and not yet filled', () => {
  assert.equal(findScope([{ kind: 'preset' }]), 'preset');
  assert.equal(findScope([{ kind: 'source' }, { kind: 'source' }]), 'source');
  assert.equal(findScope([{ kind: 'destination' }]), 'destination');
  assert.equal(findScope([{ kind: 'source', n: 3 }]), '', 'a chosen Source is not narrowed');
  assert.equal(findScope([{ kind: 'preset', scope: 'everyone', name: 'Cam' }]), '');
  assert.equal(findScope([{ kind: 'source' }, { kind: 'source', n: 1 }]), '', 'one already filled');
  assert.equal(findScope([{ kind: 'source' }, { kind: 'preset' }]), '', 'mixed kinds');
  assert.equal(findScope([{ kind: 'take' }]), '', 'Take has no list to narrow');
  assert.equal(findScope([]), '', 'an empty square');
});

test('while Fixed destination is on, Find greys every Destination with its own reason and places none', async () => {
  const { offTag } = await import('../findrules.js');
  const list = findEntries(names, presets, { sources: [1] }, { fixed: true });
  const dsts = list.filter((e) => e.category === 'destination');
  assert.equal(dsts.length, 2);
  assert.ok(dsts.every((e) => e.off && offTag(e) === 'Fixed destination is on'));
  assert.equal(offTag(list.find((e) => e.label === 'Camera 1')), 'Outside rights');
  assert.equal(offTag(list.find((e) => e.label === 'Camera 2')), '');
  assert.deepEqual(fillRun(list, dsts[0], null), []);
  const hidden = findEntries(names, presets, { sources: [1], visibility: 'hidden' }, { fixed: true });
  assert.equal(hidden.filter((e) => e.category === 'destination').length, 2, 'hidden is about Rights only');
});

test('the fixed destination picker searches Destinations only, leaves out those chosen and honours Rights and Visibility', async () => {
  const { fixedPick } = await import('../findrules.js');
  const all = findEntries(names, presets, { dsts: [1] });
  assert.deepEqual(
    fixedPick(all, 'mon', []).map((e) => [e.label, e.off]),
    [
      ['Monitor B', false],
      ['Monitor A', true],
    ],
  );
  assert.deepEqual(
    fixedPick(all, '', [1]).map((e) => e.label),
    ['Monitor A'],
  );
  assert.deepEqual(
    fixedPick(findEntries(names, presets, { dsts: [1], visibility: 'hidden' }), 'mon', []).map((e) => e.label),
    ['Monitor B'],
  );
  assert.deepEqual(
    fixedPick(all, '2', []).map((e) => e.label),
    ['Monitor B'],
    'a typed number is the one a person sees',
  );
});

test('Pages are found by name and by number, and place Go to Page cells down the grid', () => {
  const pages = [{ id: 'aa' }, { id: 'bb', name: 'Studio B' }, { id: 'cc' }];
  const list = pageEntries(pages);
  assert.deepEqual(
    rankFind(list, 'studio').map((e) => e.id),
    ['bb'],
  );
  assert.deepEqual(
    rankFind(list, 'page 2').map((e) => e.id),
    ['bb'],
  );
  assert.deepEqual(
    rankFind(list, '3').map((e) => e.id),
    ['cc'],
  );
  const g = select(select(createGrid({ rows: 2, cols: 2, cells: [] }), 1, 1), 1, 2, true);
  const out = applyFind(g, findPlan(g, fillRun(list, list[1], null)));
  assert.deepEqual(
    out.cells.map((c) => [c.r, c.c, c.kind, c.page, c.n]),
    [
      [1, 1, 'goToPage', 'bb', undefined],
      [1, 2, 'goToPage', 'cc', undefined],
    ],
  );
});

test('the Pages chip holds the Pages and every Page kind, Pages first', () => {
  const list = [...findEntries({ sources: [], destinations: [] }, { presets: [] }, null), ...pageEntries([{ id: 'aa' }])];
  assert.deepEqual(
    rankFind(list, '', chipKinds('page')).map((e) => e.category),
    ['page', 'pageName', 'goToPage', 'nextPage', 'previousPage'],
  );
  assert.deepEqual(chipKinds('source'), ['source']);
  assert.deepEqual(chipKinds(''), []);
});

test('two squares side by side take Previous then Next Page, and one above the other too', () => {
  const all = findEntries({ sources: [], destinations: [] }, { presets: [] }, null);
  const next = all.find((e) => e.category === 'nextPage');
  const sel = (r2, c2) => select(select(createGrid({ rows: 3, cols: 3, cells: [] }), 1, 1), r2, c2, true);
  const kinds = (g) => applyFind(g, pairPlan(g, next, all)).cells.map((c) => [c.r, c.c, c.kind, c.n]);
  assert.deepEqual(kinds(sel(1, 2)), [
    [1, 1, 'previousPage', undefined],
    [1, 2, 'nextPage', undefined],
  ]);
  assert.deepEqual(kinds(sel(2, 1)), [
    [1, 1, 'previousPage', undefined],
    [2, 1, 'nextPage', undefined],
  ]);
  assert.equal(pairPlan(sel(2, 2), next, all), null);
  assert.equal(
    pairPlan(
      sel(1, 2),
      all.find((e) => e.category === 'take'),
      all,
    ),
    null,
  );
});

test('the pair also takes two merged cells, or a merged cell and an empty square', () => {
  const all = findEntries({ sources: [], destinations: [] }, { presets: [] }, null);
  const next = all.find((e) => e.category === 'nextPage');
  const cell = (r, c, rs, cs) => ({ r, c, rs, cs, kind: 'take' });
  const run = (cells, r2, c2) => {
    const g = select(select(createGrid({ rows: 4, cols: 4, cells }), 1, 1), r2, c2, true);
    return applyFind(g, pairPlan(g, next, all)).cells.map((c) => [c.r, c.c, c.rs, c.cs, c.kind]);
  };
  assert.deepEqual(run([cell(1, 1, 2, 1), cell(1, 2, 2, 1)], 2, 2), [
    [1, 1, 2, 1, 'previousPage'],
    [1, 2, 2, 1, 'nextPage'],
  ]);
  assert.deepEqual(run([cell(1, 1, 1, 2), cell(2, 1, 1, 2)], 2, 2), [
    [1, 1, 1, 2, 'previousPage'],
    [2, 1, 1, 2, 'nextPage'],
  ]);
  assert.deepEqual(run([cell(1, 1, 1, 2)], 1, 3), [
    [1, 1, 1, 2, 'previousPage'],
    [1, 3, 1, 1, 'nextPage'],
  ]);
});
