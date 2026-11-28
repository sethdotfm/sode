// The panels page (/panels): a list on the left, the chosen panel on the right. Calls go over /api/v1
// with the user's session key. The server decides what a user can see and change (canEdit).
import { $, el, icon, on } from './core.js';
import { sizeText } from './gridmodel.js';
import { sessionFetch, jsonCall, startSession, signInAgain, onSessionKey } from './sessionkey.js';
import { paintOnline, paintOpenNow, openNow, watchBox, MON_POLL_MS } from './panelsmon.js';
import {
  groupPanels,
  actionsFor,
  deleteStep,
  thumbCells,
  gridThumb,
  gridSize,
  actsText,
  actsShort,
  shareLabel,
  launchHref,
  editHref,
  listKey,
  stepPanel,
  emptyText,
} from './panelsrules.js';
import { resultStep, mac, keyLabel } from './keys.js';
import { openShare } from './panelshare.js';
import { openPush } from './panelpush.js';
import { artLeftOut, SIGN_IN_ENDED } from './panelrules.js';
import { openAssign, askName } from './panelassign.js';
import { clashOf, renameRefusal } from './panelassignrules.js';
import { isGridPanel } from './gridpaneledges.js';
import { sendCopy } from './sendcopy.js';
import { layout, onLayout } from './layout.js';
import { desktopOnly } from './desktoponly.js';
import { statusBadge, panelStatus } from './brokenbadge.js';

const DISARM_MS = 5000;
const S = { panels: [], cur: '', armed: '', filter: '', me: '', trusted: false, loaded: false };
let disarm = 0;
// The list belongs to the user who loaded it: a sign-out or another user clears it, and the next key loads again.
let keyed = false;
let shownFor = null;

const say = (text) => {
  const n = $('panels-note');
  n.textContent = text;
  n.hidden = !text;
};

const api = jsonCall(sessionFetch, { failed: (s) => `Sode answered ${s}` });

function thumb(p, small) {
  const box = el('div', `sode-ov__thumb${small ? ' sode-ov__thumb--sm' : ''}`);
  box.setAttribute('aria-hidden', 'true');
  const grid = isGridPanel(p);
  const g = grid ? gridThumb(p, small ? 'medium' : 'large') : null;
  const { rows, cols } = g || p;
  box.style.gridTemplate = `repeat(${rows}, minmax(0, 1fr)) / repeat(${cols}, minmax(0, 1fr))`;
  box.style.aspectRatio = `${cols} / ${rows}`;
  // A list picture is never higher than --sode-ov-thumb-h, so every row has one height.
  if (small && !grid) box.style.width = `min(6rem, var(--sode-ov-thumb-h) * ${cols} / ${rows})`;
  if (grid) {
    box.classList.add('sode-ov__thumb--grid');
    box.dataset.size = g.size;
  }
  for (let r = 1; r <= rows; r++) {
    for (let c = 1; c <= cols; c++) {
      if (grid && r === 1 && c === 1) continue;
      const i = el('i');
      i.style.gridArea = `${r} / ${c}`;
      box.append(i);
    }
  }
  for (const c of g ? g.cells : thumbCells(p)) {
    const i = el('i', `sode-ov__k-${c.kind}`);
    i.style.gridArea = `${c.r} / ${c.c} / span ${c.rs} / span ${c.cs}`;
    box.append(i);
  }
  // A Grid panel's counts are tiles in its two bars. A Buttons panel's size is the caption under the large picture only.
  for (const t of g ? g.tiles : []) {
    const i = el('i', `sode-ov__k-${t.kind} sode-ov__tile sode-ov__tile--${t.kind}${t.half ? ' sode-ov__tile--half' : ''}`, t.label);
    i.style.gridArea = `${t.r} / ${t.c} / span ${t.rs} / span ${t.cs}`;
    box.append(i);
  }
  return box;
}

const shownIds = () => groupPanels(S.panels, S.me, S.trusted, S.filter).flatMap((g) => g.panels.map((p) => p.id));

const mine = (p) => p.owner.toLowerCase() === S.me.toLowerCase();
const owner = (p) => (mine(p) ? 'You' : p.owner);

