// Link state with more than one router (docs/tielines-design.md, Link state on the page). The page's Healthy, Degraded
// and Down, the Degraded note's words, and the /log rules for each router.
import test from 'node:test';
import assert from 'node:assert/strict';
import { routerLinks, routerLink, routerMarks, lost, wholeState, degradedLine, DEGRADED_NAMES_MAX } from '../routerlinkrules.js';
import { logPasses, pressRouter, routerWord, linkResult } from '../logfilter.js';
import { logCSV, statesCSV, minutesCSV, LOG_COLS } from '../logfile.js';

const up = { wsState: 'up' };
const link = (prefix, state, err = '') => routerLink(up, `u${prefix}`, prefix, '', '10.0.0.1:8910', state, err);

test('routerLinks: every router from of(uid), the first one by a blank UID', () => {
  const s = {
    wsState: 'up',
    routers: [
      { uid: 'ua', prefix: 'A', title: 'Main' },
      { uid: 'ub', prefix: 'B', title: 'Truck', address: '10.0.0.2:8910' },
    ],
  };
  const of = (uid) =>
    ({
      '': { uid: 'ua', prefix: 'A', title: 'Main', link: 'up', addr: '10.0.0.1:8910', err: '' },
      ub: { uid: 'ub', link: 'down', addr: '', err: 'refused' },
    })[uid];
  const list = routerLinks(s, of);
  assert.deepEqual(
    list.map((r) => [r.uid, r.prefix, r.title, r.addr, r.state, r.word, r.err]),
    [
      ['ua', 'A', 'Main', '10.0.0.1:8910', 'ok', 'connected', ''],
      ['ub', 'B', 'Truck', '10.0.0.2:8910', 'error', 'down', 'refused'],
    ],
  );
  assert.equal(routerLinks({ wsState: 'up' }, of).length, 1, 'before the hello, only the first router');
  assert.equal(routerLinks({ ...s, routers: s.routers.slice(0, 1) }, of).length, 1);
});

test('a link that connects again after a failure is lost and red; a first connect is not', () => {
  const again = link('B', 'connecting', 'refused');
  assert.deepEqual([again.state, again.word, lost(again)], ['error', 'down', true]);
  const first = link('B', 'connecting');
  assert.deepEqual([first.state, lost(first)], ['warn', false]);
  assert.equal(lost(link('B', 'down')), true);
  assert.equal(lost(routerLink({ wsState: 'down' }, 'x', 'B', '', '', 'down', 'refused')), false, 'no server link: nothing known');
});

test('wholeState: Healthy, Degraded and Down', () => {
  assert.equal(wholeState([link('A', 'up'), link('B', 'up')]), 'healthy');
  assert.equal(wholeState([link('A', 'up'), link('B', 'down')]), 'degraded');
  assert.equal(wholeState([link('A', 'down'), link('B', 'connecting', 'refused')]), 'down');
  assert.equal(wholeState([link('A', 'up'), link('B', 'connecting')]), '', 'a first connect is neither');
  assert.equal(wholeState([link('A', 'down')]), 'down');
});

test('Degraded: only the down router is red', () => {
  const marks = routerMarks([link('A', 'up'), link('B', 'down')]);
  assert.deepEqual(
    marks.map((m) => [m.label, m.state]),
    [
      ['Router A', 'ok'],
      ['Router B', 'error'],
    ],
  );
});

test('the Degraded note names the router and its destinations', () => {
  const r = { prefix: 'B', title: 'Truck' };
  const d = (n) => ({ label: `B.${n} MON ${n}` });
  assert.equal(degradedLine(r, []), 'Router B Truck is down. You cannot patch its destinations now.');
  assert.equal(degradedLine({ prefix: 'B', title: 'Router B' }, []), 'Router B is down. You cannot patch its destinations now.', 'no title said twice');
  assert.equal(degradedLine(r, [d(1)]), 'Router B Truck is down. You cannot patch B.1 MON 1 now.');
  assert.equal(degradedLine(r, [d(1), d(2)]), 'Router B Truck is down. You cannot patch its 2 destinations now: B.1 MON 1, B.2 MON 2.');
  const many = Array.from({ length: DEGRADED_NAMES_MAX + 1 }, (_, i) => d(i + 1));
  assert.equal(degradedLine({ prefix: 'B' }, many), `Router B is down. You cannot patch its ${many.length} destinations now: B.1 MON 1 to B.9 MON 9.`);
});

