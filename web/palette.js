// The Palette (GLOSSARY.md): one search list of what a person can use, on every page. The Palette character typed first
// in an empty Source or Destination box opens it, and so does Ctrl+Shift+P from any focus. The rows come from
// palettecatalogue.js, the lists and ranks from paletterules.js.
//
// The rows follow the browser's own sign-in: lockState, myRights and the socket's palettePanels list. A panel page's key
// never reaches them. Some rows run on Home: a preset, a filter set or a setting. On Home, palettehome.js gives
// setHomeRunner. Another page puts the row in sessionStorage and opens Home.
//
// Bookmarks (bookmarks.js) open their rows through runRow, and a grey one through explainRow. The empty query lists
// the shown list's bookmarks that have Show in Palette. Alt+B or a row's Bookmark button adds the row to Me, or to This
// browser for a Guest. openPalette({ pick }) hands the picked row to pick instead (the Bookmarks editor's Add).
import { $, el, S, on, emit, icon, request, sentence, LINK_DOWN } from './core.js';
import { chord, listStep } from './keys.js';
import { modal, fadeOut, stopFade } from './modal.js';
import { bigScroll } from './bigscroll.js';
import { closeShown } from './dialogkeys.js';
import { closeOpenConfirm } from './keyconfirm.js';
import { lockState } from './lock.js';
import { lockCovers } from './lockrules.js';
import { pref, setPref } from './device.js';
import { profileName, profile, saveProfile } from './profile.js';
import { readJSON, writeJSON, readTab, writeTab } from './store.js';
import { myRights } from './editorrights.js';
import { allPresets } from './presetstore.js';
import { allSets } from './filterstore.js';
import { layout } from './layout.js';
import { steppedIn } from './stepin.js';
import { settingsRows } from './palettesettings.js';
import { catalogue, pageOf } from './palettecatalogue.js';
import { paletteView, pathText, stepIndex, firstIndex, addPick, cleanPicks, paletteCharOf, opensPalette } from './paletterules.js';
import { shownBookmarks, bookmarksIn, saveBookmarks } from './bookmarkstore.js';
import { paletteMarks, ownScope, addBookmark, fromRow } from './bookmarkrules.js';

const PICKS_KEY = 'sode-palette-picks';
// A row sent to Home: { key, fill, offer, at }. Home runs it once, while it is fresh.
export const HANDOFF_KEY = 'sode-palette-run';
export const HANDOFF_MS = 60_000;
const BOXES = new Set(['src', 'dst', 'm-src', 'm-dst']);

const appRoot = () => new URL('./', document.baseURI);
const PAGE = pageOf(location.pathname.slice(appRoot().pathname.length));

let homeRunner = null;
// Home's runner: (row, { fill, offer }) runs a row on this page.
export const setHomeRunner = (fn) => {
  homeRunner = fn;
};

let panels = [];
let settings = [];
let rows = [];
let items = [];
let cur = -1;
let path = [];
let origin = null; // the patch box that opened the Palette
let back = null;
let picked = null; // the grey row whose why shows
let pickFn = null; // openPalette({ pick }): Enter hands the row here
let pickCancel = null; // ...and a close with no row runs this

// Last picks: Me for a signed-in user, else This browser.
const picks = () => cleanPicks(profileName() ? profile().palettePicks : readJSON(PICKS_KEY, []));
function notePick(key) {
  const next = addPick(picks(), key);
  if (profileName()) saveProfile({ palettePicks: next });
  else writeJSON(PICKS_KEY, next);
}

function who() {
  const L = lockState();
  return {
    role: L.role || '',
    user: L.user || '',
    rights: myRights(),
    locks: S.cfg?.locks || {},
    users: S.users,
    phone: layout() === 'phone',
    stepped: steppedIn(),
  };
}

export function paletteRows() {
  return catalogue({
    page: PAGE,
    presets: S.cfg ? allPresets().map((p) => ({ scope: p.scope, id: p.id, name: p.name })) : [],
    filterSets: S.cfg ? allSets().map((s) => ({ scope: s.scope, id: s.id, name: s.name })) : [],
    panels,
    settings,
    who: who(),
  });
}

