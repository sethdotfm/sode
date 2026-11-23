import { el, emit, icon, on, onMsg, request } from './core.js';
import { toastText } from './inboxrules.js';

const TOAST_MS = 20000;

export const inbox = { cards: [], sent: [], got: false };

// The signed-in user's name from the last `access`; '' with nobody signed in. Only they send or receive.
let user = '';
on('access', (L) => {
  user = L.user || '';
});
export const inboxUser = () => user;

export const ask = (m) => request('inboxResult', m);

let toasts = false; // only a page with an Inbox button shows them, never a panel page
let openFn = () => {};
export function showToasts(open) {
  toasts = true;
  openFn = open;
}

let stack = null;
function toast(c) {
  // The toast goes in panelwatch.js's stack if this page has one.
  stack ||= document.querySelector('.pb-toast-stack');
  if (!stack) {
    stack = el('div', 'pb-toast-stack');
    stack.setAttribute('aria-live', 'polite');
    document.body.append(stack);
  }
  const title = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode-inbox__toast-open', 'Open Inbox');
  title.type = 'button';
  const close = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm pb-toast__close sode__toast-timed', icon('x'));
  close.type = 'button';
  close.setAttribute('aria-label', 'Dismiss');
  close.style.setProperty('--toast-ms', `${TOAST_MS}ms`);
  const node = el(
    'div',
    'pb-toast',
    el('span', 'pb-toast__icon', icon('inbox')),
    el('div', 'pb-toast__body', el('div', 'pb-toast__title', 'New copy in your Inbox'), el('div', 'pb-toast__msg', toastText(c)), title),
    close,
  );
  node.dataset.inboxToast = c.id;
  const drop = () => node.remove();
  close.addEventListener('click', drop);
  title.addEventListener('click', () => {
    drop();
    openFn();
  });
  stack.append(node);
  setTimeout(drop, TOAST_MS);
}

onMsg('inbox', (m) => {
  inbox.cards = m.cards || [];
  inbox.sent = m.sent || [];
  inbox.got = true;
  if (toasts) for (const id of m.new || []) toast(inbox.cards.find((c) => c.id === id) || { id, count: 0 });
  emit('inbox');
});
