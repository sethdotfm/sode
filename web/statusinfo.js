// The router list of the status bar. A click or a press on the Routers icon (connview.js paintRouters) opens it. It
// stays open until Escape, a click outside it or a second click on the icon. It gives the Server link and its run
// time, then one line per router, lost routers first (routerlinkrules.js listRows). A line opens to show the address
// and the error. With Log rights it also shows the last 24 hours. Each line has links to Router settings and the
// Communication Log. A find box shows from FIND_FROM routers. The Degraded note's Show routers button opens the list
// with the bus event 'routerListOpen'.
import { el, S, on, emit, wsSend, linkList } from './core.js';
import { layout } from './layout.js';
import { slotFor, summary } from './uptime.js';
import { uptimeBars } from './uptimebars.js';
import { logReadable, spanText, routerUpSince } from './statusinforules.js';
import { listRows, FIND_FROM } from './routerlinkrules.js';
import { readTab, writeTab } from './store.js';

const DAY = 86_400_000;
// Above log.js's ids, which count up from 1, so the two uptimeResult readers on /log never take each other's reply.
const ID_BASE = 1_000_000_000;
// Router settings from a page other than Home: Home reads this after its hello and goes to that router's row.
const JUMP_KEY = 'sode-router-jump';

let access = null;
on('access', (m) => {
  access = m;
});
on('conn', () => {
  if (S.wsState !== 'up') access = null;
});
const admin = () => access?.role === 'administrator' && !access.locked;

let seq = 0;
let nextId = ID_BASE;
const boxes = new WeakMap();

// One router list per status root. open is the Routers button that opens it; a paint can give it again.
export function statusInfo(root, open) {
  let b = boxes.get(root);
  if (!b) boxes.set(root, (b = makeBox(root)));
  b.trigger(open);
  return b;
}

const fmtTime = (t) => {
  const d = new Date(t);
  const p = (n) => String(n).padStart(2, '0');
  return `${p(d.getHours())}:${p(d.getMinutes())}`;
};

// api/health, not healthz: only it has uptimeSeconds. It answers 503 while the router is down, with the same body.
async function askHealth() {
  try {
    const r = await fetch('api/health', { cache: 'no-store' });
    const m = await r.json();
    return typeof m.uptimeSeconds === 'number' ? { secs: m.uptimeSeconds, at: Date.now() } : null;
  } catch {
    return null;
  }
}

// Settings > Routers, scrolled to the row of the router. Home only.
function jumpTo(uid) {
  const list = document.getElementById('routers-list');
  if (!list) return;
  const first = !S.routers.length || S.routers[0].uid === uid;
  const row = first ? list.firstElementChild : [...list.children].find((n) => n.dataset.router === uid);
  if (!row) return;
  row.scrollIntoView({ block: 'center' });
  row.querySelector('input, select, button')?.focus({ preventScroll: true });
}

function routerSettings(uid, e) {
  if (!document.getElementById('settings-view')) {
    writeTab(JUMP_KEY, uid);
    return; // the link opens Home at #settings
  }
  e.preventDefault();
  emit('settingsOpen');
  requestAnimationFrame(() => jumpTo(uid));
}

on('hello', () => {
  const uid = readTab(JUMP_KEY);
  if (uid == null || !document.getElementById('settings-view')) return;
  writeTab(JUMP_KEY, null);
  setTimeout(() => {
    if (document.documentElement.dataset.view !== 'settings') emit('settingsOpen');
    requestAnimationFrame(() => jumpTo(uid));
  });
});

