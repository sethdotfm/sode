import test from 'node:test';
import assert from 'node:assert/strict';
import {
  buildExport,
  matchEnd,
  matchItem,
  matchPreset,
  conflict,
  renameFor,
  applyChoices,
  defaultChoice,
  renameProblem,
  sizeProblem,
  jsonBytes,
  itemIn,
  SOCKET_LIMIT,
  saveAsName,
  filterOf,
} from '../exportfmt.js';

const live = {
  srcs: [
    { n: 0, name: 'CAM 1' },
    { n: 1, name: 'CAM 2' },
    { n: 2, name: 'BARS' },
    { n: 3, name: 'DUP' },
    { n: 4, name: 'dup' },
  ],
  dsts: [
    { n: 0, name: 'MV 1' },
    { n: 1, name: 'MV 2' },
    { n: 5, name: 'REC' },
  ],
  levelCount: 3,
};

test('buildExport carries names, omits blanks', () => {
  const doc = buildExport({
    prefs: { theme: 'dark' },
    site: 'Studio A',
    user: '',
    now: Date.UTC(2026, 10, 3, 12),
    filterSets: [{ name: 'MVs', f: { src: '', dst: 'MV', sode: true, ext: false, hidden: false } }],
    presets: [{ name: 'Open', items: [{ src: 1, dsts: [0, 9], levels: [1, 2] }] }],
    srcName: (n) => live.srcs.find((x) => x.n === n)?.name || '',
    dstName: (n) => live.dsts.find((x) => x.n === n)?.name || '',
  });
  assert.equal(doc.sode, 1);
  assert.equal(doc.site, 'Studio A');
  assert.ok(!('user' in doc));
  assert.equal(doc.exported, '2026-11-03T12:00:00Z');
  assert.deepEqual(doc.prefs, { theme: 'dark' });
  assert.deepEqual(doc.filterSets[0], { name: 'MVs', src: '', dst: 'MV', sode: true, ext: false, hidden: false });
  assert.deepEqual(doc.presets[0].items[0], { src: 1, srcName: 'CAM 2', dsts: [{ n: 0, name: 'MV 1' }, { n: 9 }], levels: [1, 2] });
  const bare = buildExport({ now: 0 });
  assert.deepEqual(Object.keys(bare), ['sode', 'exported']);
});

test('matching: number and name agree', () => {
  assert.deepEqual(matchEnd(1, 'cam 2 ', live.srcs), { status: 'same', n: 1 });
});

test('matching: unique name elsewhere remaps the number', () => {
  const m = matchEnd(7, 'CAM 2', live.srcs);
  assert.deepEqual(m, { status: 'byName', n: 1, was: 7 });
  // the number exists but names a different thing: the name wins
  assert.equal(matchEnd(0, 'CAM 2', live.srcs).n, 1);
});

test('matching: a name on several is ambiguous, never the first', () => {
  const m = matchEnd(9, 'dup', live.srcs);
  assert.equal(m.status, 'ambiguous');
  assert.equal(m.n, 9);
  assert.equal(m.count, 2);
  // number + name still agree when the number is one of them
  assert.equal(matchEnd(4, 'dup', live.srcs).status, 'same');
});

test('matching: neither is unmatched; a missing name falls back to the number', () => {
  assert.equal(matchEnd(1, 'GONE', live.srcs).status, 'unmatched');
  assert.equal(matchEnd(2, '', live.srcs).status, 'same');
  assert.equal(matchEnd(99, '', live.srcs).status, 'unmatched');
});

test('matchItem: worst status, remapped numbers, levels above the count', () => {
  const it = itemIn({
    src: 7,
    srcName: 'CAM 2',
    dsts: [
      { n: 0, name: 'MV 1' },
      { n: 8, name: 'NOPE' },
    ],
    levels: [1, 5],
  });
  const m = matchItem(it, live);
  assert.equal(m.status, 'unmatched');
  assert.deepEqual(m.saved, { src: 1, dsts: [0, 8], levels: [1, 5] });
  assert.deepEqual(m.levelsOver, [5]);
  assert.equal(m.src.status, 'byName');
});

test('matchPreset summarises its items', () => {
  const p = matchPreset(
    {
      name: 'P',
      items: [
        { src: 0, srcName: 'CAM 1', dsts: [{ n: 0, name: 'MV 1' }], levels: [] },
        { src: 9, srcName: 'dup', dsts: [{ n: 5, name: 'REC' }], levels: [4] },
      ],
    },
    live,
  );
  assert.equal(p.status, 'ambiguous');
  assert.equal(p.count.ambiguous, 1);
  assert.deepEqual(p.levelsOver, [4]);
  assert.equal(p.saved.items.length, 2);
});

