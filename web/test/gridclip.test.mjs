// The editor's clipboard block (gridclip.js): copy, read and paste rules. Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { createGrid, select } from '../gridmodel.js';
import { copyBlock, blockText, readBlock, pastePlan, applyPaste, pasteNote, CLIP_MARK, CLIP_VERSION } from '../gridclip.js';
import { GRID_CHORDS } from '../gridchords.js';

const grid = (cells, size = { rows: 4, cols: 8 }) => createGrid({ ...size, cells });
const pick = (g, r1, c1, r2 = r1, c2 = c1) => select(select(g, r1, c1), r2, c2, true);

test('copy, cut and paste are in the chord table for Cmd and Ctrl', () => {
  for (const m of ['ctrl', 'meta']) {
    assert.equal(GRID_CHORDS[`${m}+x`], 'cut');
    assert.equal(GRID_CHORDS[`${m}+c`], 'copy');
    assert.equal(GRID_CHORDS[`${m}+v`], 'paste');
  }
});

test('a copy holds the selected cells whole, relative to the selection, with every field', () => {
  const g = grid([
    { r: 2, c: 2, rs: 1, cs: 2, kind: 'source', n: 5, caption: 'Cam 1', levels: [1, 2], art: 'icon:camera', artB: '2', fontSize: 120, hideText: true },
    { r: 3, c: 3, rs: 1, cs: 1, kind: 'take' },
    { r: 1, c: 8, rs: 1, cs: 1, kind: 'clear' },
  ]);
  const b = copyBlock(pick(g, 2, 2, 3, 4), 'p-1');
  assert.equal(b.sode, CLIP_MARK);
  assert.equal(b.version, CLIP_VERSION);
  assert.equal(b.panel, 'p-1');
  assert.deepEqual([b.rows, b.cols], [2, 3]);
  assert.deepEqual(b.cells, [
    { r: 1, c: 1, rs: 1, cs: 2, kind: 'source', n: 5, caption: 'Cam 1', levels: [1, 2], art: 'icon:camera', artB: '2', fontSize: 120, hideText: true },
    { r: 2, c: 2, rs: 1, cs: 1, kind: 'take' },
  ]);
});

test('a selection that cuts a merged cell grows to hold it, so the copy never splits one', () => {
  const g = grid([{ r: 1, c: 1, rs: 2, cs: 2, kind: 'take' }]);
  const b = copyBlock(pick(g, 2, 2), '');
  assert.deepEqual([b.rows, b.cols], [2, 2]);
  assert.deepEqual(b.cells, [{ r: 1, c: 1, rs: 2, cs: 2, kind: 'take' }]);
});

test('nothing to copy without a selection or without a cell in it', () => {
  assert.equal(copyBlock(grid([]), ''), null);
  assert.equal(copyBlock(pick(grid([]), 1, 1, 2, 2), ''), null);
});

test('the block survives its text form', () => {
  const g = grid([{ r: 1, c: 1, rs: 1, cs: 1, kind: 'preset', scope: 'me', name: 'Show' }]);
  const b = copyBlock(pick(g, 1, 1), 'p-1');
  assert.deepEqual(readBlock(blockText(b)), b);
});

test('anything but a Sode block of this version reads as null', () => {
  const good = { sode: CLIP_MARK, version: CLIP_VERSION, panel: '', rows: 1, cols: 2, cells: [{ r: 1, c: 1, rs: 1, cs: 1, kind: 'take' }] };
  const bad = [
    '',
    'hello',
    'Cam 1\tCam 2',
    '42',
    'null',
    '[]',
    JSON.stringify({ ...good, sode: 'other' }),
    JSON.stringify({ ...good, version: CLIP_VERSION + 1 }),
    JSON.stringify({ ...good, rows: 0 }),
    JSON.stringify({ ...good, cols: 99 }),
    JSON.stringify({ ...good, cells: 'x' }),
    JSON.stringify({ ...good, cells: [] }),
    JSON.stringify({ ...good, cells: [{ r: 1, c: 1, rs: 1, cs: 1, kind: 'rocket' }] }),
    JSON.stringify({ ...good, cells: [{ r: 1, c: 2, rs: 1, cs: 2, kind: 'take' }] }),
    JSON.stringify({ ...good, cells: [{ r: 0, c: 1, rs: 1, cs: 1, kind: 'take' }] }),
    JSON.stringify({ ...good, cells: [{ r: 1, c: 1.5, rs: 1, cs: 1, kind: 'take' }] }),
    JSON.stringify({
      ...good,
      cells: [
        { r: 1, c: 1, rs: 1, cs: 2, kind: 'take' },
        { r: 1, c: 2, rs: 1, cs: 1, kind: 'clear' },
      ],
    }),
    JSON.stringify(good) + 'x',
    JSON.stringify({ ...good, panel: 'x'.repeat(10) }).padEnd(2_000_000, ' '),
  ];
  for (const text of bad) assert.equal(readBlock(text), null, text.slice(0, 80));
  assert.deepEqual(readBlock(JSON.stringify(good)), good);
  assert.equal(readBlock(null), null);
});

