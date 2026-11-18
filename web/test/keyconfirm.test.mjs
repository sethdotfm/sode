// The take confirm works from a plan the caller supplies, so a Grid panel can use it.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

// tooltip.js touches document.body at load; the small fake below replaces it after the import.
const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };
globalThis.WebSocket = { OPEN: 1 };
// Modules that add document listeners at load must load while the Proxy is in place.
await import('./all.mjs');

function fake(tag) {
  const on = {};
  const e = {
    tag,
    kids: [],
    hidden: false,
    disabled: false,
    className: '',
    focus() {},
    setAttribute() {},
    dataset: {},
    style: { setProperty() {}, removeProperty() {} },
    addEventListener: (t, fn) => (on[t] = fn),
    click: () => on.click?.(),
    querySelector: () => null,
    append: (...k) => e.kids.push(...k.filter((x) => x != null)),
    replaceChildren: (...k) => (e.kids = k.filter((x) => x != null)),
    classList: { remove: (c) => (e.className = e.className.replace(c, '').trim()), contains: (c) => e.className.split(' ').includes(c) },
  };
  let own = null;
  Object.defineProperty(e, 'textContent', {
    get: () => own ?? e.kids.map((k) => (typeof k === 'string' ? k : k.textContent)).join(''),
    set: (v) => {
      own = String(v);
      e.kids = [];
    },
  });
  return e;
}

const ids = ['confirm', 'confirm-dialog', 'confirm-title', 'confirm-body', 'confirm-ok', 'confirm-cancel'];
let dom;
function fakeDocument() {
  dom = new Map(ids.map((id) => [id, fake('div')]));
  globalThis.document = { getElementById: (id) => dom.get(id) ?? null, createElement: fake, body: fake('body') };
}

const tick = () => new Promise((r) => setTimeout(r, 5));
const key = (k, extra = {}) => ({ key: k, preventDefault() {}, stopPropagation() {}, ...extra });
const shownKey = () =>
  dom
    .get('confirm-ok')
    .textContent.match(/\((\w)\)$/)[1]
    .toLowerCase();

function setup(S, opts = {}) {
  fakeDocument();
  S.cfg = { confirmLockoutMs: 0, confirmEnter: false, ...opts.cfg };
  const got = { take: [], close: [] };
  return got;
}

test('keyConfirm: a supplied plan with a summary from the caller', async () => {
  const { keyConfirm } = await import('../keyconfirm.js');
  const { S } = await import('../core.js');
  const got = setup(S);
  const plan = { verb: 'patch', items: [{ dst: { n: 1 } }] };
  let seen = null;
  const c = keyConfirm({
    prompt: (k) => ['Press ', k],
    verb: () => 'take',
    summary: (p) => {
      seen = p;
      return ['stub summary'];
    },
    onTake: (p, viaKeys) => got.take.push([p, viaKeys]),
    onClose: (p, viaKeys) => got.close.push([p, viaKeys]),
  });
  c.open(plan, new Map());
  assert.equal(seen, plan);
  assert.equal(dom.get('confirm').hidden, false);
  assert.equal(dom.get('confirm-title').textContent, 'Confirm take');
  assert.match(dom.get('confirm-body').textContent, /^stub summaryPress \w$/);
  assert.equal(dom.get('confirm-ok').disabled, true, 'locked until the lockout passes');
  await tick();
  assert.equal(dom.get('confirm-ok').disabled, false);
  const k = shownKey();
  const wrong = 'abcdefghjkmnpqrstuvwxyz'.split('').find((x) => x !== k);
  c.key(key(wrong));
  assert.equal(got.take.length, 0, 'a wrong letter does nothing');
  c.key(key(k));
  assert.deepEqual(got.take, [[plan, true]]);
  assert.equal(dom.get('confirm').hidden, true);
  assert.equal(c.isOpen(), false);
});

