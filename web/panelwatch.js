// Monitoring in the browser: the `panelNotice` toast, the `panelsOffline` badge and, if allowed, a
// browser notification. Imported by the Sode page and the panels pages, so a watcher is told on either.
import { el, icon, onMsg } from './core.js';
import { noticeText, badgeText, shouldNotify } from './panelwatchrules.js';

const TOAST_MS = 20000;
let stack = null;
let badge = null;

function host() {
  if (stack) return stack;
  stack = el('div', 'pb-toast-stack');
  stack.setAttribute('aria-live', 'polite');
  badge = el('a', 'pb-badge pb-badge--error');
  badge.href = 'panels';
  badge.hidden = true;
  badge.dataset.panelBadge = '';
  stack.append(badge);
  document.body.append(stack);
  return stack;
}

function toast(n) {
  const t = noticeText(n);
  const close = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm pb-toast__close', icon('x'));
  close.type = 'button';
  close.setAttribute('aria-label', 'Dismiss');
  const node = el(
    'div',
    `pb-toast pb-toast--${t.kind === 'ok' ? 'ok' : t.kind}`,
    el('span', 'pb-toast__icon', icon(t.kind === 'ok' ? 'wifi' : 'wifi-off')),
    el('div', 'pb-toast__body', el('div', 'pb-toast__title', t.title), el('div', 'pb-toast__msg', t.text)),
    close,
  );
  node.dataset.panelToast = n.state;
  if (t.kind === 'error') node.setAttribute('role', 'alert');
  const drop = () => node.remove();
  close.addEventListener('click', drop);
  host().append(node);
  // A loss stays until dismissed, since the person may be away from the screen; a return fades.
  if (n.state === 'back') setTimeout(drop, TOAST_MS);
  return t;
}

function browserNotice(t, n) {
  if (typeof Notification === 'undefined') return;
  if (!shouldNotify(Notification.permission, document.hidden, document.hasFocus())) return;
  try {
    new Notification(t.title, { body: t.text, tag: `sode-panel-${n.panel}` });
  } catch {}
}

export function setBadge(rows) {
  host();
  badge.hidden = rows.length === 0;
  badge.textContent = rows.length ? badgeText(rows) : '';
  badge.title = rows.map((r) => `${r.name}: ${r.device}`).join('\n');
}

// Asked from a click on Watch: a browser only shows its permission prompt for a gesture.
export function askNotifyPermission() {
  if (typeof Notification === 'undefined' || Notification.permission !== 'default') return;
  try {
    Notification.requestPermission();
  } catch {}
}

onMsg('panelNotice', (n) => browserNotice(toast(n), n));
onMsg('panelsOffline', (m) => setBadge(m.offline || []));