function row(p) {
  const b = el('button', 'sode-ov__item');
  b.type = 'button';
  b.dataset.id = p.id;
  b.setAttribute('aria-current', String(p.id === S.cur));
  const online = el('span', 'sode-ov__online');
  online.dataset.online = p.id;
  const sub = `${owner(p)} · ${actsShort(p)}`;
  const who = el(
    'span',
    'sode-ov__who',
    el('strong', { class: 'sode-ov__line', title: p.name }, p.name),
    el('span', { class: 'sode-ov__muted sode-ov__line', title: sub }, sub),
  );
  const badge = statusBadge(panelStatus(p));
  if (badge) who.append(badge);
  b.append(thumb(p, true), who, online);
  return b;
}

function list(groups) {
  const nav = $('panels-list');
  nav.replaceChildren();
  for (const g of groups) {
    nav.append(el('h2', 'sode-ov__group', g.title));
    for (const p of g.panels) nav.append(row(p));
  }
  if (groups.length) return;
  const empty = el('p', 'sode-ov__muted sode-ov__empty', emptyText(S.loaded, S.panels.length > 0, S.filter));
  if (S.loaded && S.panels.length) {
    const clear = el('button', 'pb-btn pb-btn--outline', 'Clear');
    clear.type = 'button';
    clear.dataset.clearFind = '';
    clear.addEventListener('click', () => {
      const find = $('panels-find');
      find.value = '';
      S.filter = '';
      draw();
      find.focus();
    });
    empty.append(clear);
  }
  nav.append(empty);
}

function button(label, ico, cls, act, id) {
  const b = el('button', `pb-btn ${cls}`, icon(ico), ` ${label}`);
  b.type = 'button';
  b.dataset.act = act;
  b.dataset.id = id;
  return b;
}

function link(label, ico, cls, href, key) {
  const a = el('a', `pb-btn ${cls}`, icon(ico), ` ${label}`);
  a.href = href;
  if (key) a.append(el('kbd', 'sode-ov__kbd', key));
  return a;
}

function detail(p) {
  const box = $('panels-detail');
  box.replaceChildren();
  if (!p) return;
  // The phone keeps Launch and Open now (docs/panels-design.md, Phone and touch).
  const phone = layout() === 'phone';
  const acts = phone ? [] : actionsFor(p);
  const main = el('div', 'sode-ov__acts-main');
  main.append(link('Launch', 'play', 'sode-btn--primary', launchHref(p.id), keyLabel('mod+enter')));
  if (acts.includes('edit')) main.append(link('Edit', 'pencil', '', editHref(p.id), keyLabel('mod+e')));
  if (acts.includes('copy')) main.append(button('Save a copy', 'copy', '', 'copy', p.id));
  if (acts.includes('delete')) {
    const armed = S.armed === p.id;
    main.append(
      button(armed ? 'Press again to delete' : 'Delete', 'trash-2', `${armed ? 'pb-btn--outline' : 'pb-btn--ghost'} sode-ov__delete`, 'delete', p.id),
    );
  }
  const more = el('div', 'sode-ov__acts-more');
  if (acts.includes('push')) more.append(button('Push to panels', 'send', '', 'push', p.id));
  if (!phone && S.me) more.append(button('Send a copy', 'forward', '', 'send', p.id));
  if (!phone && (S.trusted || p.canEdit)) more.append(button('Assign to', 'monitor-smartphone', '', 'assign', p.id));
  if (acts.includes('share')) more.append(button('Share', 'share-2', '', 'share', p.id));
  const bar = el('div', 'sode-ov__acts', main, more.childElementCount ? more : null);
  const kv = el(
    'dl',
    'sode-ov__kv',
    el('dt', '', 'Owner'),
    el('dd', '', owner(p) + (p.canEdit ? '' : ' (you cannot edit this panel)')),
    el('dt', '', 'Acts as'),
    el('dd', '', actsText(p)),
    el('dt', '', 'Shared with'),
    el('dd', '', shareLabel(p.share, mine(p))),
  );
  box.append(
    el('h1', 'sode-ov__title', p.name, statusBadge(panelStatus(p)) || ''),
    el(
      'div',
      'sode-ov__big',
      el('div', '', thumb(p), el('p', 'pb-hint', isGridPanel(p) ? gridSize(p) : sizeText(p.rows, p.cols))),
      el(
        'div',
        'sode-ov__pane',
        bar,
        phone ? desktopOnly('edit, share, assign or delete panels and manage their keys') : null,
        kv,
        watchBox(p, setWatch),
        openNow(p, openHandlers()),
      ),
    ),
  );
}