function makeBox(root) {
  const pop = el('div', { class: 'sode-statusinfo', id: `sode-statusinfo-${++seq}`, role: 'dialog', 'aria-label': 'Router list' });
  pop.hidden = true;
  const server = el('div', 'sode-statusinfo__rows');
  const find = el('input', {
    class: 'pb-input sode-routerlist__find',
    type: 'search',
    'aria-label': 'Find a router',
    placeholder: 'Find a router',
    autocomplete: 'off',
    spellcheck: false,
  });
  const list = el('ul', 'sode-routerlist');
  const none = el('p', 'sode-routerlist__none', 'No router matches.');
  pop.append(server, find, list, none);
  root.append(pop);
  let btn = null;
  let health = null;
  let tick = 0;
  // The row of each router by its UID, the open rows, the uptime of each router and the router of each uptime ask.
  const rows = new Map();
  const opened = new Set();
  const ups = new Map();
  const wants = new Map();
  const isOpen = () => !pop.hidden;

  function row(label, ...kids) {
    return el('div', 'sode-statusinfo__row', el('span', 'sode-statusinfo__label', label), el('span', 'sode-statusinfo__value', ...kids));
  }

  function paintServer() {
    const srv = S.wsState === 'up' ? 'Connected' : S.wsState === 'connecting' ? 'Connecting' : 'Disconnected';
    const run = health ? spanText(health.secs + (Date.now() - health.at) / 1000) : S.wsState === 'up' ? 'Loading…' : 'Not known';
    server.replaceChildren(row('Server link', srv), row('Server run time', run));
  }

  function makeRow(uid) {
    const name = el('span', 'sode-routerlist__name');
    const state = el('span', 'sode-routerlist__state');
    const head = el('button', { class: 'sode-routerlist__head', type: 'button' }, name, state);
    const body = el('div', { class: 'sode-routerlist__body', id: `${pop.id}-r${rows.size}` });
    head.setAttribute('aria-controls', body.id);
    head.addEventListener('click', () => {
      if (opened.has(uid)) opened.delete(uid);
      else {
        opened.add(uid);
        askUptime(uid);
      }
      paint();
    });
    const r = { li: el('li', 'sode-routerlist__item', head, body), head, name, state, body };
    r.li.dataset.router = uid;
    rows.set(uid, r);
    return r;
  }

  function paintBody(r, x, n) {
    const kids = [];
    const facts = [];
    if (x.r.addr) facts.push(row('Address', el('span', 'sode-statusinfo__num', x.r.addr)));
    if (x.r.err) facts.push(row('Error', x.r.err));
    if (facts.length) kids.push(el('div', 'sode-statusinfo__rows', ...facts));
    if (x.note) kids.push(el('p', 'sode-routerlist__note', x.note));
    const data = ups.get(x.r.uid);
    if (data && S.wsState === 'up') kids.push(strip(data, x, n));
    const links = [];
    // Settings > Routers needs an Administrator and a computer (the phone has no Routers topic).
    if (admin() && layout() !== 'phone') {
      const a = el('a', { class: 'pb-btn pb-btn--outline pb-btn--sm', href: './#settings' }, 'Router settings');
      a.addEventListener('click', (e) => {
        close(false);
        routerSettings(x.r.uid, e);
      });
      links.push(a);
    }
    links.push(el('a', { class: 'pb-btn pb-btn--outline pb-btn--sm', href: 'log' }, 'Communication Log'));
    kids.push(el('div', 'sode-routerlist__links', ...links));
    r.body.replaceChildren(...kids);
  }

  function strip(data, x, n) {
    const nodes = [];
    const name = n === 1 ? 'Router' : `Router ${x.r.prefix}`;
    const since = x.r.link === 'up' ? routerUpSince(data) : null;
    if (since)
      nodes.push(
        el(
          'div',
          'sode-statusinfo__since',
          since.over ? `${name} link up for more than 24 hours` : `${name} link up for ${spanText((data.now - since.since) / 1000)}`,
        ),
      );
    const text = summary(data);
    const bars = el('div', 'sode-up__bars sode-up__bars--sm', ...uptimeBars(data, { fmt: fmtTime }));
    bars.dataset.router = x.r.uid;
    bars.setAttribute('role', 'img');
    bars.setAttribute('aria-label', `${name} uptime, last 24 hours: ${text}`);
    nodes.push(bars, el('div', 'sode-statusinfo__sum', `Last 24 hours: ${text}`));
    return el('div', 'sode-statusinfo__strip', ...nodes);
  }

  function paint() {
    paintServer();
    const all = linkList();
    find.hidden = all.length < FIND_FROM;
    const shown = listRows(all, find.hidden ? '' : find.value);
    const want = shown.map((x) => {
      const r = rows.get(x.r.uid) || makeRow(x.r.uid);
      r.name.textContent = x.name;
      r.state.textContent = x.word;
      if (x.state) r.state.dataset.state = x.state;
      else delete r.state.dataset.state;
      const on = opened.has(x.r.uid);
      r.head.setAttribute('aria-expanded', String(on));
      r.body.hidden = !on;
      if (on) paintBody(r, x, all.length);
      return r.li;
    });
    // Put the rows in order. A moved row loses focus, so focus goes back after.
    const same = want.length === list.children.length && want.every((li, i) => list.children[i] === li);
    if (!same) {
      const had = document.activeElement;
      list.replaceChildren(...want);
      if (had && list.contains(had) && document.activeElement !== had) had.focus({ preventScroll: true });
    }
    none.hidden = want.length > 0;
  }

  // One uptime ask for one router, only for an open row. The reply echoes the id, and wants gives its router.
  function askUptime(uid) {
    if (!logReadable(access, S.cfg?.locks)) return;
    const now = Date.now();
    const id = ++nextId;
    wants.set(id, uid);
    wsSend({ t: 'uptime', id, router: uid, from: now - DAY, to: now, slot: slotFor(now - DAY, now) });
  }

  function show() {
    if (isOpen()) return;
    health = null;
    ups.clear();
    wants.clear();
    askHealth().then((h) => {
      health = h;
      if (isOpen()) paintServer();
    });
    for (const uid of opened) askUptime(uid);
    paint();
    pop.classList.remove('sode-statusinfo--start');
    pop.hidden = false;
    btn?.setAttribute('aria-expanded', 'true');
    // A status with no room on its left (the setup wizard's bar) opens the list rightward.
    if (pop.getBoundingClientRect().left < 0) pop.classList.add('sode-statusinfo--start');
    tick = setInterval(paint, 30_000);
    (find.hidden ? list.querySelector('.sode-routerlist__head') : find)?.focus({ preventScroll: true });
  }

  // back: focus goes back to the Routers button.
  function close(back) {
    if (!isOpen()) return;
    clearInterval(tick);
    wants.clear();
    pop.hidden = true;
    btn?.setAttribute('aria-expanded', 'false');
    if (back) btn?.focus();
  }

  // Up and Down move between the find box and the rows.
  function step(e) {
    const stops = [find, ...list.querySelectorAll('.sode-routerlist__head')].filter((n) => !n.hidden);
    const i = stops.indexOf(document.activeElement);
    if (i < 0) return;
    e.preventDefault();
    stops[Math.max(0, Math.min(stops.length - 1, i + (e.key === 'ArrowDown' ? 1 : -1)))].focus();
  }

  pop.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      close(true);
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') step(e);
  });
  find.addEventListener('input', paint);
  // A click outside the list and its button closes it, and focus stays where the click put it.
  document.addEventListener('pointerdown', (e) => {
    if (isOpen() && !pop.contains(e.target) && !btn?.contains(e.target)) close(false);
  });
  // Escape with focus outside the list closes it and still does its usual job.
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && isOpen() && !pop.contains(document.activeElement)) close(false);
  });
  on('routerListOpen', () => {
    if (btn?.getClientRects().length) show();
  });
  on('uptimeResult', (m) => {
    if (!isOpen() || !wants.has(m.id)) return;
    ups.set(wants.get(m.id), { start: m.start, trans: m.trans, counts: m.counts, from: m.from, to: m.to, slot: m.slot, now: m.now });
    paint();
  });
  on('conn', () => isOpen() && paint());
  on('access', () => {
    if (!isOpen()) return;
    if (!logReadable(access, S.cfg?.locks)) ups.clear();
    paint();
  });

  const toggle = () => (isOpen() ? close(false) : show());
  return {
    trigger(open) {
      if (!open || open === btn) return;
      btn = open;
      btn.setAttribute('aria-controls', pop.id);
      btn.setAttribute('aria-expanded', String(isOpen()));
      btn.addEventListener('click', toggle);
    },
  };
}
