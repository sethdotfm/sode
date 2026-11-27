// Settings > Routers > Protects (Administrator): every Protect and Router lock that each router reports, in one list
// of fixed height with a big scroll bar. A group for each router has a heading that stays in view. A find box, the
// kind and router filters and a count line narrow it. Free on a row turns the row into the Confirm, so the list does
// not move. The row and the status line under the list then show the result; the banner of Home is out of sight
// here. Free Sode's Protects asks which routers (settingsprotectsfree.js). Rules in protectlistrules.js.

import { $, el, S, on, icon, dstItem, numText, routerData, protectsOf } from './core.js';
import { levelText, allLevels } from './levelview.js';
import { holdLabel, holdShort, holdLook } from './protectrules.js';
import { protectPlan, sendProtect, clearSodeProtects, everyHold } from './protects.js';
import { btn, opt } from './formbits.js';
import { churnKey, lockoutMs } from './lockrules.js';
import { dialogShown } from './dialogkeys.js';
import {
  KINDS,
  ALL_ROUTERS,
  ROUTER_BUTTONS_MAX,
  rowKey,
  countLine,
  kindCounts,
  routerCounts,
  groupsOf,
  groupCount,
  freeBlock,
  staleNote,
  freeResult,
  rowResult,
  rowStatus,
} from './protectlistrules.js';

const DONE_MS = 2000; // how long a freed row shows "Protect freed" before it goes

const host = $('protects-list');
const tools = $('protects-tools');
const countEl = $('protects-count');
const status = $('protects-status');
const freeAll = $('protects-clear-all');

const F = { q: '', kind: 'all', router: ALL_ROUTERS };
const closed = new Set(); // router UIDs whose group is closed.
const done = new Map(); // row key → { row, ok, text, busy }: the result of a Free in the row.
const results = {}; // router UID → { ok, text }: the last Free Sode's Protects there.
const downAt = new Map(); // router UID → when this page saw its link go down.
const lastLink = new Map();
let ask = null; // the open Confirm in a row: { key, letter, ready, lock }.

const linked = () => !!S.ws && S.wsState === 'up';

// Every router, the first one first. uid is the routerData UID (blank is the first), id the router's own UID, which
// the server names in a result.
function routers() {
  const list = S.routers.length ? S.routers : [{ uid: '', prefix: '', title: '' }];
  return list.map((r, i) => {
    const uid = i ? r.uid : '';
    const rd = routerData(uid);
    return {
      uid,
      id: r.uid,
      prefix: r.prefix,
      title: r.title,
      link: rd.link,
      device: protectsOf(uid).device,
      at: downAt.get(uid) || rd.protectAt || null,
    };
  });
}

function rows() {
  return everyHold().map(({ n, hold, uid }) => {
    const dst = dstItem(n, '', uid);
    return { key: rowKey(uid, n, hold), uid, n, num: numText(dst), label: dst.label, name: dst.name, hold, dst };
  });
}

// The rows to draw: the current rows, and a row that went but still shows its result.
function withDone(list, rts) {
  const have = new Set(list.map((r) => r.key));
  const gone = [...done.values()].filter((d) => !have.has(d.row.key)).map((d) => d.row);
  if (!gone.length) return list;
  const order = new Map(rts.map((r, i) => [r.uid, i]));
  return [...list, ...gone].sort((a, b) => order.get(a.uid) - order.get(b.uid) || a.n - b.n);
}

const sayStatus = (text, bad = false) => {
  status.replaceChildren(...(text ? [icon(bad ? 'circle-x' : 'circle-check'), el('span', null, text)] : []));
  status.dataset.state = bad ? 'error' : '';
};

function levelsCell(r) {
  const all = allLevels(r.uid);
  const full = all.length === r.hold.levels.length;
  return el('span', 'sode__prow-lv', full ? '' : levelText(r.hold.levels, r.uid));
}