// The rows with the settings and panels read: Bookmarks and Home's handoff use it, since the Palette reads them only on
// open.
export async function paletteRowsReady() {
  settings = await settingsRows();
  await readPanels();
  return paletteRows();
}

// Why the Palette does not open now, or ''. Under the Lock, on a Kiosk panel and over unsaved edits it does nothing.
export function paletteRefusal() {
  const root = document.documentElement;
  if (lockCovers(lockState()) || document.querySelector('.sode-lock.is-on')) return 'lock';
  if (root.hasAttribute('data-sode-kiosk')) return 'kiosk';
  if (root.hasAttribute('data-sode-setup')) return 'setup';
  // A dialog with edits answers through the bus (settingssave.js, presets.js, filters.js).
  const ask = { unsaved: false };
  emit('unsavedAsk', ask);
  return ask.unsaved ? 'unsaved' : '';
}

const input = el('input', {
  class: 'pb-input sode-palette__q',
  type: 'text',
  role: 'combobox',
  'aria-expanded': 'true',
  'aria-controls': 'palette-list',
  'aria-autocomplete': 'list',
  'aria-label': 'Search the Palette',
  placeholder: 'Search actions, presets, pages and settings',
  spellcheck: false,
  autocomplete: 'off',
});
const where = el('div', 'sode-palette__where');
const list = el('ul', { class: 'sode-palette__list', id: 'palette-list', role: 'listbox', 'aria-label': 'Palette' });
const why = el('div', { class: 'sode-palette__why', role: 'status', hidden: true });
const note = el('p', { class: 'sode__hint sode-palette__note', role: 'status' });
// The key hint: short words, each key in a kbd.
const FOOT = [
  ['↑↓', 'move'],
  ['Enter', 'open'],
  ['⇧Enter', 'fill the Patch boxes'],
  ['Alt+B', 'bookmark'],
  ['Esc', 'back'],
];
const PICK_FOOT = [
  ['↑↓', 'move'],
  ['Enter', 'add'],
  ['Esc', 'back'],
];
const foot = el('p', 'sode__hint sode-palette__keys');
const footOf = (keys) => keys.flatMap(([k, what]) => [el('span', 'sode-palette__key', el('kbd', null, k), ' ', what)]);

const dlg = modal({
  titleId: 'palette-title',
  title: 'Palette',
  // The search box says what this is, so the title is for screen readers only.
  headCls: 'pb-sr-only',
  cls: 'sode-palette',
  rootCls: 'sode-palette__scrim',
  rootId: 'palette',
  dialogId: 'palette-dialog',
  body: el('div', 'pb-modal__body sode-palette__body', input, where, list, why, note, foot),
  onEsc: () => back1(),
});
// A panel page draws no bars of its own (bigScrollAll), so the list gets one here.
bigScroll(list);

function rowNode(row, i) {
  const off = !row.access.ok;
  const li = el(
    'li',
    { id: `palette-opt-${i}`, role: 'option', class: `sode-palette__row${off ? ' sode-palette__row--off' : ''}`, 'data-i': i, 'data-key': row.key },
    row.icon ? icon(row.icon) : null,
    el('span', 'sode-palette__label', row.label),
    row.hint ? el('span', 'sode-palette__hint', row.hint) : null,
  );
  if (row.access.lock) li.append(el('span', 'sode-palette__tag', icon('lock'), 'Sign in'));
  if (off) li.append(el('span', 'sode-palette__tag', row.access.why.startsWith('Outside your Rights') ? 'Outside your Rights' : 'Outside your Role'));
  if (!pickFn && row.kind !== 'url')
    li.append(
      el(
        'button',
        {
          type: 'button',
          tabindex: -1,
          class: 'pb-btn pb-btn--ghost pb-btn--sm pb-btn--icon pb-tip--end sode-palette__mark',
          'aria-label': `Bookmark ${row.label}`,
          'data-pb-tip': 'Bookmark (Alt+B)',
          'data-mark': '',
        },
        icon('bookmark-plus'),
      ),
    );
  return li;
}

