import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  presetMarks,
  filterSetMarks,
  statusOf,
  leftOutLines,
  newBannerState,
  useItem,
  dismissItem,
  bannerItems,
  localPresetMarks,
  localFilterMarks,
  cellStatus,
} from '../brokenrules.js';

const dst = (n, name) => ({ kind: 'destination', ref: { router: 'R', n, name } });
const src = (n, name) => ({ kind: 'source', ref: { router: 'R', n, name } });
const preset = (extra = {}) => ({
  scope: 'global',
  id: 'p1',
  name: 'Studio',
  items: [{ src: 0, dsts: [22, 39], levels: [1, 2], router: 'R', srcName: 'CAM 1', dstNames: {} }],
  ...extra,
});

test('a preset mark counts only while the preset still holds that reference', () => {
  assert.deepEqual(presetMarks(preset({ broken: [dst(22, 'MON 23')] })).broken, [dst(22, 'MON 23')]);
  assert.deepEqual(presetMarks(preset({ broken: [dst(5, 'MON 6')] })).broken, [], 'the editor moved off destination 6');
  assert.deepEqual(presetMarks(preset({ broken: [{ kind: 'level', ref: { router: 'R', n: 2 } }] })).broken.length, 1);
  assert.deepEqual(presetMarks(preset({ moved: [src(0, 'CAM 1')] })).moved.length, 1);
});

test('status: Broken wins over Name moved; no marks is OK', () => {
  assert.equal(statusOf({ broken: [dst(22)], moved: [src(0)] }), 'broken');
  assert.equal(statusOf({ broken: [], moved: [src(0)] }), 'moved');
  assert.equal(statusOf({ broken: [], moved: [] }), '');
});

test('a filter set mark counts while its number or exact term is still in the text', () => {
  const mark = { kind: 'destination', ref: { router: 'R', n: -1, name: 'MON 23' } };
  const num = { kind: 'source', ref: { router: 'R', n: 12 } };
  const set = (src, dst) => ({ scope: 'global', name: 'Desk', f: { src, dst }, broken: [mark, num] });
  assert.equal(filterSetMarks(set('13', '"MON 23", MON'), 1).broken.length, 2);
  assert.equal(filterSetMarks(set('14', '"MON 24"'), 1).broken.length, 0);
  assert.equal(filterSetMarks(set('13', '!"MON 23"'), 1).broken.length, 1, 'a negated term never breaks');
});

test('left out lines name the part and why', () => {
  assert.deepEqual(
    leftOutLines(
      [
        dst(22, 'MON 23'),
        { kind: 'level', ref: { router: 'R', n: 5 } },
        { kind: 'destination', ref: { router: 'R', n: -1, name: 'MON 9' } },
        { kind: 'router', ref: { router: 'X' } },
      ],
      1,
    ),
    [
      'Destination 23 MON 23: not on the router now',
      'Level 5: not on the router now',
      'Destination "MON 9": no destination has this name now',
      'Its router is not in Sode',
    ],
  );
});

test('the banner shows an item once, until dismissed or fixed', () => {
  const st = newBannerState();
  const broken = { key: 'preset:global:p1', name: 'Studio', broken: [dst(22, 'MON 23')], moved: [] };
  useItem(st, broken);
  useItem(st, broken);
  assert.equal(bannerItems(st, new Map([[broken.key, broken]])).length, 1, 'firing twice shows one entry');

  const fixed = { ...broken, broken: [] };
  assert.equal(bannerItems(st, new Map([[broken.key, fixed]])).length, 0, 'fixed: the entry goes');
  useItem(st, fixed);
  assert.equal(bannerItems(st, new Map([[broken.key, fixed]])).length, 0, 'a fixed item never shows');

  useItem(st, broken);
  dismissItem(st, broken.key);
  useItem(st, broken);
  assert.equal(bannerItems(st, new Map([[broken.key, broken]])).length, 0, 'dismissed stays dismissed for the same marks');
  const worse = { ...broken, broken: [dst(22, 'MON 23'), dst(39, 'MON 40')] };
  useItem(st, worse);
  assert.equal(bannerItems(st, new Map([[broken.key, worse]])).length, 1, 'a new missing part shows it again');
  assert.equal(bannerItems(st, new Map()).length, 0, 'a deleted item goes');
});

test('a panel cell takes the status of the mark on its number', () => {
  const p = { broken: [dst(22, 'MON 23')], moved: [src(0, 'CAM 1')] };
  assert.equal(cellStatus(p, { kind: 'destination', n: 22 }), 'broken');
  assert.equal(cellStatus(p, { kind: 'source', n: 0 }), 'moved');
  assert.equal(cellStatus(p, { kind: 'source', n: 22 }), '');
  assert.equal(cellStatus(p, { kind: 'take' }), '');
});

test('Name moved alone never shows the banner', () => {
  const st = newBannerState();
  const moved = { key: 'preset:global:p2', name: 'Spare', broken: [], moved: [src(0, 'CAM 1')] };
  useItem(st, moved);
  assert.equal(bannerItems(st, new Map([[moved.key, moved]])).length, 0);
});

test('This browser: a stored name now at another number is Name moved; a gone exact name breaks a set', () => {
  const names = {
    src: new Map([
      [0, 'CAM 9'],
      [4, 'CAM 1'],
    ]),
    dst: new Map([[22, 'MON 23']]),
  };
  const p = { scope: 'local', id: 'l1', name: 'Mine', items: [{ src: 0, dsts: [22], levels: [], router: 'R', srcName: 'CAM 1', dstNames: { 22: 'MON 23' } }] };
  assert.deepEqual(localPresetMarks(p, names, 'R'), { broken: [], moved: [src(0, 'CAM 1')] });
  const renamed = { ...p, items: [{ ...p.items[0], srcName: 'CAM 0' }] };
  assert.deepEqual(localPresetMarks(renamed, names, 'R').moved, [], 'a plain rename is no mark');
  const set = { scope: 'local', name: 'Mine', f: { src: '"CAM 1", "CAM 7"', dst: '' } };
  assert.deepEqual(localFilterMarks(set, names, 'R').broken, [{ kind: 'source', ref: { router: 'R', n: -1, name: 'CAM 7' } }]);
});