// The Set by cell: plain words with the shield. Another device's shield is magenta (docs/color-roles.md, Protected).
function byCell(r) {
  const other = !r.hold.sode;
  const i = icon(holdLook(r.hold).icon);
  if (other) i.classList.add('sode__prow-shield');
  const cell = el('span', { class: 'sode__prow-by', 'data-pb-tip': holdLabel(r.hold) }, i, el('span', null, holdShort(r.hold)));
  cell.dataset.owner = r.hold.owner;
  return cell;
}

function rowNode(r, rt) {
  const lead = [el('span', 'sode__prow-n', r.num), el('span', 'sode__prow-name', r.name || '')];
  const node = el('div', { class: 'sode__prow', role: 'listitem' }, ...lead);
  node.dataset.hold = r.key; // not data-key: the Settings form reads each [data-key] as a field.
  node.dataset.dst = String(r.n);
  if (r.uid) node.dataset.router = r.uid;
  const d = done.get(r.key);
  if (ask?.key === r.key) {
    node.dataset.confirm = '';
    const keyEl = el('kbd', `sode__key${ask.ready ? '' : ' sode__key--locked'}`, ask.letter.toUpperCase());
    const ok = btn('sode-btn--primary', `Free (${ask.letter.toUpperCase()})`, () => goFree());
    ok.disabled = !ask.ready;
    ok.dataset.act = 'confirm';
    const no = btn('pb-btn--outline', 'Cancel', () => endAsk(true));
    no.dataset.act = 'cancel';
    const text = el('span', null, `Free the Protect on ${r.label}, every level? ${holdLabel(r.hold)}.`);
    node.append(el('span', 'sode__prow-confirm', text, el('span', 'sode__prompt', 'Press', keyEl, 'to confirm', ok, no)));
    return node;
  }
  if (d) {
    node.dataset.done = d.busy ? 'busy' : d.ok ? 'ok' : 'error';
    const words = el('span', 'sode__prow-result', icon(d.busy ? 'loader-circle' : d.ok ? 'circle-check' : 'circle-x'), el('span', null, d.text));
    const act = el('span', 'sode__prow-act');
    if (!d.ok && !d.busy) {
      const again = btn('pb-btn--outline pb-btn--sm', 'Try again', () => openAsk(r));
      again.dataset.act = 'again';
      act.append(again);
    }
    node.append(levelsCell(r), words, act);
    return node;
  }
  if (rt.stale) node.dataset.stale = '';
  const act = el('span', 'sode__prow-act');
  if (r.hold.sode) {
    const why = freeBlock(rt.router, linked());
    const b = btn('pb-btn--outline pb-btn--sm', 'Free', () => openAsk(r));
    b.dataset.act = 'free';
    b.setAttribute('aria-label', `Free the Protect on ${r.label}`);
    if (why) {
      b.disabled = true;
      b.dataset.pbTip = why;
    }
    act.append(b);
  }
  node.append(levelsCell(r), byCell(r), act);
  return node;
}

