import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as P from '../logpager.js';

// A fake server: seqs 0..total-1, a page is the n below before, newest first.
function server(total) {
  return ({ before, n }) => {
    const top = before < 0 ? total : before;
    const entries = [];
    for (let s = top - 1; s >= 0 && entries.length < n; s--) entries.push({ seq: s, time: '2026-10-29T10:00:00Z', kind: s % 10 === 0 ? 'connect' : 'keepalive' });
    return { before, entries, more: top - entries.length > 0, total };
  };
}
const isConnect = (e) => e.kind === 'connect';
const shown = (p) => p.list.filter(isConnect).length;

// Drives the loop log.js runs: send, reply, ask again.
function run(p, srv, req) {
  let sent = 0;
  while (req) {
    sent++;
    const r = p.receive(srv(req));
    assert.ok(r);
    req = p.next(shown(p));
  }
  return sent;
}

test('open reads on until PAGE rows are shown, not PAGE entries', () => {
  const p = P.newPager();
  const srv = server(5000);
  run(p, srv, p.open());
  assert.ok(shown(p) >= P.PAGE, `shown ${shown(p)}`);
  assert.ok(p.list.length > P.PAGE);
});

test('Load older with a page of hidden entries still adds rows', () => {
  const p = P.newPager();
  const srv = server(5000);
  run(p, srv, p.open());
  const before = shown(p);
  const req = p.older(before);
  assert.equal(req.n, P.PAGE);
  run(p, srv, req);
  assert.ok(shown(p) >= before + P.PAGE);
});

test('stops at the page cap while everything is hidden, and at the log start', () => {
  const p = P.newPager();
  const hidden = server(100_000);
  const srv = (r) => { const m = hidden(r); m.entries.forEach((e) => { e.kind = 'keepalive'; }); return m; };
  assert.equal(run(p, srv, p.open()), P.AUTO_PAGES);
  assert.ok(p.older(0));
  const q = P.newPager();
  run(q, server(30), q.open());
  assert.equal(q.more, false);
  assert.equal(q.older(3) && q.next(3), null);
});

test('a reply for another request is ignored and leaves the pending one', () => {
  const p = P.newPager();
  const srv = server(300);
  p.open();
  assert.equal(p.receive(srv({ before: 50, n: 10 })), null);
  assert.equal(p.loading, true);
  assert.ok(p.receive(srv({ before: -1, n: 100 })));
  assert.equal(p.loading, false);
});

test('a new connection drops the old one\'s reply; Load older works after', () => {
  const p = P.newPager();
  const srv = server(300);
  run(p, srv, p.open());
  const stale = p.older(0);
  p.close();
  assert.equal(p.receive(srv(stale)), null);
  const req = p.open();
  assert.equal(p.receive(srv(stale)), null);
  assert.ok(p.receive(srv(req)));
  assert.equal(p.loading, false);
  assert.ok(p.older(0));
});

test('live entries before the newest page are merged, not duplicated', () => {
  const p = P.newPager();
  const srv = server(300);
  const req = p.open();
  p.live({ entries: [{ seq: 299, time: '2026-10-29T10:00:00Z', kind: 'connect' }, { seq: 300, time: '2026-10-29T10:00:00Z', kind: 'connect' }] });
  p.receive(srv(req));
  assert.deepEqual(p.list.map((e) => e.seq).slice(0, 3), [300, 299, 298]);
  assert.equal(new Set(p.list.map((e) => e.seq)).size, p.list.length);
});

test('a time range keeps reading regardless of the cap', () => {
  const p = P.newPager();
  const srv = server(100_000);
  p.open();
  let req = null, n = 0;
  p.receive(srv({ before: -1, n: 100 }));
  while ((req = p.back()) && n < 50) { p.receive(srv(req)); n++; }
  assert.equal(n, 50);
});
