// The panels page (/panels): a list on the left, the chosen panel on the right. Calls go over /api/v1
// with the user's session key. The server decides what a user may see and change (canEdit).
import { $, el, icon, on } from './core.js';
import { sessionFetch, startSession, signInAgain, onSessionKey } from './sessionkey.js';
import { paintOnline, paintOpenNow, openNow, watchBox, MON_POLL_MS } from './panelsmon.js';
import { groupPanels, actionsFor, deleteStep, thumbCells, actsLabel, shareLabel, launchHref, editHref } from './panelsrules.js';
import { openShare } from './panelshare.js';
import './panelkeytab.js';
import { openPush } from './panelpush.js';

const DISARM_MS = 5000;
const S = { panels: [], cur: '', armed: '', filter: '', me: '', trusted: false, loaded: false };
let disarm = 0;

const say = (text) => {
  const n = $('panels-note');
  n.textContent = text;
  n.hidden = !text;
};

async function api(path, init) {
  const res = await sessionFetch(path, init);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error?.message || `Sode answered ${res.status}`);
  }
  return res.status === 204 ? null : res.json();
}

function thumb(p, small) {
  const box = el('div', `sode-ov__thumb${small ? ' sode-ov__thumb--sm' : ''}`);
  box.setAttribute('aria-hidden', 'true');
  box.style.gridTemplate = `repeat(${p.rows}, 1fr) / repeat(${p.cols}, 1fr)`;
  box.style.aspectRatio = `${p.cols} / ${p.rows}`;
  for (let r = 1; r <= p.rows; r++) {
    for (let c = 1; c <= p.cols; c++) {
      const i = el('i');
      i.style.gridArea = `${r} / ${c}`;
      box.append(i);
    }
  }
  for (const c of thumbCells(p)) {
    const i = el('i', `sode-ov__k-${c.kind}`);
    i.style.gridArea = `${c.r} / ${c.c} / span ${c.rs} / span ${c.cs}`;
    box.append(i);
  }
  return box;
}

const owner = (p) => (p.owner.toLowerCase() === S.me.toLowerCase() ? 'You' : p.owner);

function row(p) {
  const b = el('button', 'sode-ov__item');
  b.type = 'button';
  b.dataset.id = p.id;
  b.setAttribute('aria-current', String(p.id === S.cur));
  // Empty hook for the online dot, so monitoring can fill it without reshaping the row.
  const online = el('span', 'sode-ov__online');
  online.dataset.online = p.id;
  b.append(
    thumb(p, true),
    el('span', 'sode-ov__who', el('strong', '', p.name), el('span', 'sode-ov__muted', `${owner(p)}, ${actsLabel(p).toLowerCase()}`)),
    online,
  );
  return b;
}

function list(groups) {
  const nav = $('panels-list');
  nav.replaceChildren();
  for (const g of groups) {
    nav.append(el('h2', 'sode-ov__group', g.title));
    for (const p of g.panels) nav.append(row(p));
  }
  if (!groups.length) nav.append(el('p', 'sode-ov__muted sode-ov__empty', S.loaded ? 'No panel matches.' : ''));
}

function button(label, ico, cls, act, id) {
  const b = el('button', `pb-btn ${cls}`, icon(ico), ` ${label}`);
  b.type = 'button';
  b.dataset.act = act;
  b.dataset.id = id;
  return b;
}

function link(label, ico, cls, href) {
  const a = el('a', `pb-btn ${cls}`, icon(ico), ` ${label}`);
  a.href = href;
  return a;
}

