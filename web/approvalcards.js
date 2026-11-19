// The server sends cards only to pages that called showApprovalCards(); a panel page does not.

import { el, S, on, emit, onMsg, wsSend, sentence } from './core.js';
import { changeText, leftText, endText } from './approvalcardrules.js';

{
  const css = document.createElement('link');
  css.rel = 'stylesheet';
  css.href = 'approvalcards.css';
  document.head.append(css);
}

// Live from the server: another browser can approve an action that needs this Role.
const approvers = { trusted: false, administrator: false };
export const canApprove = (key) => !!approvers[key];

const END_SHOWN_MS = 5000;
let stack = null;
let held = []; // ended cards that wait for the Lock to go
let tick = 0;
let seq = 0;
let asking = null; // an approvalAsk with no answer yet.
let mine = null; // this browser's pending request.
const cards = new Map(); // approver cards by request.

function stackEl() {
  if (!stack) {
    stack = el('div', 'pb-toast-stack sode-approvals');
    stack.id = 'approval-cards';
    document.body.append(stack);
  }
  return stack;
}

function button(label, cls, fn) {
  const b = el('button', `pb-btn pb-btn--sm ${cls}`, label);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
}

function toast(kind, title, ...body) {
  const head = el('div', 'pb-toast__title', title);
  const box = el('div', 'pb-toast__body', head, ...body);
  const node = el('div', `pb-toast pb-toast--${kind} sode-approvals__card`, box);
  node.setAttribute('role', 'status');
  stackEl().append(node);
  return { node, head, box };
}

function ending(card, kind, text, ...extra) {
  card.node.className = `pb-toast pb-toast--${kind} sode-approvals__card`;
  card.head.textContent = text;
  card.box.replaceChildren(card.head, ...extra);
  if (extra.length) return;
  // Under the Lock the card waits, so the result shows when the Lock is dismissed.
  if (stack.hidden) held.push(card.node);
  else setTimeout(() => card.node.remove(), END_SHOWN_MS);
}

const leftNode = () => el('span', 'sode-approvals__left');

// One timer for every countdown, only while one shows.
function ticking() {
  const live = [...cards.values(), ...(mine?.left ? [mine] : [])];
  if (!live.length) {
    clearInterval(tick);
    tick = 0;
    return;
  }
  for (const c of live) c.left.textContent = `${leftText(c.until - Date.now())} left`;
  if (!tick) tick = setInterval(ticking, 1000);
}

export const requesting = () => !!(asking || mine);

// Resolves '' when the server accepted it, else the reason.
export function requestApproval(action, { what, onEnd = () => {}, onTimeout = () => {} }) {
  if (requesting()) return Promise.resolve('An Approval request is already open.');
  const id = ++seq;
  if (!wsSend({ t: 'approvalAsk', id, for: action })) return Promise.resolve('Not connected to Sode.');
  return new Promise((resolve) => {
    asking = { id, what, onEnd, onTimeout, resolve };
  });
}

export function cancelRequest() {
  if (mine) wsSend({ t: 'approvalCancel' });
}

onMsg('approvalAskResult', (m) => {
  if (!asking || m.id !== asking.id) return;
  const a = asking;
  asking = null;
  if (!m.ok) return a.resolve(sentence(m.err) || 'Not sent.');
  mine = { onEnd: a.onEnd, onTimeout: a.onTimeout, what: a.what };
  a.resolve('');
});

onMsg('approvalWaiting', (m) => {
  if (!mine || mine.card) return;
  mine.request = m.request;
  mine.until = Date.now() + m.left;
  mine.left = leftNode();
  const cancel = button('Cancel', 'pb-btn--outline', cancelRequest);
  cancel.id = 'approval-wait-cancel';
  mine.card = toast('info', 'Waiting for approval', el('div', 'pb-toast__msg', `${mine.what}. `, mine.left), el('div', 'sode-approvals__actions', cancel));
  mine.card.node.id = 'approval-waiting';
  ticking();
});

