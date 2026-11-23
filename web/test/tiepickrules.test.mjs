import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  pickState,
  chosen,
  choices,
  choose,
  keyStep,
  moveStep,
  ready,
  takeRoute,
  expectOf,
  stepsOf,
  ago,
  changedText,
  countUp,
  TICK_MS,
  refresh,
  reopenState,
  pathKey,
  tieMarked,
  hitMarked,
  toggleProtect,
  protectable,
  protectOn,
  heldText,
  reserveAsk,
  confirmReserve,
  reserveText,
  withOneOffs,
  oneOffEnds,
  addOneOff,
  oneOffId,
  chooseOneOff,
  oneOffsOf,
  storedState,
  storedText,
  tieIdsOf,
  oneOffPicked,
} from '../tiepickrules.js';

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
    // A Reserved path is usable: the person confirms one more time (reserveAsk).
    const first = c.want.paths.findIndex((p) => !p.protected);
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

test('the timer ticks each second and stops on close', () => {
  const live = new Map();
  let seq = 0;
  const timers = {
    setInterval: (fn, ms) => (live.set(++seq, { fn, ms }), seq),
    clearInterval: (id) => live.delete(id),
  };
  let ticks = 0;
  const t = countUp(() => ticks++, timers);
  assert.equal(live.size, 1);
  assert.equal([...live.values()][0].ms, TICK_MS);
  [...live.values()][0].fn();
  assert.equal(ticks, 1);
  assert.ok(t.running());
  t.stop();
  t.stop();
  assert.equal(live.size, 0, 'no timer stays after the picker closes');
  assert.ok(!t.running());
});

// A copy of a route with path i changed by fn.
const edit = (route, i, fn) => {
  const r = structuredClone(route);
  fn(r.paths[i]);
  return r;
};

test('a new plan with no change on the chosen path keeps the lockout and the selection', () => {
  const route = apiRoute(byName('mesh: the cheapest'));
  const st = keys(pickState(route), 'j');
  const { st: next, changed } = refresh(st, structuredClone(route));
  assert.ok(!changed);
  assert.deepEqual(ties(next), ['t1']);
  assert.deepEqual(next.marks, st.marks);
  // A change on a path that is not chosen does not count.
  const other = edit(
    route,
    route.paths.findIndex((p) => pathKey(p) === 't2'),
    (p) => (p.cost += 5),
  );
  assert.ok(!refresh(st, other).changed);
});

test('a change to the cost, the Protect state or a link of the chosen path counts and marks the row', () => {
  const route = apiRoute(byName('mesh: the cheapest'));
  const st = pickState(route);
  const at = st.path;
  // A link: the tie line carries another source now.
  let r = refresh(
    st,
    edit(route, at, (p) => (p.hops[0].now.origin = end('a', 9))),
  );
  assert.ok(r.changed);
  assert.deepEqual(ties(r.st), ['t2'], 'the selection never moves by itself');
  assert.ok(tieMarked(r.st, 't2') && !tieMarked(r.st, 't1'));
  // The Protect state: the chosen path is held now, so Take stays off, and the selection stays on it.
  r = refresh(
    st,
    edit(route, at, (p) => {
      p.hops[0].protect = 'device 3';
      p.protected = true;
    }),
  );
  assert.ok(r.changed && !ready(r.st));
  assert.deepEqual(ties(r.st), ['t2']);
  // The cost and a new Affected destination.
  r = refresh(
    st,
    edit(route, at, (p) => {
      p.cost += 1;
      p.affected.push(end('b', 50));
    }),
  );
  assert.ok(r.changed && r.st.marks.affected);
  assert.ok(hitMarked(r.st, { router: 'b', n: 50 }));
  // The destination's own hop.
  r = refresh(
    st,
    edit(route, at, (p) => (p.hops.at(-1).protect = 'device 3')),
  );
  assert.ok(r.changed && r.st.marks.end);
});

test('the paths can move in rank; the selection follows its tie lines, not its place', () => {
  const route = apiRoute(byName('mesh: the cheapest'));
  const st = keys(pickState(route), 'j');
  const turned = { ...structuredClone(route), paths: structuredClone(route.paths).reverse() };
  const { st: next, changed } = refresh(st, turned);
  assert.ok(!changed);
  assert.deepEqual(ties(next), ['t1']);
});

test('a chosen path that the new plan leaves out stays, grey, and cannot go', () => {
  const route = apiRoute(byName('mesh: the cheapest'));
  const st = keys(pickState(route), 'j');
  const less = { ...structuredClone(route), paths: route.paths.filter((p) => pathKey(p) !== 't1') };
  const { st: next, changed } = refresh(st, less);
  assert.ok(changed);
  assert.deepEqual(ties(next), ['t1']);
  assert.ok(!ready(next));
  assert.ok(choices(next, 0).find((c) => c.tie === 't1').gone);
  // The same plan again changes nothing more; the path back is a change.
  assert.ok(!refresh(next, structuredClone(less)).changed);
  const back = refresh(next, structuredClone(route));
  assert.ok(back.changed && ready(back.st));
  assert.deepEqual(ties(back.st), ['t1']);
});