test('/log: the Routers filter keeps the rows of the picked routers and every row that names no router', () => {
  const f = { kinds: [], problems: false, src: '', dst: '', from: null, to: null, routers: ['ub'] };
  const at = '2026-11-23T10:00:00Z';
  assert.equal(logPasses({ kind: 'link', result: 'down', router: 'ub', time: at, cmd: -1, src: -1, dst: -1 }, f), true);
  assert.equal(logPasses({ kind: 'link', result: 'down', router: 'ua', time: at, cmd: -1, src: -1, dst: -1 }, f), false);
  assert.equal(logPasses({ kind: 'server', result: 'start', time: at, cmd: -1, src: -1, dst: -1 }, f), true);
  assert.equal(logPasses({ kind: 'link', result: 'down', router: 'ua', time: at, cmd: -1, src: -1, dst: -1 }, { ...f, routers: [] }), true);
});

test('/log: the Routers chips', () => {
  const uids = ['ua', 'ub', 'uc'];
  assert.deepEqual(pressRouter([], 'ub', uids), ['ub'], 'from every router, a press shows only that one');
  assert.deepEqual(pressRouter(['ub'], 'uc', uids), ['ub', 'uc']);
  assert.deepEqual(pressRouter(['ub', 'uc'], 'ua', uids), [], 'every router picked is every router');
  assert.deepEqual(pressRouter(['ub'], 'ub', uids), [], 'none picked is every router');
  assert.deepEqual(pressRouter(['gone'], 'ua', uids), ['ua'], 'a router that is gone does not count');
});

test('/log: a link row names its router with two or more routers', () => {
  const routers = [
    { uid: 'ua', prefix: 'A' },
    { uid: 'ub', prefix: 'B' },
  ];
  assert.equal(routerWord(routers, 'ub'), 'Router B');
  assert.equal(routerWord(routers.slice(0, 1), 'ua'), 'Router');
  assert.equal(linkResult('Router down', { kind: 'link', router: 'ub' }, routers), 'Router B down');
  assert.equal(linkResult('Router down', { kind: 'link', router: 'ub' }, routers.slice(0, 1)), 'Router down');
  assert.equal(linkResult('Started', { kind: 'server' }, routers), 'Started');
});

test('/log downloads: a router column only with two or more routers', () => {
  const lines = (t) => t.split('\r\n').filter(Boolean);
  const word = (uid) => ({ ua: 'A', ub: 'B' })[uid] || uid;
  assert.equal(lines(logCSV([], {}))[0], LOG_COLS.join(','));
  assert.equal(lines(logCSV([], { router: word }))[0], [...LOG_COLS, 'router'].join(','));
  const row = { time: 't', seq: 1, kind: 'link', result: 'down', router: 'ub', cmd: -1, wire: -1, src: -1, dst: -1, hex: '' };
  assert.ok(lines(logCSV([row], { router: word }))[1].endsWith(',B'));
  assert.ok(lines(logCSV([{ ...row, kind: 'server', router: undefined }], { router: word }))[1].endsWith(','));
  const t = Date.UTC(2026, 10, 23, 10);
  assert.equal(
    statesCSV([{ t, s: 'down', router: 'ub' }], word),
    `time,state,note,router\r\n2026-11-23T10:00:00.000Z,down,Router link down or connecting,B\r\n`,
  );
  assert.equal(statesCSV([{ t, s: 'up', router: 'ub' }]), `time,state,note\r\n2026-11-23T10:00:00.000Z,up,Router link up\r\n`);
  assert.equal(minutesCSV([{ m: t / 60_000, bad: 1, slow: 0, router: 'ua' }], word), `minute,bad,slow,router\r\n2026-11-23T10:00:00.000Z,1,0,A\r\n`);
});