const block = (cells, rows, cols, panel = 'p-1') => ({ sode: CLIP_MARK, version: CLIP_VERSION, panel, rows, cols, cells });

test('a paste lands with its top left on the selection top left, whichever way the selection was dragged', () => {
  const g = pick(grid([]), 3, 4, 2, 2); // dragged up and left: the anchor is the bottom right
  const b = block([{ r: 1, c: 2, rs: 1, cs: 1, kind: 'take' }], 1, 2);
  const plan = pastePlan(g, b, { panel: 'p-1' });
  assert.equal(plan.fits, true);
  assert.deepEqual(plan.at, { r: 2, c: 2 });
  assert.deepEqual(plan.placed, [{ r: 2, c: 3, rs: 1, cs: 1, kind: 'take' }]);
});

test('a paste that fits replaces the cells it touches whole and selects what it pasted', () => {
  const g = pick(
    grid([
      { r: 1, c: 3, rs: 2, cs: 2, kind: 'take' }, // shares one square with a pasted cell: goes whole
      { r: 3, c: 2, rs: 1, cs: 1, kind: 'clear' }, // in the block's area but under a square the block leaves blank: stays
      { r: 4, c: 8, rs: 1, cs: 1, kind: 'current' },
    ]),
    2,
    2,
  );
  const b = block(
    [
      { r: 1, c: 1, rs: 1, cs: 1, kind: 'source', n: 1 },
      { r: 1, c: 2, rs: 2, cs: 1, kind: 'source', n: 2 },
    ],
    2,
    2,
  );
  const next = applyPaste(g, pastePlan(g, b, { panel: 'p-1' }).placed);
  assert.deepEqual(
    next.cells.map(({ r, c, rs, cs, kind }) => [r, c, rs, cs, kind]).sort(),
    [
      [2, 2, 1, 1, 'source'],
      [2, 3, 2, 1, 'source'],
      [3, 2, 1, 1, 'clear'],
      [4, 8, 1, 1, 'current'],
    ].sort(),
  );
  assert.deepEqual(next.sel, { anchor: { r: 2, c: 2 }, head: { r: 3, c: 3 } });
});

test('the pasted cells are copies, so editing the grid never changes the block', () => {
  const b = block([{ r: 1, c: 1, rs: 1, cs: 1, kind: 'source', n: 1, levels: [1] }], 1, 1);
  const g = pick(grid([]), 1, 1);
  const next = applyPaste(g, pastePlan(g, b, { panel: 'p-1' }).placed);
  next.cells[0].levels.push(2);
  assert.deepEqual(b.cells[0].levels, [1]);
});

test('a paste past the edge does not fit and changes nothing, with the counts a Clip or Wrap choice needs', () => {
  const g = pick(grid([{ r: 4, c: 8, rs: 1, cs: 1, kind: 'take' }]), 4, 7);
  const b = block(
    [
      { r: 1, c: 1, rs: 1, cs: 1, kind: 'take' },
      { r: 1, c: 2, rs: 2, cs: 2, kind: 'clear' },
    ],
    2,
    3,
  );
  const plan = pastePlan(g, b, { panel: 'p-1' });
  assert.equal(plan.fits, false);
  assert.deepEqual(plan.at, { r: 4, c: 7 });
  assert.deepEqual([plan.rows, plan.cols], [2, 3]);
  assert.equal(plan.outside, 1);
  assert.match(pasteNote(plan), /past the edge/);
});

test('no selection, no paste', () => {
  const plan = pastePlan(grid([]), block([{ r: 1, c: 1, rs: 1, cs: 1, kind: 'take' }], 1, 1), { panel: 'p-1' });
  assert.equal(plan, null);
});

test('into another panel, uploaded images go and vendored icons, library icons and Icon B stay', () => {
  const b = block(
    [
      { r: 1, c: 1, rs: 1, cs: 1, kind: 'take', art: 'a-0123abcd', artB: '3' },
      { r: 1, c: 2, rs: 1, cs: 1, kind: 'take', art: 'd-0123abcd' },
      { r: 1, c: 3, rs: 1, cs: 1, kind: 'take', art: 'icon:camera', artB: 'A' },
      { r: 1, c: 4, rs: 1, cs: 1, kind: 'take', art: 'lib:0123abcd' },
    ],
    1,
    4,
    'p-1',
  );
  const g = pick(grid([]), 1, 1);
  const other = pastePlan(g, b, { panel: 'p-2' });
  assert.deepEqual(
    other.placed.map((c) => [c.art, c.artB]),
    [
      [undefined, '3'],
      [undefined, undefined],
      ['icon:camera', 'A'],
      ['lib:0123abcd', undefined],
    ],
  );
  assert.ok(other.placed.every((c) => !('art' in c) || c.art));
  assert.equal(other.lostArt, 2);
  assert.match(pasteNote(other), /2 cells lost their uploaded image/);

  const same = pastePlan(g, b, { panel: 'p-1' });
  assert.equal(same.lostArt, 0);
  assert.equal(same.placed[0].art, 'a-0123abcd');

  // A panel not yet saved has no images of its own, so it counts as another panel either way.
  assert.equal(pastePlan(g, { ...b, panel: '' }, { panel: '' }).lostArt, 2);
});