test('after tie_line_changed the picker opens again on the same tie lines, with the changes marked', () => {
  const route = apiRoute(byName('mesh: the cheapest'));
  const st = reopenState(route, ['t3', 't4'], ['t3'], [end('b', 7)]);
  assert.deepEqual(ties(st), ['t3', 't4']);
  assert.ok(tieMarked(st, 't3') && !tieMarked(st, 't4'));
  assert.ok(hitMarked(st, { router: 'b', n: 7 }));
  // Without that path, it opens on the best path.
  assert.deepEqual(ties(reopenState(route, ['zz'])), ['t2']);
});

test('P asks for a Protect on the tie line of the step with focus, only with the Protect right', () => {
  const st = pickState(apiRoute(byName('chain: two tie lines')));
  assert.deepEqual(keyStep(st, 'p'), { st, act: null });
  const one = keyStep(st, 'p', true).st;
  assert.deepEqual(one.protect, ['t1']);
  assert.deepEqual(
    takeRoute(one).ties.map((t) => [t.id, !!t.protect]),
    [
      ['t1', true],
      ['t2', false],
    ],
  );
  const two = keys(one, 'l');
  assert.deepEqual(keyStep(two, 'p', true).st.protect, ['t1', 't2']);
  // A second P stops asking.
  assert.deepEqual(keyStep(one, 'p', true).st.protect, []);
  assert.ok(protectOn(one, 't1') && !protectOn(one, 't2'));
  // The shield of a step acts on that step.
  assert.deepEqual(toggleProtect(st, 1).protect, ['t2']);
  assert.equal(toggleProtect(st, 5), st);
});

test('a tie line that has a Protect cannot get a second one; it stays usable when it carries the source', () => {
  const st = pickState(apiRoute(byName('Protect: a Protected tie line that carries the source')));
  const hop = chosen(st).hops[0];
  assert.ok(!protectable(hop));
  assert.ok(ready(st));
  assert.equal(keyStep(st, 'p', true).st, st);
  // A Protect that someone set while the picker was open drops the ask from the take.
  const line = pickState(apiRoute(byName('line: one tie line')));
  const asked = toggleProtect(line, 0);
  const held = {
    ...asked,
    route: { ...asked.route, paths: [{ ...chosen(asked), hops: [{ ...chosen(asked).hops[0], protect: 'Ana' }, chosen(asked).hops[1]] }] },
  };
  assert.equal(takeRoute(held).ties[0].protect, undefined);
});

test('a Protected tie line says who set it and when; a Reserved one names the panel', () => {
  const clock = (t) => t;
  assert.equal(heldText({ protect: 'Ana', protectSince: '10:42' }, clock), 'Protected by Ana at 10:42');
  assert.equal(heldText({ protect: 'device 12' }, clock), 'Protected by device 12');
  assert.equal(heldText({ reserve: 'Desk 2' }, clock), 'Reserved by panel Desk 2');
  assert.equal(heldText({}, clock), '');
});

test('after tie_line_changed the picker keeps the Protects that the person asked for', () => {
  const route = apiRoute(byName('line: one tie line'));
  assert.deepEqual(reopenState(route, ['t1'], [], [], ['t1']).protect, ['t1']);
  assert.deepEqual(reopenState(route, ['t1']).protect, []);
});

test('a tie line Reserved by a panel is not grey, and Take asks one more time', () => {
  const route = apiRoute(byName('Reserve: t1'));
  route.paths[1].hops[0].reserve = 'Lobby';
  const st = pickState(route);
  assert.deepEqual(ties(st), ['t2']);
  assert.ok(!reserveAsk(st), 'a path with no Reserve asks nothing more');
  const c = choices(st, 0).find((x) => x.tie === 't1');
  assert.ok(c && !c.held, 'the Reserved choice is not held');
  const on = keys(st, 'j');
  assert.deepEqual(ties(on), ['t1'], 'J moves onto the Reserved tie line');
  assert.ok(ready(on));
  assert.ok(reserveAsk(on));
  assert.equal(reserveText(on), 'T1 is Reserved by panel Lobby. The panel can change it at any time. Press Take again to use it.');
  const ok = confirmReserve(on);
  assert.ok(!reserveAsk(ok));
  // Another path asks again.
  assert.ok(!reserveAsk(keys(ok, 'k')));
  assert.ok(reserveAsk({ ...keys(ok, 'k', 'j'), reserveOk: 't2' }));
});