function draw(keep = false) {
  rows = paletteRows();
  const marks = pickFn ? [] : paletteMarks(shownBookmarks().list, rows);
  items = paletteView(rows, { path, query: input.value, last: pickFn ? [] : picks(), marks });
  if (!keep || !items[cur] || items[cur].type === 'head') cur = firstIndex(items);
  where.textContent = pathText(path, rows);
  where.hidden = !path.length;
  list.replaceChildren(
    ...items.map((it, i) => {
      if (it.type === 'head') return el('li', { role: 'presentation', class: 'sode-palette__head' }, it.label);
      if (it.type === 'cat')
        return el(
          'li',
          { id: `palette-opt-${i}`, role: 'option', class: 'sode-palette__row sode-palette__row--cat', 'data-i': i },
          el('span', 'sode-palette__label', it.label),
          el('span', 'sode-palette__hint', String(it.count)),
          icon('chevron-right'),
        );
      return rowNode(it.row, i);
    }),
  );
  if (!items.some((it) => it.type !== 'head')) list.append(el('li', { role: 'presentation', class: 'sode-palette__none' }, 'Nothing found'));
  mark();
}

function mark() {
  for (const n of list.querySelectorAll('[role="option"]')) n.setAttribute('aria-selected', String(+n.dataset.i === cur));
  if (cur >= 0) input.setAttribute('aria-activedescendant', `palette-opt-${cur}`);
  else input.removeAttribute('aria-activedescendant');
  list.querySelector(`[data-i="${cur}"]`)?.scrollIntoView({ block: 'nearest' });
}

function showWhy(row) {
  picked = row;
  const offers = row.access.offer.map((o) =>
    o === 'stepIn'
      ? el('button', { type: 'button', class: 'pb-btn pb-btn--outline pb-btn--sm', onclick: () => runRow(row, { offer: 'stepIn' }) }, 'Step in…')
      : el(
          'button',
          { type: 'button', class: 'pb-btn pb-btn--outline pb-btn--sm', onclick: () => runRow(row, { offer: 'approval' }) },
          'Open and ask for Approval on Save',
        ),
  );
  why.replaceChildren(
    el('p', null, el('strong', null, row.label), `: ${row.access.why}`),
    el('div', 'sode-palette__offers', ...offers, el('button', { type: 'button', class: 'pb-btn pb-btn--ghost pb-btn--sm', onclick: () => hideWhy() }, 'Back')),
  );
  why.hidden = false;
  (why.querySelector('button') || input).focus();
}
function hideWhy() {
  picked = null;
  why.hidden = true;
  input.focus();
}

// An outside web address. A new tab never gets this page as its opener.
export function openUrl(url, newTab) {
  if (newTab) window.open(url, '_blank', 'noopener,noreferrer');
  else location.assign(url);
}

// Pages and panels open in this tab, or in a new one (newTab, from a bookmark). Home's rows run here on Home, else go
// to Home with the row. A row of kind 'url' is a bookmark's web address. press: a bookmark presses a Settings button
// (palettehome.js), where the Palette only shows it.
export function runRow(row, { fill = false, offer = '', newTab = false, press = false } = {}) {
  close(false);
  if (row.kind === 'url') return openUrl(row.target.url, row.target.newTab);
  if (!offer) notePick(row.key);
  if (offer === 'stepIn' && !homeRunner) return toHome({ key: 'action:stepIn' });
  if (row.kind === 'action' && !row.home) return runHere(row);
  if ((row.kind === 'page' || row.kind === 'panel') && !row.home) {
    const to = new URL(row.target.href, appRoot());
    return newTab ? window.open(to, '_blank', 'noopener') : location.assign(to);
  }
  if (homeRunner) return homeRunner(row, { fill, offer, press });
  toHome({ key: row.key, fill, offer, press });
}

// A grey row from outside the Palette (a bookmark with No access): the Palette opens with its why and offers.
export function explainRow(row) {
  openPalette();
  showWhy(row);
}