function groupNode(g, many) {
  const rt = g.router;
  const shut = closed.has(rt.uid);
  const name = many ? [el('span', 'sode__pgroup-prefix', rt.prefix), ' ', rt.title] : [rt.title || 'Router'];
  const toggle = btn('pb-btn--ghost pb-btn--icon pb-btn--sm', icon(shut ? 'chevron-right' : 'chevron-down'), () => {
    if (closed.has(rt.uid)) closed.delete(rt.uid);
    else closed.add(rt.uid);
    draw();
  });
  toggle.setAttribute('aria-expanded', String(!shut));
  toggle.setAttribute('aria-label', `${shut ? 'Show' : 'Hide'} the rows of ${many ? `router ${rt.prefix}` : 'the router'}`);
  toggle.dataset.act = `toggle-${rt.uid}`;
  const head = el('div', 'sode__pgroup-head', toggle, el('strong', 'sode__pgroup-name', ...name));
  if (g.stale) head.append(el('span', 'sode__pgroup-link', icon('circle-x'), el('span', null, rt.link === 'connecting' ? 'Connecting' : 'Not connected')));
  head.append(el('span', 'sode__hint', groupCount(g)));
  head.append(el('span', 'sode__pgroup-gap'));
  if (rt.device) head.append(el('span', 'sode__hint', `Device ${rt.device}`));
  if (many) {
    const b = btn('pb-btn--outline pb-btn--sm', `Free Sode's on ${rt.prefix}…`, () => openFreeSode(rt.uid));
    b.dataset.act = `free-router-${rt.uid}`;
    b.dataset.freeRouter = rt.uid;
    const why = freeBlock(rt, linked()) || (g.sode ? '' : 'Sode has no Protect on this router.');
    if (why) {
      b.disabled = true;
      b.dataset.pbTip = why;
    }
    head.append(b);
  }
  const notes = [];
  if (g.stale) notes.push(el('span', 'sode__hint', `${staleNote(rt)} Sode cannot free a Protect while the router is not connected.`));
  if (!rt.device) notes.push(el('span', 'sode__hint', "Protect is off for this router: Sode's device number is 0. Set it in the Router settings."));
  const res = results[rt.uid];
  if (res) notes.push(el('span', `sode__hint sode__pgroup-result${res.ok ? '' : ' sode__pgroup-result--error'}`, res.text));
  if (notes.length) head.append(el('div', 'sode__pgroup-notes', ...notes));
  const sec = el('section', { class: 'sode__pgroup', 'aria-label': many ? `Router ${rt.prefix} ${rt.title}` : rt.title || 'Router' }, head);
  sec.dataset.router = rt.uid;
  if (g.stale) sec.dataset.stale = '';
  if (shut) return sec;
  const body = g.rows.map((r) => rowNode(r, g));
  if (!body.length) body.push(el('div', 'sode__pempty', g.total ? 'No row of this router matches.' : 'No Protects or Router locks.'));
  sec.append(el('div', { role: 'list' }, ...body));
  return sec;
}

function filterButton(text, count, on, act, fn) {
  const b = btn(`pb-btn--sm${on ? ' pb-btn--selected' : ''}`, [text, ' ', el('span', 'sode__pcount', String(count))], fn);
  b.setAttribute('aria-pressed', String(on));
  b.dataset.act = act;
  return b;
}

function drawTools(all, rts) {
  const kc = kindCounts(all, F);
  const kinds = el(
    'span',
    { class: 'sode__pseg', role: 'group', 'aria-label': 'Show' },
    ...KINDS.map((k) =>
      filterButton(k.text, kc[k.id], F.kind === k.id, `kind-${k.id}`, () => {
        F.kind = k.id;
        draw();
      }),
    ),
  );
  const parts = [kinds];
  if (rts.length > 1) {
    const rc = routerCounts(all, rts, F);
    const pick = (uid) => () => {
      F.router = uid;
      draw();
    };
    if (rts.length <= ROUTER_BUTTONS_MAX) {
      parts.push(
        el(
          'span',
          { class: 'sode__pseg', role: 'group', 'aria-label': 'Router' },
          filterButton('All routers', rc[ALL_ROUTERS], F.router === ALL_ROUTERS, 'router-all', pick(ALL_ROUTERS)),
          ...rts.map((rt) => filterButton(rt.prefix, rc[rt.uid], F.router === rt.uid, `router-${rt.uid}`, pick(rt.uid))),
        ),
      );
    } else {
      const sel = el('select', { class: 'pb-select', id: 'protects-router', 'aria-label': 'Router' });
      sel.append(opt(ALL_ROUTERS, `All routers (${rc[ALL_ROUTERS]})`), ...rts.map((rt) => opt(rt.uid, `${rt.prefix} ${rt.title} (${rc[rt.uid]})`)));
      sel.value = F.router;
      sel.addEventListener('change', () => pick(sel.value)());
      parts.push(el('span', 'pb-select-wrap', sel));
    }
  }
  tools.replaceChildren(find, ...parts);
}