test('keyConfirm: Esc closes with the plan; Enter takes only when the caller allows it', async () => {
  const { keyConfirm } = await import('../keyconfirm.js');
  const { S } = await import('../core.js');
  const got = setup(S);
  let enter = false;
  const c = keyConfirm({
    prompt: (k) => [k],
    verb: () => 'undo',
    summary: () => [],
    enter: () => enter,
    onTake: (p) => got.take.push(p),
    onClose: (p, viaKeys) => got.close.push([p, viaKeys]),
  });
  const plan = { verb: 'undo', items: [] };
  c.open(plan);
  assert.equal(dom.get('confirm-title').textContent, 'Confirm undo');
  assert.match(dom.get('confirm-ok').textContent, /^Undo \([A-Z]\)$/);
  c.key(key('Escape'));
  assert.deepEqual(got.close, [[plan, true]]);
  c.open(plan);
  await tick();
  c.key(key('Enter'));
  assert.equal(got.take.length, 0);
  enter = true;
  c.key(key('Enter'));
  assert.deepEqual(got.take, [plan]);
});

test('keyConfirm: a second open cancels the first lockout', async (t) => {
  // Fake clock: real timers slip under CPU load and made the 25ms check land after the second unlock.
  t.mock.timers.enable({ apis: ['setTimeout'] });
  // lockout.js ready() reads performance.now, so both clocks advance together.
  let now = 0;
  t.mock.method(performance, 'now', () => now);
  const tick = (ms) => {
    now += ms;
    t.mock.timers.tick(ms);
  };
  const { keyConfirm } = await import('../keyconfirm.js');
  const { S } = await import('../core.js');
  const got = setup(S, { cfg: { confirmLockoutMs: 40 } });
  const c = keyConfirm({ prompt: (k) => [k], verb: () => 'take', summary: () => [], onTake: (p) => got.take.push(p), onClose() {} });
  c.open({ verb: 'patch', items: [] });
  tick(20);
  const second = { verb: 'patch', items: [] };
  c.open(second);
  tick(25);
  assert.equal(dom.get('confirm-ok').disabled, true, "the first open's timer must not unlock the second");
  c.take();
  await new Promise((r) => setImmediate(r));
  assert.equal(got.take.length, 0);
  tick(30);
  c.take();
  await new Promise((r) => setImmediate(r));
  assert.deepEqual(got.take, [second]);
});

test('keyConfirm does not import the plan module', () => {
  const src = readFileSync(new URL('../keyconfirm.js', import.meta.url), 'utf8');
  assert.doesNotMatch(src, /from '\.\/plans\.js'/);
});

// The main page supplies planVerb and planSummary from the plan module.
async function mainPage(n) {
  const m = await import('./all.mjs');
  const { keyConfirm } = await import('../keyconfirm.js');
  const { S } = m;
  const got = setup(S);
  S.levels = [{ ui: 1 }, { ui: 2 }];
  S.levelBy = new Map([1, 2].map((u) => [u, {}]));
  S.tally = new Map();
  const it = (k, x) => ({ n: x, num: x + 1, name: `${k}${x + 1}`, label: `${k}${x + 1}` });
  const plan = m.patchPlan(
    it('Cam ', 0),
    Array.from({ length: n }, (_, i) => it('Mon ', i)),
    [1, 2],
  );
  const c = keyConfirm({
    prompt: (k) => ['Press ', k],
    verb: m.planVerb,
    summary: m.planSummary,
    onTake: (p) => got.take.push(p),
    onClose() {},
  });
  c.open(plan, new Map());
  await tick();
  return { c, got, plan, body: dom.get('confirm-body').textContent.replace(/\s+/g, ' ') };
}

test('keyConfirm: a main-page plan of 1 route', async () => {
  const { c, got, plan, body } = await mainPage(1);
  assert.equal(dom.get('confirm-title').textContent, 'Confirm take');
  assert.match(body, /Source Cam 1 to Destination Mon 1\?/);
  assert.doesNotMatch(body, /1\/1/);
  c.key(key(shownKey()));
  assert.equal(got.take[0], plan);
});

test('keyConfirm: a main-page plan of 128 routes lists every one', async () => {
  const { c, got, plan, body } = await mainPage(128);
  assert.match(body, /Source Cam 1 to 128 destinations\?/);
  for (const i of [1, 64, 128]) assert.ok(body.includes(`${i}/128 Mon ${i} `), `item ${i} listed`);
  c.key(key(shownKey()));
  assert.equal(got.take[0], plan);
  assert.equal(got.take[0].items.length, 128);
});