function detail(p) {
  const box = $('panels-detail');
  box.replaceChildren();
  if (!p) return;
  const acts = actionsFor(p);
  const bar = el('div', 'sode-ov__acts');
  if (acts.includes('edit')) bar.append(link('Edit', 'pencil', '', editHref(p.id)));
  if (acts.includes('copy')) bar.append(button('Save a copy', 'copy', '', 'copy', p.id));
  if (acts.includes('share')) bar.append(button('Share', 'share-2', '', 'share', p.id));
  if (acts.includes('push')) bar.append(button('Push to panels', 'send', '', 'push', p.id));
  // Link with key goes here.
  const slot = el('span', 'sode-ov__more');
  slot.id = 'panels-more';
  bar.append(slot);
  if (acts.includes('delete')) bar.append(button(S.armed === p.id ? 'Delete? Press again' : 'Delete', 'trash-2', 'pb-btn--ghost', 'delete', p.id));
  const kv = el(
    'dl',
    'sode-ov__kv',
    el('dt', '', 'Owner'),
    el('dd', '', owner(p) + (p.canEdit ? '' : ' (you can use it, not change it)')),
    el('dt', '', 'Acts as'),
    el('dd', '', actsLabel(p).replace('Acts as ', '') + (p.acts === 'api' ? ' (opens with its key)' : ' (asks for a PIN at launch)')),
    el('dt', '', 'Shared with'),
    el('dd', '', shareLabel(p.share)),
  );
  box.append(
    el('h1', 'sode-ov__title', p.name),
    el(
      'div',
      'sode-ov__big',
      el('div', '', thumb(p), el('p', 'pb-hint', `${p.rows} by ${p.cols} grid`)),
      el('div', 'sode-ov__pane', link('Launch', 'play', 'pb-btn--ok sode-ov__launch', launchHref(p.id)), bar, kv, watchBox(p, setWatch), openNow(p)),
    ),
  );
}

function draw() {
  const groups = groupPanels(S.panels, S.me, S.trusted, S.filter);
  const shown = groups.flatMap((g) => g.panels);
  if (!shown.some((p) => p.id === S.cur)) S.cur = shown[0]?.id || '';
  list(groups);
  detail(shown.find((p) => p.id === S.cur));
  paintOnline(S.panels);
}

async function setWatch(id, on) {
  try {
    await api(`api/v1/panels/${id}/watch`, { method: on ? 'POST' : 'DELETE' });
    const p = S.panels.find((x) => x.id === id);
    if (p) p.watching = on;
  } catch (e) {
    say(e.message);
    draw();
  }
}

// Online marks and the Open now list change without a redraw, so a button being pressed is not rebuilt.
async function refreshOnline() {
  if (!S.loaded || document.hidden) return;
  try {
    const next = (await api('api/v1/panels')).panels;
    for (const p of next) {
      const old = S.panels.find((x) => x.id === p.id);
      if (old) Object.assign(old, { online: p.online, instances: p.instances });
    }
    paintOnline(S.panels);
    paintOpenNow(S.panels.find((x) => x.id === S.cur));
  } catch {}
}

async function load(pick) {
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
      if (p) openShare(p, { api, onSaved: (saved) => load(saved.id) });
      return;
    }
    if (kind === 'push') {
      const p = S.panels.find((x) => x.id === id);
      if (p) openPush(p, { api, panels: S.panels, me: S.me, trusted: S.trusted, onDone: () => load(id) });
      return;
    }
    if (kind === 'copy') {
      const made = await api(`api/v1/panels/${id}/copy`, { method: 'POST' });
      return load(made.id);
    }
  } catch (e) {
    S.armed = '';
    say(e.message);
    draw();
  }
}

export function runPanelsList() {
  $('panels-signin').addEventListener('click', signInAgain);
  $('panels-find').addEventListener('input', (e) => {
    S.filter = e.target.value;
    draw();
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
  });
  let first = true;
  onSessionKey((k, step) => {
    $('panels-signin').hidden = step !== 'sign-in';
    if (k) {
      if (first) {
        first = false;
        load();
      }
    } else if (step === 'sign-in') say('Sign in with your PIN to see your panels.');
    else if (step === 'no-users') say('Panels need a user, and Sode has none.');
  });
  setInterval(refreshOnline, MON_POLL_MS);
  startSession();
}