const find = el('input', {
  class: 'pb-input sode__pfind',
  type: 'search',
  id: 'protects-find',
  placeholder: 'Find a destination by name or number',
  'aria-label': 'Find a destination',
});
find.autocomplete = 'off';
find.addEventListener('input', () => {
  F.q = find.value;
  draw();
});
find.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') e.preventDefault();
  if (e.key === 'Escape' && find.value) {
    e.preventDefault();
    e.stopPropagation();
    find.value = F.q = '';
    draw();
  }
});

let shown = []; // the rows of the last draw, for the dialog and the Confirm
let routerList = [];

function draw() {
  if (!S.cfg) return;
  const rts = (routerList = routers());
  const all = rows();
  shown = all;
  if (ask && !all.some((r) => r.key === ask.key)) endAsk(false);
  const list = withDone(all, rts);
  const many = rts.length > 1;
  // Keep the place and the focus: a draw comes with every Protect change.
  const scroller = host;
  const top = scroller.scrollTop;
  const focus = document.activeElement;
  const focusAct = host.contains(focus) ? focus.dataset.act : null;
  const focusKey = focusAct ? focus.closest('[data-hold]')?.dataset.hold : null;
  const groups = groupsOf(list, rts, F, results);
  const nodes = groups.map((g) => groupNode(g, many));
  if (!nodes.length) nodes.push(el('div', 'sode__pempty', list.length ? 'No Protect or Router lock matches.' : 'No Protects or Router locks.'));
  host.replaceChildren(
    el(
      'div',
      { class: 'sode__prow sode__prow--head', 'aria-hidden': 'true' },
      el('span', null, 'Number'),
      el('span', null, 'Destination'),
      el('span', null, 'Levels'),
      el('span', null, 'Set by'),
      el('span'),
    ),
    ...nodes,
  );
  scroller.scrollTop = top;
  if (document.activeElement !== find) drawTools(all, rts);
  else updateCounts(all, rts);
  countEl.textContent = countLine(all, rts, F);
  freeAll.disabled = !all.some((r) => r.hold.sode) || !rts.some((rt) => !freeBlock(rt, linked()));
  if (ask) {
    const n = host.querySelector(`[data-hold="${CSS.escape(ask.key)}"]`);
    (n?.querySelector(ask.ready ? '[data-act="confirm"]' : '[data-act="cancel"]') || n)?.focus({ preventScroll: true });
  } else if (focusAct) {
    const sel = focusKey ? `[data-hold="${CSS.escape(focusKey)}"] [data-act="${focusAct}"]` : `[data-act="${focusAct}"]`;
    host.querySelector(sel)?.focus({ preventScroll: true });
  }
}

// While the find box has focus, its row of tools stays; only the counts and the pressed buttons change.
function updateCounts(all, rts) {
  const kc = kindCounts(all, F);
  const rc = routerCounts(all, rts, F);
  for (const b of tools.querySelectorAll('[data-act]')) {
    const c = b.querySelector('.sode__pcount');
    if (!c) continue;
    const a = b.dataset.act;
    let on = false;
    if (a.startsWith('kind-')) {
      c.textContent = String(kc[a.slice(5)]);
      on = F.kind === a.slice(5);
    } else if (a === 'router-all') {
      c.textContent = String(rc[ALL_ROUTERS]);
      on = F.router === ALL_ROUTERS;
    } else if (a.startsWith('router-')) {
      c.textContent = String(rc[a.slice(7)]);
      on = F.router === a.slice(7);
    }
    b.classList.toggle('pb-btn--selected', on);
    b.setAttribute('aria-pressed', String(on));
  }
  const sel = tools.querySelector('#protects-router');
  if (sel) sel.value = F.router;
}

// One draw for a burst of changes. A timer, not a frame: a page out of sight gets no frames, and the list must be
// right when Settings opens.
let frame = 0;
const later = () => {
  frame ||= setTimeout(() => {
    frame = 0;
    draw();
  }, 30);
};