function draw() {
  $('panels-new').hidden = layout() === 'phone';
  const groups = groupPanels(S.panels, S.me, S.trusted, S.filter);
  const shown = groups.flatMap((g) => g.panels);
  if (!shown.some((p) => p.id === S.cur)) S.cur = shown[0]?.id || '';
  list(groups);
  const act = document.activeElement?.closest('#panels-detail [data-act]')?.dataset.act;
  detail(shown.find((p) => p.id === S.cur));
  if (act) $('panels-detail').querySelector(`[data-act="${act}"]`)?.focus();
  paintOnline(S.panels);
}

async function setWatch(id, on, instance = '') {
  try {
    await api(`api/v1/panels/${id}/watch${instance ? `?instance=${encodeURIComponent(instance)}` : ''}`, { method: on ? 'POST' : 'DELETE' });
    const p = S.panels.find((x) => x.id === id);
    if (!p) return;
    if (!instance) p.watching = on;
    else {
      const name = instance.toLowerCase();
      p.watchingInstances = (p.watchingInstances || []).filter((d) => d !== name).concat(on ? [name] : []);
    }
  } catch (e) {
    say(e.message);
    draw();
  }
}

// A hide only changes this user's view; the next read brings the counts that leave it out.
async function setHide(id, instance, on) {
  try {
    await api(`api/v1/panels/${id}/instances/hide?instance=${encodeURIComponent(instance)}`, { method: on ? 'POST' : 'DELETE' });
    const p = S.panels.find((x) => x.id === id);
    if (p) {
      const name = instance.toLowerCase();
      p.hiddenInstances = (p.hiddenInstances || []).filter((d) => d !== name).concat(on ? [name] : []);
    }
    draw();
    refreshOnline();
  } catch (e) {
    say(e.message);
  }
}

// Renames the instance's device name: the page hears it down its stream (POST .../instances/rename). Answers null after
// it is sent, or after a clash is handed to the name dialog, else renameRefusal's {why}. keyless names a keyless
// page by its id; it has no watch and no name check.
async function rename(id, instance, name, keyless = '') {
  const p = S.panels.find((x) => x.id === id);
  try {
    await api(`api/v1/panels/${id}/instances/rename`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(keyless ? { keyless, name } : { instance, name }),
    });
    // The server moved the watch with the name; the poll does not carry watches.
    if (p) p.watchingInstances = (p.watchingInstances || []).map((d) => (d === instance.toLowerCase() ? name.toLowerCase() : d));
    refreshOnline();
    return null;
  } catch (e) {
    const suggest = clashOf(e);
    if (suggest === null) return renameRefusal(e);
    askName(p || { name: 'This panel' }, name, suggest, (chosen) => rename(id, instance, chosen).then((r) => r && say(r.why)), 'rename');
    return null;
  }
}

// Reload from the Open now list: target is {instance}, {keyless} or {all: true}. The page reloads on its own.
async function reload(id, target) {
  try {
    await api(`api/v1/panels/${id}/instances/reload`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(target) });
    refreshOnline();
  } catch (e) {
    say(e.message);
  }
}

const openHandlers = () => ({ setWatch, setHide, rename, reload, trusted: S.trusted });

// Online marks and the Open now list change without a redraw, so a button that a person presses is not rebuilt.
async function refreshOnline() {
  if (!S.loaded || !keyed || document.hidden) return;
  try {
    const next = (await api('api/v1/panels')).panels;
    for (const p of next) {
      const old = S.panels.find((x) => x.id === p.id);
      if (old) {
        const { online, instances, keyless, instancesOnline, instancesKnown, hiddenInstances } = p;
        Object.assign(old, { online, instances, keyless, instancesOnline, instancesKnown, hiddenInstances });
      }
    }
    paintOnline(S.panels);
    paintOpenNow(
      S.panels.find((x) => x.id === S.cur),
      openHandlers(),
    );
  } catch {}
}

async function load(pick) {
  shownFor = S.me;
  try {
    S.panels = (await api('api/v1/panels')).panels;
    S.loaded = true;
    if (pick) S.cur = pick;
    say('');
  } catch (e) {
    say(`Panels could not be loaded. ${e.message}`);
  }
  draw();
}

