// /panels monitoring: row status, Watch and the Open now list.
import { el, icon } from './core.js';
import { statusText } from './statustext.js';
import { askNotifyPermission, notifyPermission } from './panelwatch.js';
import { durationText, permissionText } from './panelwatchrules.js';
import { panelStatus, shownInstances, hiddenShown } from './panelsrules.js';
import { slotFor, summary } from './uptime.js';
import { uptimeBars } from './uptimebars.js';
import { streamState, renameProblem } from './panelassignrules.js';
import { INSTANCE_MAX } from './panelkeyrules.js';
import { reloadButton, reloadAllButton, offlineNote } from './panelsopen.js';

// No push channel to /panels, so this is how fast the online mark follows.
export const MON_POLL_MS = 2000;

const badge = (online) => el('span', `pb-badge${online ? ' pb-badge--ok' : ''}`, online ? 'Online' : 'Offline');

// A list row's status is plain words, not a badge, so it does not read as a button. It has no colored square.
const rowBadge = (p) => {
  const s = panelStatus(p);
  const node = statusText(s.tone || 'idle', s.text);
  node.classList.add('sode-ov__status');
  return node;
};

const INSTANCE_WORDS = { up: 'online', down: 'offline' };

const fmtTime = (t) => {
  const d = new Date(t);
  const p = (n) => String(n).padStart(2, '0');
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
};

export function paintOnline(panels) {
  for (const span of document.querySelectorAll('[data-online]')) {
    const p = panels.find((x) => x.id === span.dataset.online);
    span.replaceChildren(...(p ? [rowBadge(p)] : []));
  }
}

// The last 24 hours of one instance, from the server's own clock (to), so the browser's clock does not shift it.
function uptimeStrip(i) {
  if (!i.uptime) return null;
  const u = i.uptime;
  const data = { start: u.start, trans: u.trans, counts: [], from: u.from, to: u.to, slot: slotFor(u.from, u.to), now: u.to };
  const text = summary(data);
  const bars = el('div', 'sode-up__bars sode-up__bars--sm', ...uptimeBars(data, { fmt: fmtTime, words: INSTANCE_WORDS }));
  bars.setAttribute('role', 'img');
  bars.setAttribute('aria-label', `${i.device}, last 24 hours: ${text}`);
  return el('div', 'sode-ov__uptime', bars, el('span', 'sode-ov__muted', `Last 24 hours: ${text}`));
}

function hideButton(p, i, setHide) {
  const b = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm', icon('eye-off'));
  b.type = 'button';
  b.dataset.hideInstance = i.device;
  b.dataset.pbTip = 'Hide. Keeps key and watch.';
  b.classList.add('pb-tip--end');
  b.setAttribute('aria-label', `Hide ${i.device}`);
  b.addEventListener('click', () => setHide?.(p.id, i.device, true));
  return b;
}

// rename(id, device, name, keyless) answers null when sent or handed to the clash dialog, else {why}.
function nameSlot(p, i, rename) {
  const slot = el('span', 'sode-ov__name');
  const state = rename ? streamState(i) : '';
  const show = (focus) => {
    if (!state) return slot.replaceChildren(el('strong', '', i.device));
    const b = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm', icon('text-cursor-input'));
    b.type = 'button';
    b.dataset.renameInstance = i.device;
    b.disabled = state === 'offline';
    b.setAttribute('aria-label', state === 'offline' ? `Rename ${i.device}: offline` : `Rename ${i.device}`);
    b.dataset.pbTip = state === 'offline' ? 'Offline: no open page' : 'Rename';
    b.addEventListener('click', edit);
    slot.replaceChildren(el('strong', '', i.device), b);
    if (focus) b.focus();
  };
  function edit() {
    const input = el('input', 'pb-input sode-panels__device-input');
    input.type = 'text';
    input.maxLength = INSTANCE_MAX;
    input.value = i.device;
    input.autocomplete = 'off';
    input.dataset.renameField = i.device;
    input.setAttribute('aria-label', `New name for ${i.device}`);
    const err = el('span', 'pb-note pb-note--error');
    err.setAttribute('role', 'alert');
    err.hidden = true;
    let busy = false;
    const fail = (t) => {
      err.textContent = t;
      err.hidden = !t;
    };
    const keydown = async (e) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        return show(true);
      }
      if (e.key !== 'Enter' || busy) return;
      e.preventDefault();
      const name = input.value.trim();
      if (name === i.device) return show(true);
      const problem = renameProblem(name);
      if (problem) return fail(problem);
      busy = true;
      const r = await rename(p.id, i.device, name, i.keyless || '');
      busy = false;
      if (!r) return show(true);
      fail(r.why);
    };
    input.addEventListener('keydown', keydown);
    input.addEventListener('blur', () => {
      if (!busy && slot.contains(input)) show();
    });
    slot.replaceChildren(input, err);
    input.focus();
    input.select();
  }
  show();
  return slot;
}