// The One-off tie line case of the fixture: a path through t1 and a path through the One-off tie line "oneoff". The
// plan echoes its id and ends (route.oneOffs, tieroutes.go).
const oneOffCase = () => {
  const c = byName('One-off tie line');
  const ends = { from: { router: c.route.src.router, n: 30 }, to: { router: c.route.dst.router, n: 31 } };
  return { route: { ...apiRoute(c), oneOffs: [{ id: 'oneoff', name: 'One-off', ...ends }] }, ends };
};

test('a One-off tie line goes to the plan on the route with no ties only', () => {
  const ends = { from: { router: 'a', n: 30 }, to: { router: 'b', n: 31 } };
  const routes = [
    { src: { router: 'a', n: 1 }, dst: { router: 'a', n: 2 }, levels: [1] },
    { src: { router: 'a', n: 1 }, dst: { router: 'b', n: 3 }, levels: [1], ties: [{ id: 't1' }] },
    { src: { router: 'a', n: 1 }, dst: { router: 'b', n: 4 }, levels: [1] },
  ];
  const out = withOneOffs(routes, [ends]);
  assert.deepEqual(
    out.map((r) => r.oneOffs || null),
    [null, null, [ends]],
  );
  assert.equal(withOneOffs(routes, []), routes);
  assert.deepEqual(addOneOff(addOneOff([], ends), { ...ends }), [ends]);
});

test('the typed numbers of a One-off tie line are numbers as the routers show them', () => {
  const route = { src: { router: 'a', n: 1 }, dst: { router: 'b', n: 2 } };
  const off = (uid) => (uid === 'a' ? 1 : 0);
  assert.deepEqual(oneOffEnds(route, '12', ' 6 ', off), { ends: { from: { router: 'a', n: 11 }, to: { router: 'b', n: 6 } } });
  assert.match(oneOffEnds(route, '', '6', off).why, /destination/);
  assert.match(oneOffEnds(route, '0', '6', off).why, /destination/);
  assert.match(oneOffEnds(route, '3', 'x', off).why, /source/);
});

test('the picker chooses the path through the One-off tie line, and the take sends its ends', () => {
  const { route, ends } = oneOffCase();
  assert.equal(oneOffId(route, ends), 'oneoff');
  const st = chooseOneOff(pickState(route), ends);
  assert.deepEqual(ties(st), ['oneoff']);
  assert.equal(ready(st), true);
  const t = takeRoute(st);
  assert.deepEqual(t.ties[0].from, ends.from);
  assert.deepEqual(t.ties[0].to, ends.to);
  assert.equal(t.ties[0].id, undefined);
  assert.deepEqual(oneOffsOf(t), [ends]);
  // A plan that does not have it changes nothing.
  assert.equal(chooseOneOff(pickState(apiRoute(byName('line: one tie line'))), ends).path, 0);
});

test('a One-off tie line with an unknown upstream destination goes with no expect', () => {
  const { route, ends } = oneOffCase();
  const st = chooseOneOff(pickState(route), ends);
  const p = chosen(st);
  const unknown = {
    ...st,
    route: { ...st.route, paths: st.route.paths.map((x) => (x === p ? { ...p, hops: [{ ...p.hops[0], now: { unknown: 'a' } }, p.hops[1]] } : x)) },
  };
  assert.equal('expect' in takeRoute(unknown).ties[0], false);
  assert.equal('expect' in takeRoute(st).ties[0], true);
});

// A preset's tie confirm page opens on the stored tie lines, even when they are not the cheapest or are Protected. Take
// stays off on a Protected one until the person chooses again or clears the route (docs/tielines-design.md, Presets).
test('a preset opens the picker on its stored tie lines and says what it found', () => {
  const route = apiRoute(byName('mesh: the cheapest'));
  const st = storedState(route, ['t3', 't4']);
  assert.deepEqual(ties(st), ['t3', 't4']);
  assert.deepEqual(tieIdsOf(st), ['t3', 't4']);
  assert.deepEqual(storedText(st), { text: 'This preset stores these tie lines.', warn: false });
  assert.match(storedText(keys(st, 'k', 'k')).text, /other tie lines than the preset stores/);
  const gone = storedState(route, ['zz']);
  assert.deepEqual(ties(gone), ['t2']);
  assert.ok(storedText(gone).warn);
  const held = storedState(apiRoute(byName('Protect: t1 costs less')), ['t1']);
  assert.deepEqual(ties(held), ['t1']);
  assert.ok(!ready(held), 'Take stays off');
  assert.match(storedText(held).text, /Protected/);
  assert.ok(ready(choose(held, 0, 't2')), 'another tie line can go');
  assert.equal(storedText(pickState(route)).text, '', 'a picker that is not a preset says nothing');
  assert.ok(!oneOffPicked(st));
});
