import test from 'node:test';
import assert from 'node:assert/strict';
import {
  LIST_KEY,
  addKey,
  freePrefix,
  linkState,
  listKey,
  orderStep,
  removeKey,
  routerDisplay,
  orderLabel,
  rowStep,
  removedText,
  usesNote,
} from '../routerlistrules.js';
import { critical, reconnects, sectionOfKey } from '../savereviewrules.js';

test('freePrefix: the first letter no router has, then two letters', () => {
  assert.equal(freePrefix(['A']), 'B');
  assert.equal(freePrefix(['A', 'C']), 'B');
  assert.equal(freePrefix([...'ABCDEFGHIJKLMNOPQRSTUVWXYZ']), 'AA');
  assert.equal(freePrefix([...'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'AA']), 'AB');
});

test('usesNote: what Remove router breaks', () => {
  assert.deepEqual(usesNote('B', []), { head: 'Router B is in no preset, filter set, panel or tie line.', lines: [] });
  const n = usesNote('B', [
    { kind: 'filterSet', name: 'B cams', owner: '' },
    { kind: 'panel', name: 'Desk', owner: 'Ana' },
    { kind: 'preset', name: 'Stage', owner: '' },
    { kind: 'preset', name: 'Mine', owner: 'Ana' },
  ]);
  assert.equal(n.head, 'If you remove router B, Sode makes 2 presets, 1 filter set and 1 panel Broken:');
  assert.deepEqual(n.lines, ['Filter set B cams (Everyone)', 'Panel Desk (Ana)', 'Preset Stage (Everyone)', 'Preset Mine (Ana)']);
  const ties = [
    { kind: 'tieLine', name: 'TIE 3', owner: '' },
    { kind: 'tieLine', name: 'TIE 4', owner: '' },
  ];
  assert.deepEqual(usesNote('B', ties), { head: 'If you remove router B, Sode removes 2 tie lines:', lines: ['Tie line TIE 3', 'Tie line TIE 4'] });
  assert.equal(
    usesNote('B', [{ kind: 'preset', name: 'Stage', owner: '' }, ties[0]]).head,
    'If you remove router B, Sode makes 1 preset Broken and removes 1 tie line:',
  );
});

test('removedText: the Now of a removed router in the Save review is whole sentences, not the note head', () => {
  assert.equal(removedText([]), 'Removed.');
  const ties = [
    { kind: 'tieLine', name: 'TIE 3', owner: '' },
    { kind: 'tieLine', name: 'TIE 4', owner: '' },
    { kind: 'tieLine', name: 'TIE 5', owner: '' },
  ];
  assert.equal(removedText(ties), 'Removed. Also removes 3 tie lines.');
  const presets = [
    { kind: 'preset', name: 'Stage', owner: '' },
    { kind: 'preset', name: 'Mine', owner: 'Ana' },
  ];
  assert.equal(removedText(presets), 'Removed. Makes 2 presets Broken.');
  assert.equal(removedText([...presets, ...ties]), 'Removed. Makes 2 presets Broken and removes 3 tie lines.');
  assert.equal(removedText(ties.slice(0, 1), 'C Playout C'), 'Removed. Also removes 1 tie line. C Playout C becomes the default router.');
  for (const t of [removedText([...presets, ...ties], 'B Studio B'), removedText([])]) assert.ok(t.endsWith('.') && !t.includes(':'), t);
});

test('orderLabel: a new default router names the was and the now, with no arrow', () => {
  assert.equal(orderLabel('A Studio A', 'A Studio A'), 'Router order');
  assert.equal(orderLabel('A Studio A Core', 'C Playout C'), 'Router order. Default router: was A Studio A Core, now C Playout C');
});

test('the router keys: section routers, and Add, Remove, Address and Matrix are Critical', () => {
  for (const k of [
    listKey('b1', 'prefix'),
    listKey('b1', 'title'),
    listKey('b1', 'address'),
    listKey('b1', 'matrix'),
    listKey('b1', 'levels'),
    listKey('b1', 'protectDevice'),
    addKey(1),
    removeKey('b1'),
  ]) {
    assert.ok(LIST_KEY.test(k), k);
    assert.equal(sectionOfKey(k), 'routers', k);
  }
  assert.ok(!LIST_KEY.test('swap'));
  for (const k of [addKey(1), removeKey('b1'), listKey('b1', 'address'), listKey('b1', 'matrix')]) assert.ok(critical(k), k);
  assert.ok(reconnects(listKey('b1', 'address')));
  assert.ok(!critical(listKey('b1', 'title')) && !critical(listKey('b1', 'prefix')));
});

test('routerDisplay: the prefix and the Router title, or Router and the prefix with no title', () => {
  assert.equal(routerDisplay({ prefix: 'B', title: 'Studio B' }), 'B Studio B');
  assert.equal(routerDisplay({ prefix: 'B', title: '' }), 'Router B');
  assert.equal(routerDisplay({ prefix: 'B', title: '  ' }), 'Router B');
  assert.equal(routerDisplay({ prefix: 'B', title: 'Router B' }), 'Router B');
});

test('linkState: the link in words; only Down and Never connected are red, and Connected has no color', () => {
  assert.equal(linkState({ link: '' }, true), null);
  assert.deepEqual(linkState({ link: 'up' }, false), { key: 'connected', word: 'Connected', glyph: 'circle-check', state: '' });
  assert.deepEqual(linkState({ link: 'connecting', err: '' }, false), { key: 'connecting', word: 'Connecting', glyph: 'loader-circle', state: 'warn' });
  assert.deepEqual(linkState({ link: 'connecting', err: 'refused' }, true), { key: 'down', word: 'Down', glyph: 'circle-x', state: 'error' });
  assert.deepEqual(linkState({ link: 'down', err: 'refused' }, true), { key: 'down', word: 'Down', glyph: 'circle-x', state: 'error' });
  assert.deepEqual(linkState({ link: 'down', err: 'refused' }, false), { key: 'never', word: 'Never connected', glyph: 'circle-x', state: 'error' });
  assert.deepEqual(linkState({ link: 'connecting', err: 'refused' }, false).word, 'Never connected');
});

test('rowStep: Left and Right move along the row of chips, Up and Down too, and the ends stop', () => {
  assert.equal(rowStep('ArrowRight', 0, 3), 1);
  assert.equal(rowStep('ArrowLeft', 2, 3), 1);
  assert.equal(rowStep('ArrowDown', 0, 3), 1);
  assert.equal(rowStep('ArrowUp', 1, 3), 0);
  assert.equal(rowStep('ArrowLeft', 0, 3), 0);
  assert.equal(rowStep('ArrowRight', 2, 3), 2);
  assert.equal(rowStep('Home', 2, 3), 0);
  assert.equal(rowStep('End', 0, 3), 2);
  assert.equal(rowStep('Enter', 0, 3), -1);
  assert.equal(rowStep('ArrowRight', 0, 0), -1);
});

test('orderStep: Alt and an arrow move a router one place', () => {
  assert.equal(orderStep('ArrowLeft'), -1);
  assert.equal(orderStep('ArrowUp'), -1);
  assert.equal(orderStep('ArrowRight'), 1);
  assert.equal(orderStep('ArrowDown'), 1);
  assert.equal(orderStep('Home'), 0);
});