onMsg('approvalEnded', (m) => {
  if (!mine || m.request !== mine.request) return;
  const { card, onEnd, onTimeout } = mine;
  mine = null;
  ticking();
  const text = endText(m.end, m.by);
  if (m.end === 'timeout') {
    const here = button('Approve here', '', () => {
      card.node.remove();
      onTimeout();
    });
    here.id = 'approval-here';
    ending(
      card,
      'warn',
      text,
      el(
        'div',
        'sode-approvals__actions',
        here,
        button('Close', 'pb-btn--outline', () => card.node.remove()),
      ),
    );
  } else if (m.end === 'denied') {
    ending(
      card,
      'error',
      text,
      el(
        'div',
        'sode-approvals__actions',
        button('Close', 'pb-btn--outline', () => card.node.remove()),
      ),
    );
  } else ending(card, m.end === 'approved' ? 'ok' : 'info', text);
  onEnd(m.end);
});

function answer(request, approve) {
  const c = cards.get(request);
  if (!c) return;
  if (approve && !c.pin.value) {
    c.err.textContent = 'Enter your PIN.';
    c.err.hidden = false;
    return c.pin.focus();
  }
  if (!wsSend({ t: 'approvalAnswer', request, approve, pin: approve ? c.pin.value : '' })) return;
  for (const b of c.buttons) b.disabled = true;
  c.err.hidden = true;
}

onMsg('approvalCard', (m) => {
  if (cards.has(m.request)) return;
  const left = leftNode();
  const pin = el('input', 'pb-input');
  Object.assign(pin, { type: 'password', inputMode: 'numeric', autocomplete: 'off', placeholder: 'Your PIN' });
  pin.setAttribute('aria-label', 'Your PIN, to approve');
  pin.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    answer(m.request, true);
  });
  const err = el('div', 'pb-note pb-note--error');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const deny = button('Deny', 'pb-btn--outline', () => answer(m.request, false));
  const approve = button('Approve', 'pb-btn--ok', () => answer(m.request, true));
  deny.dataset.approvalDeny = approve.dataset.approvalApprove = m.request;
  const list = el('ul', 'sode-approvals__list', ...(m.changes || []).map((f) => el('li', '', changeText(f))));
  const stops = (m.interruptions || []).length
    ? [el('div', 'pb-note pb-note--warn', el('div', '', 'Interruptions:'), el('ul', 'sode-approvals__list', ...m.interruptions.map((s) => el('li', '', s))))]
    : [];
  const card = toast(
    'warn',
    'Approval requested',
    el('div', 'pb-toast__msg', `${m.from} asks from ${m.browser}: ${m.what}. `, left),
    ...(list.childElementCount ? [list] : []),
    ...stops,
    pin,
    err,
    el('div', 'sode-approvals__actions', deny, approve),
  );
  card.node.dataset.approvalCard = m.request;
  cards.set(m.request, { until: Date.now() + m.left, card, left, pin, err, buttons: [deny, approve] });
  ticking();
});

onMsg('approvalAnswerResult', (m) => {
  const c = cards.get(m.request);
  if (!c || m.ok) return;
  for (const b of c.buttons) b.disabled = false;
  c.pin.value = '';
  c.err.textContent = sentence(m.err);
  c.err.hidden = false;
});

onMsg('approvalCardClosed', (m) => {
  const c = cards.get(m.request);
  if (!c) return;
  cards.delete(m.request);
  ticking();
  const text = endText(m.end, m.by);
  if (!text) return void c.card.node.remove();
  ending(c.card, m.end === 'approved' ? 'ok' : m.end === 'denied' ? 'error' : 'info', text);
});

onMsg('approvers', (m) => {
  approvers.trusted = !!m.trusted;
  approvers.administrator = !!m.administrator;
  emit('approvers');
});

// A socket drop ends this browser's request on the server and loses every card's answer.
on('wsclose', () => {
  if (asking) asking.resolve('Not connected to Sode.');
  asking = null;
  if (mine?.card) ending(mine.card, 'info', 'Cancelled');
  mine?.onEnd('cancelled');
  mine = null;
  for (const c of cards.values()) c.card.node.remove();
  cards.clear();
  approvers.trusted = approvers.administrator = false;
  emit('approvers');
  ticking();
});

on('access', (a) => {
  if (!stack) return;
  stack.hidden = !!a.locked;
  if (stack.hidden) return;
  for (const node of held) setTimeout(() => node.remove(), END_SHOWN_MS);
  held = [];
});

let shown = false;
export function showApprovalCards() {
  if (shown) return;
  shown = true;
  const say = () => S.wsState === 'up' && wsSend({ t: 'approvalCards', on: true });
  on('conn', say);
  say();
}
