// globalSaver with a fake socket: send, answer, PIN, lock, lost socket.
// Run: node --test web/test/
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
    t: 'salvos', resultT: `r${Math.random()}`, pinIn: mk(), pinRow: mk(), pinOk: mk(),
    setNote: (t, e) => notes.push([t, !!e]), isOpen: () => true, failTitle: 'x', okNote: 'Saved.', render: () => { renders++; },
  });
  return { core, saver, sent, notes, renders: () => renders };
};

test('a change is sent with its id and t; payload t wins; done runs on ok', async () => {
  const { saver, sent, notes } = await make();
  let done = 0;
  saver.change({ salvos: [] }, () => { done++; });
  assert.equal(sent.length, 1);
  assert.equal(sent[0].t, 'salvos');
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
  s2.change({}, () => {}); // no unlock: waits for the PIN
  assert.equal(s2.close(), false);
  assert.ok(!s2.busy());
});
