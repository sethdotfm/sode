import test from 'node:test';
import assert from 'node:assert/strict';
import { LIST_KEY, addKey, freePrefix, listKey, readRouter, removeKey, routerDisplay, routerListProblem, usesNote } from '../routerlistrules.js';
import { critical, reconnects, sectionOfKey } from '../savereviewrules.js';

test('freePrefix: the first letter no router has, then two letters', () => {
  assert.equal(freePrefix(['A']), 'B');
  assert.equal(freePrefix(['A', 'C']), 'B');
  assert.equal(freePrefix([...'ABCDEFGHIJKLMNOPQRSTUVWXYZ']), 'AA');
  assert.equal(freePrefix([...'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'AA']), 'AB');
});

test('readRouter: typed text to the saved values', () => {
  assert.deepEqual(readRouter({ prefix: ' ba ', title: ' Stage 2 ', address: ' 10.0.0.9:8910 ', matrix: '2' }), {
    prefix: 'BA',
    title: 'Stage 2',
    address: '10.0.0.9:8910',
    matrix: 2,
  });
  assert.equal(readRouter({ prefix: 'C', levels: '' }).levels, 0);
  assert.equal(readRouter({ prefix: 'C', levels: '4' }).levels, 4);
});

test('routerListProblem: prefixes are one or two letters and each router has its own', () => {
  const r = (prefix, more = {}) => ({ name: `Router ${prefix}`, prefix, title: '', matrix: 0, ...more });
  assert.equal(routerListProblem([r('A'), r('B'), r('BA')]), '');
  assert.match(routerListProblem([r('A'), r('A')]), /Two routers have the prefix A/);
  assert.match(routerListProblem([r('A'), r('B1')]), /Router B1: the prefix must be one or two letters/);
  assert.match(routerListProblem([r('A'), r('ABC')]), /one or two letters/);
  assert.match(routerListProblem([r('A'), r('B', { title: 'x'.repeat(33) })]), /at most 32 characters/);
  assert.match(routerListProblem([r('A'), r('B', { matrix: 256 })]), /matrix/);
  assert.match(routerListProblem([r('A'), r('B', { levels: 300 })]), /levels/);
  assert.equal(routerListProblem([r('A'), r('B', { levels: 0 })]), '');
});

test('usesNote: what Remove router breaks', () => {
  assert.deepEqual(usesNote('B', []), { head: 'Router B is in no preset, filter set, panel or tie line.', lines: [] });
  const n = usesNote('B', [
    { kind: 'filterSet', name: 'B cams', owner: '' },
    { kind: 'panel', name: 'Desk', owner: 'Ana' },
    { kind: 'preset', name: 'Stage', owner: '' },
    { kind: 'preset', name: 'Mine', owner: 'Ana' },
  ]);
  assert.equal(n.head, 'Remove router B makes 2 presets, 1 filter set and 1 panel Broken:');
  assert.deepEqual(n.lines, ['Filter set B cams (Everyone)', 'Panel Desk (Ana)', 'Preset Stage (Everyone)', 'Preset Mine (Ana)']);
  const ties = [
    { kind: 'tieLine', name: 'TIE 3', owner: '' },
    { kind: 'tieLine', name: 'TIE 4', owner: '' },
  ];
  assert.deepEqual(usesNote('B', ties), { head: 'Remove router B removes 2 tie lines:', lines: ['Tie line TIE 3', 'Tie line TIE 4'] });
  assert.equal(usesNote('B', [{ kind: 'preset', name: 'Stage', owner: '' }, ties[0]]).head, 'Remove router B makes 1 preset Broken and removes 1 tie line:');
});

test('the Routers list keys: section routers, and Add, Remove, Address and Matrix are Critical', () => {
  for (const k of [listKey('b1', 'prefix'), listKey('b1', 'title'), listKey('b1', 'address'), listKey('b1', 'matrix'), addKey(1), removeKey('b1')]) {
    assert.ok(LIST_KEY.test(k), k);
    assert.equal(sectionOfKey(k), 'routers', k);
  }
  assert.equal(sectionOfKey('router/b1/levels'), 'router-levels');
  assert.ok(!LIST_KEY.test('router/b1/levels'));
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
