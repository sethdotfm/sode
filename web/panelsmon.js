// Monitoring on the panels page: the online mark of each row, the Watch checkbox and the Open now list of the
// chosen panel. Kept out of panelslist.js so that file only calls three functions.
import { el } from './core.js';
import { askNotifyPermission } from './panelwatch.js';
import { durationText } from './panelwatchrules.js';

export const MON_POLL_MS = 10000;

const badge = (online) => el('span', `pb-badge${online ? ' pb-badge--ok' : ''}`, online ? 'Online' : 'Offline');

// Fills every row's empty online span (the hook row() leaves) from the panels last read.
export function paintOnline(panels) {
  for (const span of document.querySelectorAll('[data-online]')) {
    const p = panels.find((x) => x.id === span.dataset.online);
    span.replaceChildren(...(p ? [badge(p.online)] : []));
  }
}

// Who has the panel open. Only an editor is sent the names.
export function openNow(p, now = Date.now()) {
  const box = el('section', 'sode-ov__opennow');
  box.id = 'panels-open';
  box.dataset.for = p.id;
  if (!p.instances) return box;
  box.append(el('h2', 'sode-ov__group', 'Open now'));
  if (!p.instances.length) {
    box.append(el('p', 'sode-ov__muted', 'No screen has opened this panel since Sode started.'));
    return box;
  }
  const ul = el('ul', 'sode-ov__open');
  for (const i of p.instances) {
    const ago = i.online ? '' : `last heard ${durationText((now - Date.parse(i.seen)) / 1000)} ago`;
    ul.append(el('li', '', badge(i.online), el('strong', '', i.device), ago && el('span', 'sode-ov__muted', ago)));
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
  input.addEventListener('change', () => {
    if (input.checked) askNotifyPermission();
    setWatch(p.id, input.checked);
  });
  return el('label', 'pb-check sode-ov__watch', input, el('span', 'pb-check__label', 'Tell me when this panel drops offline'));
}

// Replaces the Open now list in place, so a refresh does not rebuild the detail pane under a hand.
export function paintOpenNow(p) {
  const old = document.getElementById('panels-open');
  if (old && p && old.dataset.for === p.id) old.replaceWith(openNow(p));
}
