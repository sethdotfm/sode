// The details of the Server and Router status, shown on hover or focus. With Log rights they add the Router link's
// last 24 hours. Patchbay tips hold text only, so this is its own element. It sits inside the status, so the pointer
// can move onto it. The phone keeps the native title.
import { el, S, on, wsSend, linkList } from './core.js';
import { layout, onLayout } from './layout.js';
import { slotFor, summary } from './uptime.js';
import { uptimeBars } from './uptimebars.js';
import { logReadable, spanText, routerUpSince } from './statusinforules.js';
import { linkLabel } from './routerlinkrules.js';

const DAY = 86_400_000;
// Above log.js's ids, which count up from 1, so the two uptimeResult readers on /log never take each other's reply.
const ID_BASE = 1_000_000_000;
// The pointer crosses the gap between the status and the details in this time.
const LEAVE_MS = 200;

let access = null;
on('access', (m) => {
  access = m;
});
on('conn', () => {
  if (S.wsState !== 'up') access = null;
});

let seq = 0;
let nextId = ID_BASE;
const boxes = new WeakMap();

// One details element per status root. paint(title) gives the words the phone's native title shows.
export function statusInfo(root) {
  let b = boxes.get(root);
  if (!b) boxes.set(root, (b = makeBox(root)));
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

function makeBox(root) {
  const pop = el('div', 'sode-statusinfo');
  pop.id = `sode-statusinfo-${++seq}`;
  pop.setAttribute('role', 'tooltip');
  pop.hidden = true;
  root.append(pop);
  let title = '';
  let open = false;
  let shut = false; // Escape closed it; it stays closed until the pointer and focus leave
  let leaveTimer = 0;
  let tick = 0;
  let health = null;
  // The uptime of each router by its UID, and the router of each uptime request by its id.
  const ups = new Map();
  const wants = new Map();

  const phone = () => layout() === 'phone';
  const hovered = () => root.matches(':hover');
  const focused = () => root.matches(':focus-within');

  function sync() {
    if (phone()) {
      close();
      root.removeAttribute('tabindex');
      root.removeAttribute('aria-describedby');
      if (title) root.title = title;
      return;
    }
    root.removeAttribute('title');
    root.tabIndex = 0;
    root.setAttribute('aria-describedby', pop.id);
    if (open) render();
  }

  function row(label, ...kids) {
    return el('div', 'sode-statusinfo__row', el('span', 'sode-statusinfo__label', label), el('span', 'sode-statusinfo__value', ...kids));
  }

  function render() {
    const srv = S.wsState === 'up' ? 'Connected' : S.wsState === 'connecting' ? 'Connecting' : 'Disconnected';
    const run = health ? spanText(health.secs + (Date.now() - health.at) / 1000) : S.wsState === 'up' ? 'Loading…' : 'Not known';
    const rows = [row('Server link', srv), row('Server run time', run)];
    const list = linkList();
    const parts = [el('div', 'sode-statusinfo__rows', ...rows)];
    // Each router: its link rows, then its 24-hour strip.
    for (const r of list) {
      const own = [row(linkLabel(r, list.length), r.link ? { up: 'Connected', connecting: 'Connecting', down: 'Down' }[r.link] || r.link : 'Not known')];
      if (r.addr) own.push(row('Address', el('span', 'sode-statusinfo__num', r.addr)));
      if (r.err) own.push(row('Error', r.err));
      parts.push(el('div', 'sode-statusinfo__rows', ...own));
      const data = ups.get(r.uid);
      if (data && S.wsState === 'up') parts.push(strip(data, r, list.length));
    }
    pop.replaceChildren(...parts);
  }

  function strip(data, r, n) {
    const nodes = [];
    const name = n === 1 ? 'Router' : `Router ${r.prefix}`;
    const since = r.link === 'up' ? routerUpSince(data) : null;
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
    bars.dataset.router = r.uid;
    bars.setAttribute('role', 'img');
    bars.setAttribute('aria-label', `${name} uptime, last 24 hours: ${text}`);
    nodes.push(bars, el('div', 'sode-statusinfo__sum', `Last 24 hours: ${text}`));
    return el('div', 'sode-statusinfo__strip', ...nodes);
  }

  // One uptime request for each router. The reply echoes the id, and wants gives its router.
  function ask() {
    health = null;
    ups.clear();
    wants.clear();
    askHealth().then((h) => {
      health = h;
      if (open) render();
    });
    if (!logReadable(access, S.cfg?.locks)) return;
    const now = Date.now();
    for (const r of linkList()) {
      const id = ++nextId;
      wants.set(id, r.uid);
      wsSend({ t: 'uptime', id, router: r.uid, from: now - DAY, to: now, slot: slotFor(now - DAY, now) });
    }
  }

  function show() {
    clearTimeout(leaveTimer);
    if (open || shut || phone()) return;
    open = true;
    ask();
    render();
    pop.classList.remove('sode-statusinfo--start');
    pop.hidden = false;
    // A status with no room on its left (the setup wizard's bar) opens the details rightward.
    if (pop.getBoundingClientRect().left < 0) pop.classList.add('sode-statusinfo--start');
    tick = setInterval(render, 30_000);
  }

  function close() {
    clearTimeout(leaveTimer);
    clearInterval(tick);
    open = false;
    wants.clear();
    pop.hidden = true;
  }

  root.addEventListener('pointerenter', show);
  root.addEventListener('focusin', (e) => {
    if (!root.contains(e.relatedTarget)) shut = false;
    show();
  });
  root.addEventListener('pointerleave', () => {
    if (focused()) return;
    shut = false;
    leaveTimer = setTimeout(close, LEAVE_MS);
  });
  root.addEventListener('focusout', () => {
    if (hovered()) return;
    shut = false;
    close();
  });
  root.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || !open) return;
    e.preventDefault();
    e.stopPropagation();
    shut = true;
    close();
  });
  // A hover opened it, so focus can be anywhere: Escape closes it and still does its usual job.
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || !open || focused()) return;
    shut = true;
    close();
  });

  on('uptimeResult', (m) => {
    if (!open || !wants.has(m.id)) return;
    ups.set(wants.get(m.id), { start: m.start, trans: m.trans, counts: m.counts, from: m.from, to: m.to, slot: m.slot, now: m.now });
    render();
  });
  on('conn', () => open && render());
  on('access', () => open && !logReadable(access, S.cfg?.locks) && (ups.clear(), render()));
  onLayout(sync);

  return {
    paint(t) {
      title = t;
      sync();
    },
  };
}
