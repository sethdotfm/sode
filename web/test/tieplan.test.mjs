import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

// Shared with internal/tieplan/tieplan_test.go, which makes each want from the planner. The page never computes
// paths: it reads this shape from the server. A change to the shape fails here and in Go.
const { plans, vias } = JSON.parse(readFileSync(new URL('./tieplan_cases.json', import.meta.url), 'utf8'));

const keys = (o) => Object.keys(o).sort();
const PATH = ['affected', 'changedLately', 'cost', 'hops', 'loop', 'newest', 'protected', 'reserved', 'unknown'];
const HOP = ['before', 'changed', 'dst', 'levels', 'now', 'protect', 'reserve', 'router', 'sends', 'src', 'tie'];
const CHAIN = ['loop', 'mixed', 'nothing', 'origin', 'unknown', 'via'];

const bit = (b) => (b ? 1 : 0);
const time = (t) => (t ? Date.parse(t) : 0);
// The ranking of docs/tielines-design.md, without the tie-breaks.
const rank = (a, b) =>
  bit(a.unknown.length > 0) - bit(b.unknown.length > 0) ||
  bit(a.protected || a.reserved) - bit(b.protected || b.reserved) ||
  a.cost - b.cost ||
  bit(a.changedLately) - bit(b.changedLately) ||
  time(a.newest) - time(b.newest);

test('every plan has the shape the page reads', () => {
  for (const c of plans) {
    assert.deepEqual(keys(c.want), ['paths', 'reasons'], c.name);
    for (const p of c.want.paths) {
      assert.deepEqual(keys(p), PATH, c.name);
      for (const h of p.hops) {
        assert.deepEqual(keys(h), HOP, c.name);
        if (h.now) assert.deepEqual(keys(h.now), CHAIN, c.name);
      }
      assert.equal(p.hops.at(-1).tie, '', `${c.name}: the last hop feeds no tie line`);
      assert.deepEqual(p.hops.at(-1).levels, c.route.levels, `${c.name}: the last hop switches the typed levels`);
    }
    if (c.want.paths.length === 0) assert.ok(c.want.reasons.length > 0, `${c.name}: no path names a reason`);
  }
  for (const c of vias) assert.deepEqual(keys(c.want), CHAIN, c.name);
});

test('paths come in rank order', () => {
  for (const c of plans) {
    const ps = c.want.paths;
    for (let i = 1; i < ps.length; i++) assert.ok(rank(ps[i - 1], ps[i]) <= 0, `${c.name}: path ${i}`);
  }
});

test('cost is the Affected destinations plus the hop weight for each hop that sends upstream', () => {
  for (const c of plans) {
    for (const p of c.want.paths) {
      const upstream = p.hops.slice(0, -1).filter((h) => h.sends).length;
      assert.equal(p.cost, p.affected.length + c.in.hopWeight * upstream, c.name);
    }
  }
});

test('the fixture covers each shape that the design names', () => {
  const names = plans.map((c) => c.name).join('\n');
  for (const word of ['line', 'mesh', 'loop', 'chain', 'down router', 'unknown tally', 'mixed levels', 'Protect', 'Reserve', '5-minute', 'One-off', 'one take'])
    assert.match(names, new RegExp(word), word);
});
