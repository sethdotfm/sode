import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { pickState, chosen, choices, choose, keyStep, moveStep, ready, takeRoute, expectOf, stepsOf, ago, changedText } from '../tiepickrules.js';

// The shared plan fixture (internal/tieplan, web/test/tieplan.test.mjs). The page reads the 409 body of POST /takes;
// apiRoute makes that shape from a case, as tieroutes.go apiTieRoute does.
const { plans } = JSON.parse(readFileSync(new URL('./tieplan_cases.json', import.meta.url), 'utf8'));
const byName = (start) => plans.find((c) => c.name.startsWith(start));
const end = (router, n) => ({ router, n, name: `${router.toUpperCase()}.${n}` });
const apiRoute = (c) => ({
  route: 0,
  src: end(c.route.src.router, c.route.src.n),
  dst: end(c.route.dst.router, c.route.dst.n),
  levels: c.route.levels,
  paths: c.want.paths.map((p) => ({
    ...p,
    affected: p.affected.map((e) => end(e.router, e.n)),
    hops: p.hops.map((h) => ({
      ...h,
      src: end(h.router, h.src),
      dst: end(h.router, h.dst),
      tie: h.tie ? { id: h.tie, name: h.tie.toUpperCase() } : null,
      now: h.now && {
        ...h.now,
        origin: h.now.origin && end(h.now.origin.router, h.now.origin.n),
        via: (h.now.via || []).map((id) => ({ id, name: id })),
        ...(h.now.mixed ? { levels: [end('a', 1), null] } : {}),
      },
    })),
  })),
  more: 0,
  reasons: [],
});
const ties = (st) =>
  chosen(st)
    .hops.slice(0, -1)
    .map((h) => h.tie.id);
const keys = (st, ...cs) => cs.reduce((s, c) => keyStep(s, c).st, st);

test('the picker opens on the best usable path', () => {
  for (const c of plans) {
    if (!c.want.paths.length) continue;
    const st = pickState(apiRoute(c));
    const first = c.want.paths.findIndex((p) => !p.protected && !p.reserved);
    assert.equal(st.path, Math.max(first, 0), c.name);
    assert.equal(st.step, 0, c.name);
  }
});

test('J and K or the arrows choose a tie line; the later steps follow the choice', () => {
  const st = pickState(apiRoute(byName('mesh: the cheapest')));
  assert.deepEqual(ties(st), ['t2']);
  assert.deepEqual(
    choices(st, 0).map((c) => c.tie),
    ['t2', 't1', 't3'],
  );
  assert.deepEqual(ties(keys(st, 'j')), ['t1']);
  assert.deepEqual(ties(keys(st, 'down', 'down')), ['t3', 't4'], 'a change on step 1 gives step 2');
  assert.deepEqual(ties(keys(st, 'j', 'j', 'j')), ['t3', 't4'], 'the last choice stays');
  assert.deepEqual(ties(keys(st, 'j', 'j', 'k')), ['t1'], 'back up drops the step after it');
  assert.deepEqual(ties(keys(st, 'up')), ['t2'], 'the first choice stays');
});

test('H and L or Tab and Shift+Tab move between steps, inside the chosen path', () => {
  let st = keys(pickState(apiRoute(byName('mesh: the cheapest'))), 'j', 'j');
  assert.equal(stepsOf(chosen(st)), 2);
  st = keys(st, 'l');
  assert.equal(st.step, 1);
  assert.equal(keys(st, 'tab').step, 1, 'the last step stays');
  assert.equal(keys(st, 'shift+tab').step, 0);
  assert.equal(keys(st, 'h', 'h').step, 0);
  // Back on step 0, a choice with one step moves nothing past it.
  assert.equal(moveStep(keys(st, 'h', 'k'), 1).step, 0);
});

test('Enter sends and Esc cancels; other keys do nothing', () => {
  const st = pickState(apiRoute(byName('line: one tie line')));
  assert.equal(keyStep(st, 'enter').act, 'send');
  assert.equal(keyStep(st, 'escape').act, 'cancel');
  for (const c of ['x', 'ctrl+j', 'alt+enter', 'space']) assert.deepEqual(keyStep(st, c), { st, act: null }, c);
});

test('a Protected tie line is grey: the keys skip it, and the picker never opens on it', () => {
  const st = pickState(apiRoute(byName('Protect: t1 costs less')));
  assert.deepEqual(ties(st), ['t2']);
  const cs = choices(st, 0);
  assert.deepEqual(
    cs.map((c) => [c.tie, c.held]),
    [
      ['t2', false],
      ['t1', true],
    ],
  );
  assert.deepEqual(ties(keys(st, 'j')), ['t2']);
  assert.ok(ready(st));
  // A tap on a grey choice shows its path, but the picker does not send it.
  const tapped = choose(st, 0, 't1');
  assert.deepEqual(ties(tapped), ['t1']);
  assert.ok(!ready(tapped));
  const all = pickState(apiRoute(byName('Protect: every tie line')));
  assert.ok(!ready(all));
});

test('a tap chooses a tie line on any step; an unknown id changes nothing', () => {
  const st = pickState(apiRoute(byName('mesh: the cheapest')));
  assert.deepEqual(ties(choose(st, 0, 't3')), ['t3', 't4']);
  assert.equal(choose(st, 0, 'nope'), st);
  assert.equal(choose(st, 4, 't1'), st);
});

test('the take carries each tie line with what the picker showed and the Affected destinations', () => {
  const st = pickState(apiRoute(byName('line: one tie line')));
  assert.deepEqual(takeRoute(st), {
    src: { router: 'a', n: 1 },
    dst: { router: 'b', n: 42 },
    levels: [1],
    ties: [{ id: 't1', expect: { router: 'a', n: 3 } }],
    affected: [
      { router: 'b', n: 20 },
      { router: 'b', n: 22 },
    ],
  });
  const chain = keys(pickState(apiRoute(byName('mesh: the cheapest'))), 'j', 'j');
  assert.deepEqual(takeRoute(chain).ties, [
    { id: 't3', expect: null },
    { id: 't4', expect: null },
  ]);
});

test('expect: one source, nothing, or one entry for each level of a mixed tie line', () => {
  assert.equal(expectOf(null), null);
  assert.equal(expectOf({ nothing: true }), null);
  assert.deepEqual(expectOf({ origin: end('a', 3) }), { router: 'a', n: 3 });
  assert.deepEqual(expectOf({ mixed: true, origin: end('a', 1), levels: [end('a', 1), null] }), [{ router: 'a', n: 1 }, null]);
  const mixed = pickState(apiRoute(byName('mixed levels')));
  assert.deepEqual(takeRoute(mixed).ties[0].expect, [{ router: 'a', n: 1 }, null]);
});

test('the last change counts up, and says before when Sode cannot know', () => {
  assert.equal(ago(0), '0 s');
  assert.equal(ago(59_999), '59 s');
  assert.equal(ago(185_000), '3 min 05 s');
  assert.equal(ago(7_620_000), '2 h 07 min');
  const at = '2026-11-23T11:58:00Z';
  const now = Date.parse('2026-11-23T12:00:30Z');
  assert.equal(
    changedText({ changed: at }, now, () => 'x'),
    'Changed 2 min 30 s ago',
  );
  assert.equal(
    changedText({ changed: at, before: true }, now, () => '11:58'),
    'Changed before 11:58',
  );
  assert.equal(
    changedText({ changed: null }, now, () => 'x'),
    'No change known',
  );
});
