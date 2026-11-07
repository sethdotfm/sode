import test from 'node:test';
import assert from 'node:assert/strict';

const node = () => new Proxy(function () {}, { get: (t, k) => (k === Symbol.toPrimitive ? () => '' : node()), apply: () => node(), set: () => true });
globalThis.document = node();
const mem = new Map();
globalThis.localStorage = { getItem: (k) => (mem.has(k) ? mem.get(k) : null), setItem: (k, v) => mem.set(k, String(v)), removeItem: (k) => mem.delete(k) };

const { browserFilters, profileFilters, browserPresets, profilePresets } = await import('../editortargets.js');
const { editTarget } = await import('../profile.js');

const norm = (f) => ({ sode: true, ext: true, hidden: false, src: '', dst: '', ...f });
const prof = (key, list, name = '') => editTarget({ user: 'Ana', name, profile: { [key]: list } }, key);

function browserF() {
  const log = [];
  let cur = norm({ src: 'a' });
  const t = browserFilters({
    read: () => cur,
    write: (n) => (cur = norm(n)),
    snapshot: () => ({ ...cur }),
    restore: (s) => log.push(['restore', s.src]),
    onClose: () => log.push(['close']),
  });
  return { t, log, get: () => cur };
}

test('browser filters: reads and writes the browser filter, no profile', () => {
  const { t, get } = browserF();
  assert.equal(t.own, true);
  assert.equal(t.edit, null);
  assert.equal(t.saver, null);
  assert.equal(t.title, 'Filters');
  t.put({ src: 'b' });
  assert.equal(t.filter().src, 'b');
  assert.equal(get().src, 'b');
});

test('browser filters: Cancel restores what the dialog opened with, a landed write moves the point', () => {
  const { t, log } = browserF();
  const closes = [];
  t.begin();
  t.put({ src: 'b' });
  t.cancel(
    () => log.push(['flush']),
    (flush) => closes.push(flush),
  );
  assert.deepEqual(log, [['flush'], ['restore', 'a']]);
  assert.deepEqual(closes, [false]);
  log.length = 0;
  t.put({ src: 'c' });
  t.landed();
  t.cancel(
    () => {},
    () => {},
  );
  assert.deepEqual(log, [['restore', 'c']]);
});

test('browser filters: Cancel before begin only closes', () => {
  const { t, log } = browserF();
  const closes = [];
  t.cancel(
    () => log.push(['flush']),
    (flush) => closes.push(flush),
  );
  assert.deepEqual(log, []);
  assert.deepEqual(closes, [true]);
});

test('browser filters: no profile save', () => {
  assert.match(browserF().t.build('x').err, /Not editing/);
});

test('profile filters: edits a draft, never the browser filter', () => {
  const { get } = browserF();
  const t = profileFilters(prof('filterSets', [{ name: 'One', src: 'a' }], 'one'), { normalize: norm, saver: 'S' });
  assert.equal(t.own, false);
  assert.equal(t.edit.user, 'Ana');
  assert.equal(t.saver, 'S');
  assert.equal(t.title, 'Edit filter set for Ana');
  assert.equal(t.name, 'One');
  t.put({ src: 'z' });
  assert.equal(t.filter().src, 'z');
  assert.equal(get().src, 'a');
  const closes = [];
  t.cancel(
    () => assert.fail('no flush'),
    (flush) => closes.push(flush),
  );
  assert.deepEqual(closes, [true]);
});

test('profile filters: a new set is titled Add', () => {
  const t = profileFilters(prof('filterSets', []), { normalize: norm, saver: null });
  assert.equal(t.title, 'Add filter set for Ana');
  assert.equal(t.name, '');
});

test('profile filters: build replaces the edited entry, adds a new one, refuses bad names', () => {
  const sets = [
    { name: 'One', src: 'a' },
    { name: 'Two', src: 'b' },
  ];
  const t = profileFilters(prof('filterSets', sets, 'One'), { normalize: norm, saver: null });
  t.put({ src: 'q' });
  const r = t.build('Uno');
  assert.deepEqual(
    r.list.map((x) => x.name),
    ['Uno', 'Two'],
  );
  assert.equal(r.list[0].src, 'q');
  assert.match(t.build('two').err, /already a filter set named/);
  assert.match(t.build('All').err, /taken/);
  assert.equal(t.build('').err, 'Name required.');
  assert.equal(t.build('one').list[0].name, 'one');
});

test('browser presets: own scope lists, scope note, saves where it was told', () => {
  const calls = [];
  const t = browserPresets({
    list: (scope) => [scope],
    note: (cur) => `from ${cur.from}`,
    save: (b, moving) => calls.push([b, moving]),
    onClose: () => calls.push('close'),
  });
  assert.equal(t.own, true);
  assert.equal(t.edit, null);
  assert.equal(t.saver, null);
  assert.equal(t.title, 'Edit Presets');
  assert.deepEqual(t.list('local'), ['local']);
  assert.equal(t.note({ from: 'global' }), 'from global');
  t.save('b', true);
  t.close();
  assert.deepEqual(calls, [['b', true], 'close']);
});

test('profile presets: the profile list, no scope note, its own saver', () => {
  const saved = [];
  const t = profilePresets(prof('presets', [{ name: 'One', items: [] }], 'One'), { save: (b) => saved.push(b), saver: 'S' });
  assert.equal(t.own, false);
  assert.equal(t.edit.user, 'Ana');
  assert.equal(t.saver, 'S');
  assert.equal(t.title, 'Edit preset for Ana');
  assert.equal(t.list('local'), t.edit.lists);
  assert.equal(t.note({ old: 'One', from: 'user' }), '');
  t.save('b');
  t.close();
  assert.deepEqual(saved, ['b']);
  assert.equal(profilePresets(prof('presets', []), { save() {}, saver: null }).title, 'Add preset for Ana');
});
