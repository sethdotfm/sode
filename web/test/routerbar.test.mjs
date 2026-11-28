// The one Routers icon of the status bar, the router list and the Degraded note (routerlinkrules.js). Owner's rule:
// one icon at every router count. No color while every router is up. Amber while one or more are lost and one or more
// are up. Red while every router is lost.
import test from 'node:test';
import assert from 'node:assert/strict';
import { routerLink, barMark, listRows, listName, routerName, rowState, degradedNote, wholeState, FIND_FROM } from '../routerlinkrules.js';

const up = { wsState: 'up' };
const P = (i) => (i < 26 ? String.fromCharCode(65 + i) : 'A' + String.fromCharCode(39 + i));
// kind: up, first (connecting for the first time), down (lost after it was up), never (lost and never up), again
// (connecting again after a failure).
const one = (kind, i) => {
  const link = { up: 'up', first: 'connecting', down: 'down', never: 'down', again: 'connecting' }[kind];
  const err = ['down', 'never', 'again'].includes(kind) ? 'connect: connection refused' : '';
  return routerLink(up, `u${i}`, P(i), `Studio ${P(i)}`, `10.0.0.${i + 1}:8910`, link, err, kind === 'never');
};
const routers = (kinds) => kinds.map(one);
const fill = (n, f) => Array.from({ length: n }, (_, i) => f(i));

test('one router: the word is Router, and the state follows its link', () => {
  const cases = [
    ['up', 'ok', 'Router connected', 0],
    ['first', 'warn', 'Router connecting', 0],
    ['down', 'error', 'Router down', 1],
    ['again', 'error', 'Router down', 1],
    ['never', 'error', 'Router never connected', 1],
  ];
  for (const [kind, state, aria, down] of cases) assert.deepEqual(barMark(routers([kind])), { state, word: 'Router', aria, down }, kind);
});

for (const n of [2, 4, 24]) {
  test(`${n} routers: no color while every router is up`, () => {
    assert.deepEqual(barMark(routers(fill(n, () => 'up'))), { state: 'ok', word: 'Routers', aria: `Routers: all ${n} connected`, down: 0 });
  });

  test(`${n} routers: amber while one or more are lost and one or more are up`, () => {
    for (const lostKind of ['down', 'never', 'again']) {
      for (const k of [1, n - 1]) {
        const m = barMark(routers(fill(n, (i) => (i < k ? lostKind : 'up'))));
        assert.deepEqual(m, { state: 'warn', word: `${k} of ${n} down`, aria: `Routers: ${k} of ${n} down`, down: k }, `${lostKind} ${k}`);
      }
    }
    // Lost routers at the end, and a mix of kinds, count the same.
    const mix = routers(fill(n, (i) => (i === n - 1 ? 'never' : i === 0 ? 'down' : 'up')));
    assert.equal(barMark(mix).word, `2 of ${n} down`);
  });

  test(`${n} routers: red while every router is lost`, () => {
    for (const lostKind of ['down', 'never', 'again']) {
      const m = barMark(routers(fill(n, () => lostKind)));
      assert.deepEqual([m.state, m.word], ['error', `${n} of ${n} down`], lostKind);
    }
    const mix = barMark(routers(fill(n, (i) => ['down', 'never', 'again'][i % 3])));
    assert.deepEqual([mix.state, mix.down], ['error', n]);
  });

  test(`${n} routers: a first connect is amber and is not counted as down`, () => {
    assert.deepEqual(barMark(routers(fill(n, (i) => (i === 0 ? 'first' : 'up')))), { state: 'warn', word: 'Routers', aria: 'Routers connecting', down: 0 });
    const m = barMark(routers(fill(n, (i) => (i === 0 ? 'first' : 'down'))));
    assert.deepEqual([m.state, m.word], ['warn', `${n - 1} of ${n} down`], 'not every router is lost');
  });

  test(`${n} routers: no Server link, nothing known`, () => {
    const list = fill(n, (i) => routerLink({ wsState: 'down' }, `u${i}`, P(i), '', '', 'up', ''));
    assert.deepEqual(barMark(list), { state: 'idle', word: 'Routers', aria: 'Routers unknown', down: 0 });
  });

  test(`${n} routers: the router list puts lost routers first and keeps config order in each group`, () => {
    const kinds = fill(n, (i) => ['up', 'down', 'first', 'never'][i % 4]);
    const rows = listRows(routers(kinds));
    assert.equal(rows.length, n);
    const words = rows.map((x) => x.word);
    const firstUp = words.indexOf('Connected');
    const lostWords = words.filter((w) => w === 'Down' || w === 'Never connected');
    assert.deepEqual(words.slice(0, lostWords.length), lostWords, 'lost first');
    assert.ok(firstUp === -1 || words.slice(firstUp).every((w) => w === 'Connected'), 'up last');
    for (const w of ['Down', 'Never connected', 'Connecting', 'Connected']) {
      const at = rows.filter((x) => x.word === w).map((x) => x.i);
      assert.deepEqual(
        at,
        [...at].sort((a, b) => a - b),
        `${w} in config order`,
      );
    }
  });
}