test('conflicts ignore case; renames are unique and at most 32 runes', () => {
  assert.equal(conflict('show open', ['Show Open']), 'Show Open');
  assert.equal(conflict('x', ['Show Open']), null);
  assert.equal(renameFor('Show', ['Show']), 'Show 2');
  assert.equal(renameFor('Show', ['Show', 'show 2']), 'Show 3');
  const long = 'é'.repeat(32);
  const r = renameFor(long, [long]);
  assert.ok([...r].length <= 32);
  assert.ok(r.endsWith(' 2'));
  assert.equal(defaultChoice('x'), 'both');
  assert.equal(defaultChoice(null), 'import');
  assert.match(renameProblem('', []), /required/);
  assert.match(renameProblem('a'.repeat(33), []), /at most 32/);
  assert.match(renameProblem('Show', ['SHOW']), /already in use/);
  assert.equal(renameProblem('Show 2', ['Show']), '');
});

test('applyChoices: replace in place, keep both renamed, skip, unique among rows', () => {
  const existing = [
    { name: 'A', v: 1 },
    { name: 'B', v: 2 },
  ];
  const incoming = [
    { name: 'a', v: 10 },
    { name: 'B', v: 20 },
    { name: 'C', v: 30 },
    { name: 'b', v: 40 },
    { name: 'D', v: 50 },
  ];
  const r = applyChoices(existing, incoming, [
    { choice: 'replace' },
    { choice: 'both', as: 'B copy' },
    { choice: 'import' },
    { choice: 'both' },
    { choice: 'skip' },
  ]);
  assert.deepEqual(
    r.list.map((x) => `${x.name}:${x.v}`),
    ['a:10', 'B:2', 'B copy:20', 'C:30', 'b 2:40'],
  );
  assert.equal(r.replaced, 1);
  assert.equal(r.added, 3);
  assert.equal(r.skipped, 1);
  assert.deepEqual(existing[0], { name: 'A', v: 1 }, 'input untouched');
});

test('applyChoices: Use current keeps the item here and adds nothing', () => {
  const r = applyChoices([{ name: 'A', v: 1 }], [{ name: 'A', v: 2 }], [{ choice: 'current' }]);
  assert.deepEqual(
    r.list.map((x) => `${x.name}:${x.v}`),
    ['A:1'],
  );
  assert.equal(r.added, 0);
});

test('applyChoices never creates a duplicate even when the choice is import', () => {
  const r = applyChoices([{ name: 'A' }], [{ name: 'A' }], [{ choice: 'import' }]);
  assert.deepEqual(
    r.list.map((x) => x.name),
    ['A', 'A 2'],
  );
});

test('size check refuses what the socket or the profile would', () => {
  assert.equal(sizeProblem('local', { presets: [] }), '');
  const big = Array.from({ length: 700 }, (_, i) => ({ name: 'P' + i, items: [{ src: i, dsts: Array.from({ length: 40 }, (_, k) => k), levels: [] }] }));
  assert.ok(jsonBytes(big) > SOCKET_LIMIT);
  assert.match(sizeProblem('global', { presets: big }), /Too big for one save .* fewer presets/);
  const mid = big.slice(0, 350);
  assert.ok(jsonBytes(mid) > 48 * 1024 && jsonBytes(mid) < SOCKET_LIMIT);
  assert.equal(sizeProblem('global', { presets: mid }), '');
  assert.match(sizeProblem('user', { presets: mid }), /profile holds at most 48 KiB/);
});

test('saveAsName suggests Name 2, or (mine) for Everyone, within 32 characters', () => {
  assert.equal(saveAsName('Open', 'local'), 'Open 2');
  assert.equal(saveAsName('Open', 'global'), 'Open (mine)');
  assert.equal(saveAsName('  ', 'local'), '');
  assert.equal(saveAsName('', ''), '');
  for (const sc of ['local', 'global']) assert.ok([...saveAsName('x'.repeat(40), sc)].length <= 32);
});

test('a filter set that leaves out sode and ext shows them', () => {
  assert.deepEqual(filterOf({ name: 'Cams', src: '1-8' }), { src: '1-8', dst: '', sode: true, ext: true, hidden: false });
});
