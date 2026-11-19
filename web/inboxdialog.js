// A panel page has no data-inbox, so no button and no toast. The phone keeps Decline only.

import { el, icon, off, on } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { makeTabs } from './tabs.js';
import { segmented } from './formbits.js';
import { layout } from './layout.js';
import { desktopOnly } from './desktoponly.js';
import { inbox, ask, showToasts } from './inbox.js';
import { choicesFor, missingText } from './importrules.js';
import * as R from './inboxrules.js';

const btn = (cls, text, fn) => {
  const b = el('button', `pb-btn ${cls}`, text);
  b.type = 'button';
  b.addEventListener('click', fn);
  return b;
};
const CAT_TEXT = { presets: 'Presets', filterSets: 'Filter sets', panels: 'Panels' };
const when = (ms) => new Date(ms).toLocaleDateString(undefined, { day: 'numeric', month: 'short' });

let signedIn = false;
const buttons = [];

function headButton() {
  const count = el('span', 'pb-badge pb-badge--info sode-inbox__count');
  const b = el('button', 'pb-btn pb-btn--ghost pb-btn--sm sode-inbox__btn', icon('inbox'), el('span', 'sode-inbox__word', 'Inbox'), count);
  b.type = 'button';
  b.dataset.inboxButton = '';
  b.addEventListener('click', () => openDialog());
  buttons.push({ b, count });
  return b;
}

function drawButtons() {
  const n = inbox.cards.length;
  for (const { b, count } of buttons) {
    b.hidden = !signedIn;
    count.textContent = String(n);
    count.hidden = !n;
    b.setAttribute('aria-label', R.countLabel(n));
  }
}

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal pb-modal--md sode-inbox');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'inbox-title');
dialog.tabIndex = -1;
const titleEl = Object.assign(el('span', 'pb-modal__title', 'Inbox'), { id: 'inbox-title' });
const cardList = el('div', 'sode-inbox__list');
cardList.id = 'inbox-cards';
const sentList = el('div', 'sode-inbox__list');
sentList.id = 'inbox-sent';
const inboxPanel = el('div', null, cardList);
const sentPanel = el('div', null, sentList);
const review = el('div', 'sode-inbox__review');
review.hidden = true;
const phoneNote = el('div');
const note = el('span', 'sode__hint');
note.id = 'inbox-note';
note.setAttribute('role', 'status');
const closeBtn = btn('pb-btn--outline', 'Close', () => close());
const backBtn = btn('pb-btn--outline', 'Back', () => endReview());
const acceptBtn = btn('pb-btn--ok', 'Accept', () => accept());
acceptBtn.id = 'inbox-accept';
const reviewActs = el('span', 'sode__xi-acts', backBtn, acceptBtn);
reviewActs.hidden = true;
const tabs = makeTabs([
  { id: 'inbox-tab', text: 'Inbox', panel: inboxPanel },
  { id: 'sent-tab', text: 'Sent', panel: sentPanel },
]);
dialog.append(
  el('div', 'pb-modal__header', titleEl),
  el('div', 'pb-modal__body sode-inbox__body', tabs.node, phoneNote, inboxPanel, sentPanel, review),
  el('div', 'pb-modal__footer', closeBtn, note, reviewActs),
);
root.append(dialog);

let open = false;
let back = null;
let reviewing = null; // { card, plan, pick }
let busy = false;
const phone = () => layout() === 'phone';

const setNote = (t, error = false) => {
  note.textContent = t;
  note.classList.toggle('sode__error-text', error);
};

async function answer(m, ok) {
  if (busy) return;
  busy = true;
  setNote('');
  const r = await ask(m);
  busy = false;
  if (r.ok) setNote(ok);
  else setNote(r.err, true);
  drawLists();
}

function cardRow(c) {
  const acts = el('span', 'sode-inbox__acts');
  if (!phone()) acts.append(btn('pb-btn--ok pb-btn--sm', 'Accept…', () => startReview(c)));
  const decline = btn('pb-btn--outline pb-btn--sm', 'Decline', () => answer({ t: 'inboxDecline', inbox: { card: c.id } }, 'Declined.'));
  decline.dataset.decline = c.id;
  acts.append(decline);
  const row = el(
    'div',
    'sode-inbox__row',
    el(
      'div',
      'sode-inbox__what',
      el('strong', null, R.fromText(c)),
      el('span', 'sode__hint', `${R.itemsText(c.items || [])} · sent ${when(c.at)} · expires ${when(c.expires)}`),
    ),
    acts,
  );
  row.dataset.card = c.id;
  if (c.items?.length) row.append(el('span', 'sode__hint sode-inbox__names', c.items.map((it) => it.name).join(', ')));
  return row;
}

function sentRow(c) {
  const acts = el('span', 'sode-inbox__acts', el('span', 'pb-badge sode-inbox__state', R.stateText(c)));
  if (c.state === 'waiting' && !phone())
    acts.append(btn('pb-btn--outline pb-btn--sm', 'Withdraw', () => answer({ t: 'inboxWithdraw', inbox: { card: c.id } }, 'Withdrawn.')));
  const row = el(
    'div',
    'sode-inbox__row',
    el(
      'div',
      'sode-inbox__what',
      el('strong', null, `To ${c.to}`),
      el('span', 'sode__hint', `${c.count} ${c.count === 1 ? 'item' : 'items'} · sent ${when(c.at)}`),
    ),
    acts,
  );
  row.dataset.sent = c.id;
  return row;
}

