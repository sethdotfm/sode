// Monitoring in the browser: the `panelNotice` toast, the `panelsOffline` badge and, if allowed, a
// browser notification. Imported by the Sode page and the panels pages, so a watcher is told on either.
import { el, icon, onMsg, wsSend } from './core.js';
import { noticeText, badgeText, bannerText, clearNote, shouldNotify, lostText } from './panelwatchrules.js';

const TOAST_MS = 20000;
let stack = null;
let badge = null;

function host() {
  if (stack) return stack;
  stack = el('div', 'pb-toast-stack');
  stack.setAttribute('aria-live', 'polite');
  badge = el('a', 'pb-badge pb-badge--error pb-tip--end');
  badge.href = 'panels';
  badge.hidden = true;
  badge.dataset.panelBadge = '';
  stack.append(badge);
  document.body.append(stack);
  return stack;
}

// Counters of open lost toasts by panel and device, so the return of the device freezes its count.
const counters = new Map();
const counterKey = (n) => `${n.panel}\n${String(n.device).toLowerCase()}`;

function toast(n) {
  const t = noticeText(n);
  if (n.state === 'back') counters.get(counterKey(n))?.();
  const title = el('a', 'sode__toast-link', t.title);
  title.href = `panels/${n.panel}`;
  const msg = el('div', 'pb-toast__msg', t.text);
  const close = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm pb-toast__close', icon('x'));
  close.type = 'button';
  close.setAttribute('aria-label', 'Dismiss');
  const node = el(
    'div',
    `pb-toast pb-toast--${t.kind === 'ok' ? 'ok' : t.kind}`,
    el('span', 'pb-toast__icon', icon(t.kind === 'ok' ? 'wifi' : 'wifi-off')),
    el('div', 'pb-toast__body', el('div', 'pb-toast__title', title), msg),
    close,
  );
  node.dataset.panelToast = n.state;
  if (t.kind === 'error') node.setAttribute('role', 'alert');
  const stop = n.state === 'lost' ? count(n, msg) : () => {};
  const drop = () => {
    stop();
    node.remove();
  };
  close.addEventListener('click', drop);
  host().append(node);
  // A loss stays until dismissed, since the person may be away from the screen; a return fades.
  if (n.state === 'back') {
    close.classList.add('sode__toast-timed');
    close.style.setProperty('--toast-ms', `${TOAST_MS}ms`);
    setTimeout(drop, TOAST_MS);
  }
  return t;
}

// Counts the silence up while the toast is open, from when the notice arrived.
function count(n, msg) {
  const from = Date.now();
  const timer = setInterval(() => {
    msg.textContent = lostText(n.device, n.secs + (Date.now() - from) / 1000);
  }, 1000);
  const key = counterKey(n);
  const stop = () => {
    clearInterval(timer);
    if (counters.get(key) === stop) counters.delete(key);
  };
  counters.set(key, stop);
  return stop;
}

function browserNotice(t, n) {
  if (typeof Notification === 'undefined') return;
  if (!shouldNotify(Notification.permission)) return;
  try {
    new Notification(t.title, { body: t.text, tag: `sode-panel-${n.panel}` });
  } catch {}
}

// The Sode page has a banner at the top of each layout. The badge is for pages without one.
const banners = [];
let offline = [];
let menuOpen = false;

function mountBanners() {
  if (banners.length || !document.getElementById('conn')) return;
  for (const next of [document.getElementById('conn'), document.getElementById('m-result')]) {
    if (!next) continue;
    const box = el('div', 'sode__outage');
    box.dataset.panelOutage = '';
    next.before(box);
    banners.push(box);
  }
}

function alarm(action) {
  for (const r of offline) wsSend({ t: 'panelAlarm', panel: r.panel, instance: r.instance || '', action });
  menuOpen = false;
}

function bannerNode() {
  const t = bannerText(offline);
  const act = (label, action) => {
    const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm', label);
    b.type = 'button';
    b.dataset.panelAlarm = action;
    b.addEventListener('click', () => alarm(action));
    return b;
  };
  const toggle = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Clear', icon(menuOpen ? 'chevron-left' : 'chevron-right'));
  toggle.type = 'button';
  toggle.dataset.panelAlarm = 'menu';
  toggle.setAttribute('aria-expanded', String(menuOpen));
  toggle.addEventListener('click', () => {
    menuOpen = !menuOpen;
    drawBanners();
  });
  const link = el('a', 'pb-btn pb-btn--sm', 'Open panels');
  link.href = 'panels';
  const acts = el('div', 'sode__outage-acts', link, toggle);
  if (menuOpen) acts.append(act('Clear alarm for now', 'clear'), act(clearNote(offline).stop, 'stop'));
  return el(
    'div',
    'sode__banner-in',
    el('div', 'pb-alert pb-alert--error', icon('wifi-off'), el('div', 'sode__banner-body', el('div', 'pb-alert__title', t.title), t.text), acts),
  );
}

// The sliding wrapper stays across redraws so only the first appearance animates.
function drawBanners() {
  for (const box of banners) {
    if (!offline.length) box.replaceChildren();
    else if (box.firstChild) box.firstChild.replaceChildren(bannerNode());
    else box.append(el('div', 'sode__banner', bannerNode()));
  }
}

export function setBadge(rows) {
  host();
  mountBanners();
  offline = rows;
  if (!rows.length) menuOpen = false;
  drawBanners();
  badge.hidden = rows.length === 0 || banners.length > 0;
  badge.textContent = rows.length ? badgeText(rows) : '';
  badge.dataset.pbTip = rows.map((r) => `${r.name}: ${r.device}`).join('\n');
}

// 'granted', 'denied', 'default', or 'unsupported' where the browser has no Notification.
export const notifyPermission = () => (typeof Notification === 'undefined' ? 'unsupported' : Notification.permission);

// Asked from a click: a browser only shows its permission prompt for a gesture. Resolves to the answer.
export async function askNotifyPermission() {
  if (notifyPermission() === 'default') {
    try {
      await Notification.requestPermission();
    } catch {}
  }
  return notifyPermission();
}

onMsg('panelNotice', (n) => browserNotice(toast(n), n));
onMsg('panelsOffline', (m) => setBadge(m.offline || []));
