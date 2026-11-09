// Monitoring on the panels page: the online mark of each row, the Watch checkbox and the Open now list of the
// chosen panel. Kept out of panelslist.js so that file only calls three functions.
import { el } from './core.js';
import { askNotifyPermission, notifyPermission } from './panelwatch.js';
import { durationText, permissionText } from './panelwatchrules.js';

// Fast enough that the online mark follows a screen within a couple of seconds, with no push channel to the page.
export const MON_POLL_MS = 2000;

const badge = (online) => el('span', `pb-badge${online ? ' pb-badge--ok' : ''}`, online ? 'Online' : 'Offline');

// Fills every row's empty online span (the hook row() leaves) from the panels last read.
export function paintOnline(panels) {
  for (const span of document.querySelectorAll('[data-online]')) {
    const p = panels.find((x) => x.id === span.dataset.online);
    span.replaceChildren(...(p ? [badge(p.online)] : []));
  }
}

// Every screen of the list, then the screens the user watches that Sode has not heard from since it started.
export function listed(p) {
  const seen = new Set(p.instances.map((i) => i.device.toLowerCase()));
  const gone = (p.watchingInstances || []).filter((d) => !seen.has(d)).map((d) => ({ device: d, online: false, seen: '' }));
  return [...p.instances, ...gone];
}

// Who has the panel open. Only an editor is sent the names. Each screen has its own Watch checkbox.
export function openNow(p, setWatch, now = Date.now()) {
  const box = el('section', 'sode-ov__opennow');
  box.id = 'panels-open';
  box.dataset.for = p.id;
  if (!p.instances) return box;
  box.append(el('h2', 'sode-ov__group', 'Open now'));
  const rows = listed(p);
  if (!rows.length) {
    box.append(el('p', 'sode-ov__muted', 'No screen has opened this panel since Sode started.'));
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
        badge(i.online),
        el('strong', '', i.device),
        ago && el('span', 'sode-ov__muted', ago),
        el('label', 'pb-check sode-ov__watchone', input, el('span', 'pb-check__label', 'Tell me when this screen drops offline')),
      ),
    );
  }
  box.append(ul);
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
export function paintOpenNow(p, setWatch) {
  const old = document.getElementById('panels-open');
  if (!old || !p || old.dataset.for !== p.id || old.contains(document.activeElement)) return;
  old.replaceWith(openNow(p, setWatch));
}