async function act(kind, id) {
  try {
    if (kind === 'delete') {
      const step = deleteStep(S.armed, id);
      S.armed = step.armed;
      clearTimeout(disarm);
      if (step.armed) disarm = setTimeout(() => ((S.armed = ''), draw()), DISARM_MS);
      if (!step.go) return draw();
      await api(`api/v1/panels/${id}`, { method: 'DELETE' });
      S.cur = '';
      return load();
    }
    if (kind === 'share') {
      const p = S.panels.find((x) => x.id === id);
      // The redraw replaces the Share button, so focus goes to the new one.
      if (p) openShare(p, { api, onSaved: (saved) => load(saved.id).then(() => document.querySelector('#panels-detail [data-act="share"]')?.focus()) });
      return;
    }
    if (kind === 'push') {
      const p = S.panels.find((x) => x.id === id);
      if (p) openPush(p, { api, panels: S.panels, me: S.me, trusted: S.trusted, onDone: () => load(id) });
      return;
    }
    if (kind === 'send') {
      const p = S.panels.find((x) => x.id === id);
      if (p) sendCopy({ panels: [p] }, p.name);
      return;
    }
    if (kind === 'assign') {
      const p = S.panels.find((x) => x.id === id);
      if (p) openAssign(p, { api, panels: () => S.panels, onDone: refreshOnline });
      return;
    }
    if (kind === 'copy') {
      const made = await api(`api/v1/panels/${id}/copy`, { method: 'POST' });
      await load(made.id);
      return say(artLeftOut(made.artSkipped));
    }
  } catch (e) {
    S.armed = '';
    say(e.message);
    draw();
  }
}

export function runPanelsList() {
  $('panels-new').hidden = layout() === 'phone';
  $('panels-signin').addEventListener('click', signInAgain);
  $('panels-find').addEventListener('input', (e) => {
    S.filter = e.target.value;
    draw();
  });
  $('panels-find').addEventListener('keydown', (e) => {
    const ids = shownIds();
    const step = resultStep(e, ids.length > 0);
    if (step) {
      e.preventDefault();
      S.cur = stepPanel(ids, S.cur, step);
      S.armed = '';
      draw();
      document.querySelector('.sode-ov__item[aria-current="true"]')?.scrollIntoView({ block: 'nearest' });
    } else if (e.key === 'Enter' && ids.length) {
      // The highlighted panel is already the chosen one; Enter hands the keyboard to Launch, so the next Enter opens it.
      e.preventDefault();
      document.querySelector('#panels-detail a[href]')?.focus();
    }
  });
  document.addEventListener('keydown', (e) => {
    // The Lock's dialog stays in the page while hidden, so only a rendered one counts.
    if ([...document.querySelectorAll('[role="dialog"]')].some((d) => d.getClientRects().length)) return;
    const what = listKey(e, mac);
    if (!what) return;
    if (what === 'next' || what === 'prev') {
      const ids = shownIds();
      e.preventDefault();
      S.cur = stepPanel(ids, S.cur, what === 'next' ? 1 : -1);
      S.armed = '';
      draw();
      document.querySelector('.sode-ov__item[aria-current="true"]')?.scrollIntoView({ block: 'nearest' });
      return;
    }
    const p = S.panels.find((x) => x.id === S.cur);
    if (!p || (what === 'edit' && (layout() === 'phone' || !actionsFor(p).includes('edit')))) return;
    e.preventDefault();
    location.href = what === 'edit' ? editHref(p.id) : launchHref(p.id);
  });
  $('panels-new').addEventListener('click', () => {
    location.href = 'panels/editor';
  });
  document.querySelector('.sode-ov').addEventListener('click', (e) => {
    const pick = e.target.closest('[data-id].sode-ov__item');
    if (pick) {
      S.cur = pick.dataset.id;
      S.armed = '';
      return draw();
    }
    const b = e.target.closest('[data-act]');
    if (b) act(b.dataset.act, b.dataset.id);
  });
  on('access', (a) => {
    S.me = a.user || '';
    S.trusted = !!a.trusted && !!a.user;
    if (shownFor === null || shownFor === S.me) return;
    shownFor = null;
    Object.assign(S, { panels: [], cur: '', armed: '', loaded: false });
    draw();
    if (keyed && S.me) load();
  });
  onSessionKey((k, step) => {
    keyed = !!k;
    $('panels-signin').hidden = step !== 'sign-in';
    if (k) {
      if (shownFor === null) load();
    } else if (step === 'ended') say(SIGN_IN_ENDED);
    else if (step === 'sign-in') say('Sign in to see your panels.');
    else if (step === 'no-users') say('Panels need a user, and Sode has none.');
  });
  onLayout(draw);
  setInterval(refreshOnline, MON_POLL_MS);
  startSession();
}
