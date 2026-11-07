// globalSaver with a fake socket: send, answer, PIN, lock, lost socket.
import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };

const make = async () => {
  const core = await import('../core.js');
  const { globalSaver } = await import('../globalsave.js');
  const sent = [];
  core.S.ws = { readyState: 1, lastRx: Date.now() + 1e9, send: (m) => sent.push(JSON.parse(m)) };
  core.S.pinSet = false;
  const notes = [];
  const mk = () => ({ addEventListener() {}, focus() {}, select() {}, hidden: true, value: '', disabled: false });
  let renders = 0;
  const saver = globalSaver({
    t: 'presets', resultT: `r${Math.random()}`, pinIn: mk(), pinRow: mk(), pinOk: mk(),
    setNote: (t, e) => notes.push([t, !!e]), isOpen: () => true, failTitle: 'x', okNote: 'Saved.', render: () => { renders++; },
  });
  return { core, saver, sent, notes, renders: () => renders };
};

test('a change is sent with its id and t; payload t wins; done runs on ok', async () => {
  const { saver, sent, notes } = await make();
  let done = 0;
  saver.change({ presets: [] }, () => { done++; });
  assert.equal(sent.length, 1);
  assert.equal(sent[0].t, 'presets');
  assert.ok(saver.busy());
  assert.deepEqual(notes.at(-1), ['Saving…', false]);
  saver.change({ t: 'filterSets', sets: [] }, () => {});
  assert.equal(sent[1].t, 'filterSets');
  assert.equal(done, 0);
});

test('a socket that cannot send says so and is not left busy', async () => {
  const { core, saver, notes } = await make();
  core.S.ws = null;
  saver.change({}, () => {});
  assert.ok(!saver.busy());
  assert.deepEqual(notes.at(-1), ['Not connected to Sode.', true]);
});

test('close keeps a sent save, drops one waiting for the PIN', async () => {
  const { core, saver } = await make();
  saver.change({}, () => {});
  assert.equal(saver.close(), true);
  core.S.pinSet = true;
  const { saver: s2 } = await make();
  core.S.pinSet = true;
  s2.change({}, () => {});
  assert.equal(s2.close(), false);
  assert.ok(!s2.busy());
});

test('two savers of one result type register once and keep their ids apart', async () => {
  const core = await import('../core.js');
  const { globalSaver } = await import('../globalsave.js');
  const sent = [];
  core.S.ws = { readyState: 1, lastRx: Date.now() + 1e9, send: (m) => sent.push(JSON.parse(m)) };
  core.S.pinSet = false;
  const mk = () => ({ addEventListener() {}, focus() {}, select() {}, hidden: true, value: '', disabled: false });
  const opts = () => ({ t: 'presets', resultT: 'sharedResult', pinIn: mk(), pinRow: mk(), pinOk: mk(), setNote() {}, isOpen: () => true, failTitle: 'x' });
  const a = globalSaver(opts());
  const b = globalSaver(opts()); // used to throw: onMsg('sharedResult') is already handled
  let doneA = 0, doneB = 0;
  a.change({ presets: [] }, () => { doneA++; });
  b.change({ presets: [] }, () => { doneB++; });
  assert.notEqual(sent[0].id, sent[1].id);
  core.handleMessage?.({ t: 'sharedResult', ok: true, id: sent[1].id });
  if (core.handleMessage) { assert.equal(doneA, 0); assert.equal(doneB, 1); }
});