test('a Go to Page cell pasted into another panel arrives unassigned, with a note; in its own panel it keeps its Page', () => {
  const b = block([{ r: 1, c: 1, rs: 1, cs: 1, kind: 'goToPage', page: 'aa' }], 1, 1, 'p-1');
  const g = pick(grid([]), 1, 1);
  const other = pastePlan(g, b, { panel: 'p-2' });
  assert.equal(other.placed[0].page, undefined);
  assert.equal(other.lostTargets, 1);
  assert.match(pasteNote(other), /1 Go to Page cell came in unassigned/);
  const same = pastePlan(g, b, { panel: 'p-1' });
  assert.equal(same.placed[0].page, 'aa');
  assert.equal(same.lostTargets, 0);
  // Pages of a panel not yet saved already have their ids, so a paste inside it keeps them.
  const unsaved = pastePlan(g, { ...b, panel: '' }, { panel: '' });
  assert.equal(unsaved.placed[0].page, 'aa');
  assert.equal(unsaved.lostTargets, 0);
});

test('while Fixed destination is on, Destination cells in the block are skipped and counted', () => {
  const b = block(
    [
      { r: 1, c: 1, rs: 1, cs: 1, kind: 'destination', n: 1 },
      { r: 1, c: 2, rs: 1, cs: 1, kind: 'source', n: 2 },
      { r: 2, c: 1, rs: 1, cs: 2, kind: 'destination', n: 3 },
    ],
    2,
    2,
  );
  const g = pick(grid([]), 1, 1);
  const on = pastePlan(g, b, { panel: 'p-1', fixed: true });
  assert.deepEqual(
    on.placed.map((c) => c.kind),
    ['source'],
  );
  assert.equal(on.skippedDests, 2);
  assert.match(pasteNote(on), /Skipped 2 Destination cells/);
  assert.equal(pastePlan(g, b, { panel: 'p-1' }).skippedDests, 0);
});

test('the note counts what was pasted', () => {
  const g = pick(grid([]), 1, 1);
  const one = pastePlan(g, block([{ r: 1, c: 1, rs: 1, cs: 1, kind: 'take' }], 1, 1), { panel: 'p-1' });
  assert.equal(pasteNote(one), 'Pasted 1 cell.');
});

test('a block with any cell field of the wrong shape is refused whole', () => {
  const one = (cell) => JSON.stringify({ sode: CLIP_MARK, version: CLIP_VERSION, panel: 'p-1', rows: 2, cols: 2, cells: [{ r: 1, c: 1, rs: 1, cs: 1, ...cell }] });
  const bad = [
    { kind: 'take', caption: {} },
    { kind: 'take', caption: 'x'.repeat(41) },
    { kind: 'source', n: 1, levels: 5 },
    { kind: 'source', n: 1, levels: [0] },
    { kind: 'source', n: 1, levels: [1.5] },
    { kind: 'source', n: 'abc' },
    { kind: 'source', n: -1 },
    { kind: 'source', n: 70000 },
    { kind: 'goToPage', page: 7 },
    { kind: 'goToPage', page: 'has space' },
    { kind: 'info', show: 'panel' },
    { kind: 'info', show: ['rocket'] },
    { kind: 'pageName', show: ['name', 'name'] },
    { kind: 'take', art: 'http://x/y.png' },
    { kind: 'take', art: 5 },
    { kind: 'take', artB: 'nope' },
    { kind: 'take', hideText: 'yes' },
    { kind: 'take', fontSize: 1000 },
    { kind: 'preset', scope: 'world', name: 'A' },
    { kind: 'preset', scope: 'me', name: 5 },
  ];
  for (const cell of bad) assert.equal(readBlock(one(cell)), null, JSON.stringify(cell));
  const tooMany = { sode: CLIP_MARK, version: CLIP_VERSION, panel: '', rows: 1, cols: 1, cells: [{ r: 1, c: 1, rs: 1, cs: 1, kind: 'take' }, { r: 1, c: 1, rs: 1, cs: 1, kind: 'take' }] };
  assert.equal(readBlock(JSON.stringify(tooMany)), null);
});

test('a block keeps every known cell field and drops unknown keys', () => {
  const cells = [
    { r: 1, c: 1, rs: 1, cs: 1, kind: 'source', n: 0, caption: 'Cam 1', levels: [1, 2], art: 'icon:camera', artB: '2', fontSize: 120, hideText: true },
    { r: 1, c: 2, rs: 1, cs: 1, kind: 'goToPage', page: 'ab-1' },
    { r: 2, c: 1, rs: 1, cs: 1, kind: 'info', show: ['panel', 'page'] },
    { r: 2, c: 2, rs: 1, cs: 1, kind: 'preset', scope: 'me', name: 'Studio' },
  ];
  const text = JSON.stringify({ sode: CLIP_MARK, version: CLIP_VERSION, panel: 'p-1', rows: 2, cols: 2, cells: [{ ...cells[0], junk: 'x'.repeat(900_000) }, ...cells.slice(1)] });
  assert.deepEqual(readBlock(text).cells, cells);
});