function hiddenLine(p, setHide) {
  const names = hiddenShown(p);
  if (!names.length) return null;
  const show = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Show all');
  show.type = 'button';
  show.dataset.showHidden = '';
  show.addEventListener('click', () => names.forEach((d) => setHide?.(p.id, d, false)));
  const shown = names.map((d) => (p.instances || []).find((i) => i.device.toLowerCase() === d)?.device || d);
  return el('p', 'sode-ov__muted sode-ov__hidden', `Hidden: ${shown.join(', ')}`, show);
}

const offlineAfter = (i, shown) => (shown && streamState(i) === 'offline' ? offlineNote() : null);

// A Keyless instance's name lives in its own browser.
function keylessItem(p, k, rename, reload) {
  const re = reloadButton(p, k, reload);
  return el(
    'li',
    '',
    el(
      'div',
      'sode-ov__inst',
      el('span', 'pb-badge', 'No key'),
      nameSlot(p, k, rename),
      k.device === k.addr ? null : el('span', 'sode-ov__muted', k.addr),
      re,
      offlineAfter(k, rename || re),
    ),
  );
}

export function openNow(p, { setWatch, setHide, rename, reload, trusted } = {}, now = Date.now()) {
  const box = el('section', 'sode-ov__opennow');
  box.id = 'panels-open';
  box.dataset.for = p.id;
  if (!p.instances) return box;
  const trustedReload = trusted ? reload : null;
  box.append(el('div', 'sode-ov__openhead', el('h2', 'sode-ov__group', 'Open now'), reloadAllButton(p, trustedReload)));
  const rows = shownInstances(p);
  const hidden = hiddenLine(p, setHide);
  const keyless = (p.keyless || []).map((k) => keylessItem(p, k, trusted ? rename : null, trustedReload));
  if (!rows.length) {
    box.append(el('p', 'sode-ov__muted', hidden ? 'All hidden' : 'No instances since Sode started'));
    if (keyless.length) box.append(el('ul', 'sode-ov__open', ...keyless));
    if (hidden) box.append(hidden);
    return box;
  }
  const watched = new Set(p.watchingInstances || []);
  const ul = el('ul', 'sode-ov__open');
  for (const i of rows) {
    const ago = i.seen ? (i.online ? '' : `last seen ${durationText((now - Date.parse(i.seen)) / 1000)} ago`) : 'not seen since Sode started';
    const input = el('input', 'pb-check__input');
    input.type = 'checkbox';
    input.checked = watched.has(i.device.toLowerCase());
    input.dataset.watchInstance = i.device;
    input.addEventListener('change', () => setWatch?.(p.id, input.checked, i.device));
    const re = reloadButton(p, i, trustedReload);
    ul.append(
      el(
        'li',
        '',
        el(
          'div',
          'sode-ov__inst',
          badge(i.online),
          nameSlot(p, i, rename),
          hideButton(p, i, setHide),
          ago && el('span', 'sode-ov__muted', ago),
          re,
          offlineAfter(i, rename || re),
          el('label', 'pb-check sode-ov__watchone', input, el('span', 'pb-check__label', 'Notify when offline')),
        ),
        uptimeStrip(i),
      ),
    );
  }
  ul.append(...keyless);
  box.append(ul);

  if (hidden) box.append(hidden);
  return box;
}

export function watchBox(p, setWatch) {
  if (!p.canEdit) return el('span');
  const input = el('input', 'pb-check__input');
  input.type = 'checkbox';
  input.id = 'panels-watch';
  input.checked = !!p.watching;
  // Browsers ask for notification permission only on a click.
  const note = el('p', 'pb-hint sode-ov__notify');
  note.dataset.notify = '';
  const allow = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Allow notifications');
  allow.type = 'button';
  const paint = () => {
    const t = permissionText(notifyPermission());
    note.textContent = t.text;
    note.dataset.notify = t.state;
    allow.hidden = t.state !== 'ask';
  };
  allow.addEventListener('click', () => askNotifyPermission().then(paint));
  input.addEventListener('change', () => {
    if (input.checked) askNotifyPermission().then(paint);
    setWatch(p.id, input.checked);
  });
  paint();
  return el('div', 'sode-ov__watchbox', el('label', 'pb-check sode-ov__watch', input, el('span', 'pb-check__label', 'Notify when offline')), note, allow);
}

// A poll skips the list while a field has focus; a focused button gets its copy focused.
export function paintOpenNow(p, handlers) {
  const old = document.getElementById('panels-open');
  const f = document.activeElement;
  const held = old?.contains(f);
  if (!old || !p || old.dataset.for !== p.id || (held && f.matches('input'))) return;
  const fresh = openNow(p, handlers);
  old.replaceWith(fresh);
  if (!held) return;
  const same = [...f.attributes].filter((a) => a.name.startsWith('data-')).map((a) => `[${a.name}="${CSS.escape(a.value)}"]`);
  if (same.length) fresh.querySelector(same.join(''))?.focus();
}