// The row's Bookmark action: Me for a signed-in user, else This browser.
async function bookmarkRow(row) {
  if (!row || row.kind === 'url') return;
  const scope = ownScope(!!profileName());
  const r = addBookmark(bookmarksIn(scope), fromRow(row));
  if (r.err) return say(sentence(r.err));
  const res = await saveBookmarks(scope, r.list);
  say(res.ok ? `${row.label}: bookmarked in ${scope === 'user' ? 'Me' : 'This browser'}.` : sentence(res.err) || 'Bookmark not saved.');
}
function say(text) {
  note.textContent = text;
}

function toHome(h) {
  writeTab(HANDOFF_KEY, JSON.stringify({ ...h, at: Date.now() }));
  location.assign(appRoot());
}

// The one-time handoff to Home, or null when there is none or it is stale.
export function takeHandoff(now = Date.now()) {
  const raw = readTab(HANDOFF_KEY);
  if (!raw) return null;
  writeTab(HANDOFF_KEY, null);
  try {
    const h = JSON.parse(raw);
    return h && typeof h.key === 'string' && now - h.at < HANDOFF_MS ? h : null;
  } catch {
    return null;
  }
}

function runHere(row) {
  const id = row.target.id;
  if (id === 'lock') emit('localLock');
  else if (id === 'signIn') emit('login');
  else if (id === 'theme') {
    const root = document.documentElement;
    const dark = root.dataset.theme ? root.dataset.theme === 'dark' : matchMedia('(prefers-color-scheme: dark)').matches;
    setPref('theme', dark ? 'light' : 'dark');
  }
}

function pick(fill = false) {
  const it = items[cur];
  if (!it) return;
  if (it.type === 'cat') {
    path = [...path, it.id];
    input.value = '';
    draw();
    return;
  }
  if (it.type !== 'row') return;
  if (pickFn) {
    const fn = pickFn;
    pickCancel = null;
    close(false);
    return fn(it.row);
  }
  if (!it.row.access.ok) return showWhy(it.row);
  runRow(it.row, { fill });
}

// Esc and Backspace: the why line, then the search, then one level up, then close.
function back1() {
  if (picked) return hideWhy();
  if (input.value) {
    input.value = '';
    draw();
    return;
  }
  if (path.length) {
    path = path.slice(0, -1);
    draw();
    return;
  }
  close(true);
}

input.addEventListener('input', () => draw());
input.addEventListener('keydown', (e) => {
  const step = listStep(e) || (!e.ctrlKey && !e.metaKey && !e.altKey && { ArrowDown: 1, ArrowUp: -1 }[e.key]) || 0;
  if (step) {
    e.preventDefault();
    const i = stepIndex(items, cur, step);
    if (i >= 0) cur = i;
    mark();
    return;
  }
  if (e.key === 'Enter' && !e.ctrlKey && !e.metaKey && !e.altKey) {
    e.preventDefault();
    pick(e.shiftKey);
    return;
  }
  if (chord(e) === 'alt+b') {
    e.preventDefault();
    if (!e.repeat && !pickFn && items[cur]?.type === 'row') bookmarkRow(items[cur].row);
    return;
  }
  if (e.key === 'Backspace' && !input.value) {
    e.preventDefault();
    back1();
    return;
  }
  // The Palette character again, first: the box searches for the character itself.
  const ch = paletteCharOf(pref('paletteChar'));
  if (origin && !path.length && opensPalette({ value: input.value, key: e.key, ctrlKey: e.ctrlKey, metaKey: e.metaKey, altKey: e.altKey }, ch)) {
    e.preventDefault();
    const box = origin;
    close(false);
    box.value = ch;
    box.dispatchEvent(new Event('input', { bubbles: true }));
    box.focus();
  }
});
list.addEventListener('mousedown', (e) => e.preventDefault());
list.addEventListener('click', (e) => {
  const n = e.target.closest('[data-i]');
  if (!n) return;
  cur = +n.dataset.i;
  if (e.target.closest('[data-mark]')) {
    mark();
    return bookmarkRow(items[cur]?.row);
  }
  pick(e.shiftKey);
});