// A router's link that leaves up: its rows are the last known ones from now.
function noteLinks() {
  const list = S.routers.length ? S.routers.map((r, i) => (i ? r.uid : '')) : [''];
  for (const uid of list) {
    const link = routerData(uid).link;
    if (lastLink.get(uid) === 'up' && link !== 'up') downAt.set(uid, Date.now());
    if (link === 'up') downAt.delete(uid);
    lastLink.set(uid, link);
  }
}
on('hello', () => {
  downAt.clear();
  lastLink.clear();
  noteLinks();
  later();
});
on('conn', () => {
  noteLinks();
  later();
});
for (const ev of ['protectedRows', 'names']) on(ev, later);

// Free in the row: the row is the Confirm, with the churning letter and the lockout.
function openAsk(r) {
  endAsk(false);
  done.delete(r.key);
  const letter = churnKey();
  ask = { key: r.key, letter, ready: false, lock: 0 };
  const a = ask;
  a.lock = setTimeout(() => {
    a.ready = true;
    draw();
  }, lockoutMs(S.cfg));
  draw();
}

function endAsk(redraw) {
  if (!ask) return;
  clearTimeout(ask.lock);
  const key = ask.key;
  ask = null;
  if (redraw) {
    draw();
    host
      .querySelector(`[data-hold="${CSS.escape(key)}"] [data-act="free"], [data-hold="${CSS.escape(key)}"] [data-act="again"]`)
      ?.focus({ preventScroll: true });
  }
}

function goFree() {
  if (!ask?.ready) return;
  const r = shown.find((x) => x.key === ask.key);
  endAsk(false);
  if (!r) return draw();
  const key = r.key;
  done.set(key, { row: r, ok: false, busy: true, text: 'Freeing the Protect…' });
  const report = (kind, head, text) => {
    if (kind === 'info') return;
    const ok = kind === 'done';
    // protects.js names the destination first ("B.23 DSM2: refused."); the row names it already.
    const why = ok ? '' : text.startsWith(`${r.label}: `) ? text.slice(r.label.length + 2) : text;
    const res = rowResult(ok, why);
    done.set(key, { row: r, ...res });
    sayStatus(rowStatus(ok, r.label, why), !ok);
    if (ok)
      setTimeout(() => {
        if (done.get(key)?.ok) done.delete(key);
        later();
      }, DONE_MS);
    later();
  };
  sendProtect(protectPlan(r.dst, false), null, null, report);
  draw();
  host.querySelector(`[data-hold="${CSS.escape(key)}"]`)?.focus?.({ preventScroll: true });
}

// Esc in the list puts the Confirm away. The letter answers anywhere in Settings, as the Confirm of Home does, but not
// in a box that takes text or under a dialog.
host.addEventListener('keydown', (e) => {
  if (!ask || e.key !== 'Escape') return;
  e.preventDefault();
  e.stopPropagation();
  endAsk(true);
});
const typing = (t) => t instanceof HTMLElement && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName)) && t.type !== 'checkbox';
document.addEventListener(
  'keydown',
  (e) => {
    if (!ask || e.ctrlKey || e.metaKey || e.altKey || e.key.toLowerCase() !== ask.letter) return;
    if (document.documentElement.dataset.view !== 'settings' || dialogShown() || typing(e.target)) return;
    e.preventDefault();
    e.stopPropagation();
    goFree();
  },
  true,
);

async function openFreeSode(only = '') {
  const { askFreeSode } = await import('./settingsprotectsfree.js');
  const back = document.activeElement;
  const picked = await askFreeSode(shown, routerList, only, linked(), back);
  if (!picked?.length) return;
  const rts = routerList;
  const ids = picked.map((uid) => rts.find((r) => r.uid === uid)?.id).filter((x) => x != null);
  sayStatus("Freeing Sode's Protects…");
  clearSodeProtects(
    (m) => {
      const prefixOf = (id) => rts.find((r) => r.id === id)?.prefix || '';
      const res = freeResult(m, prefixOf);
      for (const [id, v] of Object.entries(res.per)) {
        const rt = rts.find((r) => r.id === id);
        if (rt) results[rt.uid] = v;
      }
      sayStatus(res.text, !res.ok);
      later();
    },
    S.routers.length ? ids : null,
  );
}

freeAll.addEventListener('click', () => openFreeSode(''));
