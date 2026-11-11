// Monitoring on the panels page: the status of each row, the Watch checkbox and the Open now list of the
// chosen panel, each instance with its uptime strip and Hide. Kept out of panelslist.js so that file only calls a few functions.
import { el, icon } from './core.js';
import { askNotifyPermission, notifyPermission } from './panelwatch.js';
import { durationText, permissionText } from './panelwatchrules.js';
import { panelStatus, shownInstances, hiddenShown } from './panelsrules.js';
import { slotFor, summary } from './uptime.js';
import { uptimeBars } from './uptimebars.js';
import { renameState, renameProblem } from './panelassignrules.js';

// Fast enough that the online mark follows an instance within a couple of seconds, with no push channel to the page.
export const MON_POLL_MS = 2000;

const badge = (online) => el('span', `pb-badge${online ? ' pb-badge--ok' : ''}`, online ? 'Online' : 'Offline');

const statusBadge = (p) => {
  const s = panelStatus(p);
  return el('span', `pb-badge${s.tone ? ` pb-badge--${s.tone}` : ''}`, s.text);
};

const INSTANCE_WORDS = { up: 'online', down: 'offline' };

const fmtTime = (t) => {
  const d = new Date(t);
  const p = (n) => String(n).padStart(2, '0');
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
};

// Fills every row's empty online span (the hook row() leaves) from the panels last read.
export function paintOnline(panels) {
  for (const span of document.querySelectorAll('[data-online]')) {
    const p = panels.find((x) => x.id === span.dataset.online);
    span.replaceChildren(...(p ? [statusBadge(p)] : []));
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
  b.dataset.pbTip = 'Hide this instance from the list. It keeps its key and watch, and shows again when it comes back.';
  b.classList.add('pb-tip--end');
  b.setAttribute('aria-label', `Hide ${i.device}`);
  b.addEventListener('click', () => setHide?.(p.id, i.device, true));
  return b;
}

// The instance's name with its rename pencil. A click turns the name into a field: Enter saves, Escape or leaving drops
// the edit. rename(id, device, name, pin) answers null when sent (or handed to the clash dialog), else {why, pin}; pin
// shows the Advanced Settings PIN field, as the Assign to modal does.
function nameSlot(p, i, rename) {
  const slot = el('span', 'sode-ov__name');
  const state = rename ? renameState(i) : '';
  const show = () => {
    if (!state) return slot.replaceChildren(el('strong', '', i.device));
    const b = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm', icon('pencil'));
    b.type = 'button';
    b.dataset.renameInstance = i.device;
    b.disabled = state === 'offline';
    b.setAttribute('aria-label', state === 'offline' ? `Rename ${i.device}: offline` : `Rename ${i.device}`);
    b.dataset.pbTip = state === 'offline' ? 'Offline: it cannot hear a new name until it opens again' : 'Rename this instance';
    b.addEventListener('click', edit);
    slot.replaceChildren(el('strong', '', i.device), b, ...(state === 'offline' ? [el('span', 'sode-ov__muted', 'offline')] : []));
  };
  function edit() {
    const input = el('input', 'pb-input sode-panels__device-input');
    input.type = 'text';
    input.maxLength = 40;
    input.value = i.device;
    input.autocomplete = 'off';
    input.dataset.renameField = i.device;
    input.setAttribute('aria-label', `New name for ${i.device}`);
    const err = el('span', 'pb-note pb-note--error');
    err.setAttribute('role', 'alert');
    err.hidden = true;
    const pin = el('input', 'pb-input');
    pin.type = 'password';
    pin.autocomplete = 'off';
    pin.dataset.renamePin = i.device;
    pin.setAttribute('aria-label', 'Advanced Settings PIN');
    const pinRow = el('label', 'sode-tpl__pin', 'Advanced Settings PIN', pin);
    pinRow.hidden = true;
    let busy = false;
    const fail = (t) => {
      err.textContent = t;
      err.hidden = !t;
    };
    const keydown = async (e) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        return show();
      }
      if (e.key !== 'Enter' || busy) return;
      e.preventDefault();
      const name = input.value.trim();
      if (name === i.device) return show();
      const problem = renameProblem(name);
      if (problem) return fail(problem);
      busy = true;
      const r = await rename(p.id, i.device, name, pin.value);
      busy = false;
      pin.value = '';
      if (!r) return show();
      fail(r.why);
      if (r.pin) {
        pinRow.hidden = false;
        pin.focus();
      }
    };
    // Moving between the name and the PIN field keeps the edit open.
    const blur = (e) => {
      if (!busy && slot.contains(input) && !slot.contains(e.relatedTarget)) show();
    };
    for (const f of [input, pin]) {
      f.addEventListener('keydown', keydown);
      f.addEventListener('blur', blur);
    }
    slot.replaceChildren(input, pinRow, err);
    input.focus();
    input.select();
  }
  show();
  return slot;
}

// The instances this user hid, with one button that shows them all again.
function hiddenLine(p, setHide) {
  const names = hiddenShown(p);
  if (!names.length) return null;
  const show = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Show them');
  show.type = 'button';
  show.dataset.showHidden = '';
  show.addEventListener('click', () => names.forEach((d) => setHide?.(p.id, d, false)));
  const shown = names.map((d) => (p.instances || []).find((i) => i.device.toLowerCase() === d)?.device || d);
  return el('p', 'sode-ov__muted sode-ov__hidden', `Hidden: ${shown.join(', ')}`, show);
}

// Who has the panel open. Only an editor is sent the names. Each instance has its own Watch checkbox, uptime strip,
// Hide and rename.
export function openNow(p, { setWatch, setHide, rename } = {}, now = Date.now()) {
  const box = el('section', 'sode-ov__opennow');
  box.id = 'panels-open';
  box.dataset.for = p.id;
  if (!p.instances) return box;
  box.append(el('h2', 'sode-ov__group', 'Open now'));
  const rows = shownInstances(p);
  const hidden = hiddenLine(p, setHide);
  if (!rows.length) {
    box.append(el('p', 'sode-ov__muted', hidden ? 'Every instance is hidden.' : 'No instance has opened this panel since Sode started.'));
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
          el('label', 'pb-check sode-ov__watchone', input, el('span', 'pb-check__label', 'Notify on disconnect')),
        ),
        uptimeStrip(i),
      ),
    );
  }
  box.append(ul);
  if (hidden) box.append(hidden);
  return box;
}

// Watch is offered where the server says the user may edit: the owner, or any panel for a Trusted user.
export function watchBox(p, setWatch) {
  if (!p.canEdit) return el('span');
  const input = el('input', 'pb-check__input');
  input.type = 'checkbox';
  input.id = 'panels-watch';
  input.checked = !!p.watching;
  // A browser asks for notification permission only after a click, so Watch (and the button) ask.
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
  return el(
    'div',
    'sode-ov__watchbox',
    el('label', 'pb-check sode-ov__watch', input, el('span', 'pb-check__label', 'Tell me when this panel drops offline')),
    note,
    allow,
  );
}

// Replaces the Open now list in place, so a refresh does not rebuild the detail pane under a hand.
// A checkbox under a hand is left alone until the next refresh.
export function paintOpenNow(p, handlers) {
  const old = document.getElementById('panels-open');
  if (!old || !p || old.dataset.for !== p.id || old.contains(document.activeElement)) return;
  old.replaceWith(openNow(p, handlers));
}
