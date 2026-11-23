import test from 'node:test';
import assert from 'node:assert/strict';
import { typedPrefix, prefixOf } from '../routerprefix.js';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
const doc = node();
globalThis.document = new Proxy(doc, { get: (t, k) => (k === 'baseURI' ? 'http://sode.test/' : doc[k]) });
const store = new Map();
globalThis.localStorage = { getItem: (k) => store.get(k) ?? null, setItem: (k, v) => store.set(k, String(v)), removeItem: (k) => store.delete(k) };
let sock = null;
globalThis.WebSocket = class {
  static OPEN = 1;
  readyState = 1;
  constructor() {
    sock = this;
  }
  send() {}
  close() {}
};
globalThis.setInterval = () => 0;

const { connect, boxSearch, routerNamed, routerOfText } = await import('../core.js');
const { boxRouter, nextBoxRouter } = await import('../boxrouter.js');

const level = (ui) => ({ ui, wire: ui - 1, label: String(ui) });
const router = (uid, prefix, srcs) => ({
  uid,
  prefix,
  title: `Router ${prefix}`,
  numberOffset: 1,
  levels: [level(1)],
  srcs,
  dsts: [[0, `${prefix} MON`]],
  tally: [],
});
const aSrcs = [
  [41, 'B42 MON'],
  [10, 'CAM 11'],
  [11, 'CAM 12'],
];
const hello = (routers) => ({
  t: 'hello',
  version: '',
  cfg: { numberOffset: 1, routerName: 'Main', filterSets: [], presets: [], bookmarks: [] },
  routers,
});
const send = (m) => sock.onmessage({ data: JSON.stringify(m) });
const labels = (list) => list.map((it) => it.label);

test('a prefix is one or two letters; the dot is optional only with two or more routers', () => {
  assert.deepEqual(prefixOf('ba.11'), { prefix: 'BA', text: '11', dot: true });
  assert.deepEqual(prefixOf('BA11'), { prefix: 'BA', text: '11', dot: false });
  assert.equal(prefixOf('CAM1').prefix, '', 'three letters are a name');
  const two = ['A', 'B', 'BA'];
  assert.deepEqual(
    ['A.11', 'A11', 'BA.11', 'BA11', 'b42', 'TX1', 'C.4'].map((q) => typedPrefix(q, two)),
    ['A', 'A', 'BA', 'BA', 'B', '', 'C'],
  );
  assert.equal(typedPrefix('A11', ['A']), '', 'with one router, A11 is a name');
  assert.equal(typedPrefix('A.11', ['A']), 'A');
});

test('B42 means B.42 before a source named "B42 MON"', () => {
  connect();
  sock.onopen();
  send(hello([router('aaa', 'A', aSrcs), router('bbb', 'B', [[41, 'VTR']]), router('bba', 'BA', [[10, 'GFX']])]));
  assert.deepEqual(labels(boxSearch('src', 'B42')), ['B.42 VTR', 'A.42 B42 MON']);
  assert.deepEqual(labels(boxSearch('src', 'b.42')), ['B.42 VTR'], 'with the dot, only the number');
  assert.deepEqual(labels(boxSearch('src', 'BA11')), ['BA.11 GFX']);
  assert.equal(boxSearch('src', 'A11')[0].label, 'A.11 CAM 11');
  assert.equal(boxSearch('src', 'C.4').length, 0, 'no router has C');
  assert.deepEqual([routerNamed('B42'), routerNamed('A.4'), routerNamed('42'), routerOfText('BA.1')], ['bbb', '', null, 'bba']);
});

test('every name match shows its prefix; a bare number is on the default router', () => {
  assert.deepEqual(labels(boxSearch('dst', 'mon')), ['A.1 A MON', 'B.1 B MON', 'BA.1 BA MON']);
  assert.deepEqual(labels(boxSearch('dst', 'mon', { def: 'bbb' })), ['B.1 B MON', 'A.1 A MON', 'BA.1 BA MON'], 'the default router first');
  assert.equal(boxSearch('src', '11')[0].label, 'A.11 CAM 11');
  assert.deepEqual(labels(boxSearch('src', '11', { def: 'bba' })), ['BA.11 GFX']);
});

test('each box keeps its default router for this browser; a new browser gets the first router', () => {
  store.clear();
  assert.deepEqual([boxRouter('src'), boxRouter('dst')], ['', '']);
  nextBoxRouter('dst');
  assert.deepEqual([boxRouter('src'), boxRouter('dst')], ['', 'bbb']);
  nextBoxRouter('dst');
  nextBoxRouter('dst');
  assert.equal(boxRouter('dst'), '', 'after the last router, the first');
  store.set('sode-box-routers', JSON.stringify({ src: 'gone' }));
  assert.equal(boxRouter('src'), '', 'a router that is gone gives the first router');
});

test('with one router nothing shows a prefix', () => {
  send(hello([router('aaa', 'A', aSrcs)]));
  assert.equal(boxSearch('src', 'A.11')[0].label, '11 CAM 11');
  assert.equal(boxSearch('src', '42')[0].label, '42 B42 MON');
  assert.equal(routerNamed('A11'), null, 'A11 is a name');
});