// The panels the browser's own sign-in reaches; asked on each open, since a sign-in or a share changes them.
async function readPanels() {
  const r = await request('palettePanelsResult', { t: 'palettePanels' });
  if (r.err === LINK_DOWN) return;
  panels = r.ok && Array.isArray(r.panels) ? r.panels : [];
  if (dlg.shown()) draw(true);
  emit('paletteRows');
}

// The box grows into the Palette. --pb-motion-normal is 0 under reduced motion, so it opens at once there.
function grow(from) {
  const d = dlg.dialog;
  const a = from.getBoundingClientRect();
  const b = d.getBoundingClientRect();
  if (!a.width || !b.width) return;
  d.classList.remove('sode-palette--grow');
  d.style.transform = `translate(${a.left - b.left}px, ${a.top - b.top}px) scale(${a.width / b.width}, ${a.height / b.height})`;
  d.getBoundingClientRect();
  d.classList.add('sode-palette--grow');
  d.style.transform = '';
}

// pick(row): the Bookmarks editor's Add. Enter hands the row there, grey or not, and runs nothing. cancel() runs when
// the Palette closes with no row.
export function openPalette({ from = null, pick = null, cancel = null } = {}) {
  if (dlg.shown()) return;
  origin = from;
  pickFn = pick;
  pickCancel = pick ? cancel : null;
  back = document.activeElement;
  path = [];
  input.value = '';
  picked = null;
  why.hidden = true;
  note.textContent = '';
  foot.replaceChildren(...footOf(pick ? PICK_FOOT : FOOT));
  stopFade(dlg.root);
  dlg.show();
  draw();
  if (from) grow(from);
  input.focus();
  readPanels();
  settingsRows().then((s) => {
    settings = s;
    if (dlg.shown()) draw(true);
  });
}

// toBox: Esc from the top. The box that opened the Palette gets focus again, empty.
function close(toBox) {
  if (!dlg.shown()) return;
  dlg.hide();
  fadeOut(dlg.root);
  why.hidden = true;
  picked = null;
  pickFn = null;
  const to = origin || back;
  origin = back = null;
  const cancel = pickCancel;
  pickCancel = null;
  if (cancel) return cancel();
  if (toBox && to?.isConnected) to.focus({ preventScroll: true });
}
export const paletteOpen = () => dlg.shown();

// Ctrl+Shift+P, Control on every OS. Capture, so it works from any focus, also inside a dialog.
document.addEventListener(
  'keydown',
  (e) => {
    if (chord(e) !== 'ctrl+shift+p') return;
    e.preventDefault();
    e.stopImmediatePropagation();
    if (e.repeat) return;
    if (dlg.shown()) return close(true);
    if (paletteRefusal()) return;
    closeOpenConfirm();
    closeShown();
    $('keys')?.querySelector('#keys-close')?.click();
    openPalette();
  },
  true,
);

// The Palette character first in an empty Source or Destination box (both layouts).
function boxKey(e, key) {
  const t = e.target;
  if (!(t instanceof HTMLInputElement) || !BOXES.has(t.id) || e.isComposing) return;
  const ch = paletteCharOf(pref('paletteChar'));
  if (!opensPalette({ value: t.value, key, ctrlKey: e.ctrlKey, metaKey: e.metaKey, altKey: e.altKey }, ch) || paletteRefusal()) return;
  e.preventDefault();
  e.stopImmediatePropagation();
  openPalette({ from: t });
}
document.addEventListener('keydown', (e) => boxKey(e, e.key), true);
// A phone keyboard often names no key in keydown; the text it inserts is still known here.
document.addEventListener('beforeinput', (e) => e.inputType === 'insertText' && boxKey(e, e.data), true);

on('locking', () => close(false));
on('access', () => dlg.shown() && draw(true));
on('presets', () => dlg.shown() && draw(true));
on('filters', () => dlg.shown() && draw(true));
on('bookmarks', () => dlg.shown() && draw(true));