function drawLists() {
  if (!open || reviewing) return;
  const cards = inbox.cards.map(cardRow);
  cardList.replaceChildren(...(cards.length ? cards : [el('p', 'sode__hint', 'Inbox empty')]));
  // Newest first, as the server sends them oldest first.
  const sent = [...inbox.sent].reverse().map(sentRow);
  sentList.replaceChildren(...(sent.length ? sent : [el('p', 'sode__hint', 'None sent (30 days)')]));
  phoneNote.replaceChildren(phone() ? desktopOnly('accept a copy or withdraw one') : '');
}

async function startReview(c) {
  if (busy) return;
  busy = true;
  setNote('Checking…');
  const r = await ask({ t: 'inboxReview', inbox: { card: c.id, plan: { pick: {}, choices: {} } } });
  busy = false;
  if (!open) return;
  if (!r.ok) return setNote(r.err, true);
  setNote('');
  reviewing = { card: c, plan: r.plan, pick: R.acceptPick(r.plan) };
  drawReview();
}

function box(key, text, checked, disabled, onChange) {
  const b = el('input', 'pb-check__input');
  b.type = 'checkbox';
  b.dataset.key = key;
  b.checked = checked;
  b.disabled = disabled;
  b.addEventListener('change', () => onChange(b.checked));
  return el('label', 'pb-check', b, el('span', 'pb-check__label', text));
}

function itemRow(cat, c, it, i) {
  const { pick } = reviewing;
  const ticked = pick.items[cat].has(it.key);
  const row = el(
    'div',
    'sode__xi-item',
    box(`item:${cat}:${it.key}`, it.name, ticked, !!c.refused || !!it.refused, (on) => {
      if (on) pick.items[cat].add(it.key);
      else pick.items[cat].delete(it.key);
      drawReview();
    }),
    el(
      'span',
      'sode__hint',
      ...(it.broken?.length ? [el('span', 'pb-badge pb-badge--error', 'Broken'), ' '] : []),
      // The server's As is for its own choice, Rename; another choice keeps no new name.
      it.refused || it.note || (it.as && (!it.clash || pick.choices[cat][it.key] === 'rename') ? `As ${it.as}` : ''),
    ),
  );
  row.dataset.item = it.key;
  if (ticked && pick.choices[cat][it.key]) {
    const seg = segmented(`inbox-choice-${cat}-${i}`, `${it.name} clash`, choicesFor(cat), (v) => {
      pick.choices[cat][it.key] = v;
      drawReview();
    });
    seg.set(pick.choices[cat][it.key]);
    for (const r of seg.node.querySelectorAll('input')) r.dataset.key = `choice:${cat}:${it.key}:${r.value}`;
    seg.node.classList.add('sode__xi-choice');
    row.append(el('div', 'sode__xi-sub', el('span', 'sode__hint', `You have ${it.clash}. `), seg.node));
  }
  if (it.broken?.length)
    row.append(el('span', 'sode__hint sode__xi-sub sode__error-text', `Missing ${it.broken.map((m) => missingText(m, off())).join(', ')}`));
  return row;
}

function drawReview() {
  const { card, plan, pick } = reviewing;
  const rows = [];
  for (const cat of R.CARD_CATS) {
    const c = plan.categories[cat];
    if (!c) continue;
    rows.push(el('div', 'sode__xi-head', el('strong', null, CAT_TEXT[cat]), el('span', `sode__hint${c.refused ? ' sode__error-text' : ''}`, c.refused || '')));
    rows.push(el('div', 'sode__xi-items', ...c.items.map((it, i) => itemRow(cat, c, it, i))));
  }
  const focused = review.contains(document.activeElement) ? document.activeElement.dataset.key : '';
  review.replaceChildren(el('p', null, R.fromText(card)), el('div', 'sode__xi-cats', ...rows));
  if (focused) [...review.querySelectorAll('[data-key]')].find((x) => x.dataset.key === focused)?.focus();
  showReview(true);
  acceptBtn.disabled = !R.pickedCount(pick);
}

function showReview(on) {
  titleEl.textContent = on ? 'Accept a copy?' : 'Inbox';
  tabs.node.hidden = on;
  review.hidden = !on;
  reviewActs.hidden = !on;
  if (on) inboxPanel.hidden = sentPanel.hidden = true;
  else tabs.select(tabs.current());
}

function endReview() {
  reviewing = null;
  showReview(false);
  drawLists();
}

async function accept() {
  if (!reviewing || busy) return;
  busy = acceptBtn.disabled = true;
  setNote('Accepting…');
  const r = await ask({ t: 'inboxAccept', inbox: { card: reviewing.card.id, plan: R.acceptReq(reviewing.pick) } });
  busy = acceptBtn.disabled = false;
  if (!r.ok) return setNote(r.err, true);
  const failed = Object.keys(r.failed || {}).length;
  endReview();
  setNote(failed ? `Accepted. ${failed} panels were not written.` : 'Accepted.');
}

export function openDialog() {
  if (!signedIn) return;
  if (!open) back = document.activeElement;
  open = true;
  reviewing = null;
  showReview(false);
  tabs.select('inbox-tab');
  setNote('');
  drawLists();
  ask({ t: 'inboxList' });
  dlg.show();
  dialog.focus();
}

function close() {
  open = false;
  reviewing = null;
  dlg.hide();
  if (back?.isConnected) back.focus();
}

let dlg = null;
const slots = document.querySelectorAll('[data-inbox]');
if (slots.length) {
  document.body.append(root);
  dlg = trapDialog(root, dialog, close);
  for (const s of slots) s.append(headButton());
  showToasts(openDialog);
  drawButtons();
  on('inbox', () => {
    drawButtons();
    drawLists();
  });
  on('access', (L) => {
    signedIn = !!L.user;
    drawButtons();
    if (!signedIn && open) close();
  });
  on('locking', () => {
    if (open) close();
  });
}