test('wholeState agrees with the color of the icon', () => {
  for (const n of [1, 2, 4, 24]) {
    assert.equal(wholeState(routers(fill(n, () => 'down'))), 'down');
    if (n > 1) assert.equal(wholeState(routers(fill(n, (i) => (i ? 'up' : 'down')))), 'degraded');
    assert.equal(wholeState(routers(fill(n, () => 'up'))), 'healthy');
  }
});

test('routerName: one name for a router on every page, never doubled', () => {
  assert.equal(routerName({ prefix: 'A', title: 'Router A' }), 'Router A');
  assert.equal(routerName({ prefix: 'A', title: '' }), 'Router A');
  assert.equal(routerName({ prefix: 'A' }), 'Router A');
  assert.equal(routerName({ prefix: 'B', title: 'Router B (studio 2)' }), 'Router B (studio 2)');
  assert.equal(routerName({ prefix: 'C', title: ' MCR ' }), 'Router C MCR');
  assert.equal(routerName({ prefix: 'A', title: 'Router AB' }), 'Router A Router AB', 'only the whole prefix counts');
});

test('routerName: a title that starts with the name, in any case and with any non-alphanumeric next character, is not doubled', () => {
  for (const t of ['Router B, studio 2', 'Router B(MCR)', 'router b', 'ROUTER B - MCR', 'Router B']) {
    assert.equal(routerName({ prefix: 'B', title: t }), t);
  }
  assert.equal(routerName({ prefix: 'B', title: 'Router B2' }), 'Router B Router B2');
  assert.equal(routerName({ prefix: 'B', title: 'Router Bar' }), 'Router B Router Bar');
});

test('the name of a router in the list: routerName, no prefix with one router', () => {
  assert.equal(listName({ prefix: 'B', title: 'Studio B' }, 4), 'Router B Studio B');
  assert.equal(listName({ prefix: 'B', title: 'Router B (studio 2)' }, 4), 'Router B (studio 2)');
  assert.equal(listName({ prefix: 'B', title: '' }, 4), 'Router B');
  assert.equal(listName({ prefix: 'B', title: 'Router B' }, 4), 'Router B', 'the prefix is not said twice');
  assert.equal(listName({ prefix: 'A', title: 'Main' }, 1), 'Main');
  assert.equal(listName({ prefix: 'A', title: '' }, 1), 'Router');
});

test('the state of a row in words: Connected has no color, Never connected says what to do', () => {
  assert.deepEqual(rowState(one('up', 0)), { word: 'Connected', state: '', note: '' });
  assert.deepEqual(rowState(one('first', 0)), { word: 'Connecting', state: 'warn', note: '' });
  assert.deepEqual(rowState(one('down', 0)), { word: 'Down', state: 'error', note: '' });
  assert.deepEqual(rowState(one('again', 0)), { word: 'Down', state: 'error', note: '' });
  assert.deepEqual(rowState(one('never', 0)), { word: 'Never connected', state: 'error', note: 'Check the address.' });
  assert.deepEqual(rowState(routerLink({ wsState: 'down' }, 'u', 'A', '', '', 'up', '')), { word: 'Not known', state: 'idle', note: '' });
  // A router that never connected and then connects is up: the server clears never.
  assert.equal(routerLink(up, 'u', 'A', '', '', 'up', '', false).never, false);
});

test('the find box: from FIND_FROM routers, by prefix, title or address', () => {
  assert.equal(FIND_FROM, 8);
  const list = routers(fill(24, () => 'up'));
  assert.deepEqual(
    listRows(list, 'studio k').map((x) => x.name),
    ['Router K Studio K'],
  );
  assert.deepEqual(
    listRows(list, '10.0.0.24').map((x) => x.r.prefix),
    ['X'],
  );
  assert.equal(listRows(list, 'router').length, 0, 'the word Router in every name is not a match');
  assert.equal(listRows(list, '  ').length, 24, 'blank text finds every router');
  assert.equal(listRows(list, 'nothing').length, 0);
});

test('the Degraded note: one line each up to two lost routers, then one sentence with a Show routers button', () => {
  const dsts = () => [{ label: 'B.1 MON 1' }];
  const lostOf = (kinds) => routers(kinds).filter((r) => r.state === 'error');
  assert.deepEqual(degradedNote([], dsts), { lines: [], more: false });
  assert.deepEqual(degradedNote(lostOf(['up', 'down']), dsts), {
    lines: ['Router B Studio B is down. You cannot patch B.1 MON 1 now.'],
    more: false,
  });
  assert.deepEqual(
    degradedNote(lostOf(['up', 'down', 'never']), () => []),
    {
      lines: [
        'Router B Studio B is down. You cannot patch its destinations now.',
        'Router C Studio C never connected. Check the address. You cannot patch its destinations now.',
      ],
      more: false,
    },
  );
  const many = lostOf(fill(24, (i) => ([1, 4, 11, 17, 22].includes(i) ? 'down' : 'up')));
  assert.deepEqual(degradedNote(many, dsts), {
    lines: ['5 routers are down: B, E, L, R, W. You cannot patch their destinations now.'],
    more: true,
  });
});
