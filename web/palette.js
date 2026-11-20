// The Palette (GLOSSARY.md): one search list of what a person can use, on every page. The Palette character typed first
// in an empty Source or Destination box opens it, and so does Ctrl+Shift+P from any focus. The rows come from
// palettecatalogue.js, the lists and ranks from paletterules.js.
//
// The rows follow the browser's own sign-in: lockState, myRights and the socket's palettePanels list. A panel page's key
// never reaches them. Some rows run on Home: a preset, a filter set or a setting. On Home, palettehome.js gives
// setHomeRunner. Another page puts the row in sessionStorage and opens Home.
//
// Bookmarks (bookmarks.js) open their rows through runRow, and a grey one through explainRow. The empty query lists
// the shown list's bookmarks that have Show in Palette. A grey row (outside the Rights or Role) shows only as such a
// bookmark. Alt+B or a row's Bookmark button adds the row to Me, or to This
// browser for a Guest. openPalette({ pick }) hands the picked row to pick instead (the Bookmarks editor's Add).
import { $, el, S, on, emit, icon, request, sentence, LINK_DOWN } from './core.js';
import { chord, listStep, keyLabel } from './keys.js';
import { modal, fadeOut, stopFade } from './modal.js';
import { bigScroll } from './bigscroll.js';
import { closeShown } from './dialogkeys.js';
import { closeOpenConfirm } from './keyconfirm.js';
import { lockState } from './lock.js';
import { lockCovers } from './lockrules.js';
import { pref, isDark, flipTheme } from './device.js';
import { profileName, profile, saveProfile } from './profile.js';
import { readJSON, writeJSON, readTab, writeTab } from './store.js';
import { myRights } from './editorrights.js';
import { allPresets } from './presetstore.js';
import { allSets } from './filterstore.js';
import { layout } from './layout.js';
import { steppedIn, stepOut } from './stepin.js';
import { settingsRows } from './palettesettings.js';
import { catalogue, pageOf } from './palettecatalogue.js';
import { showResult } from './banner.js';
import {
  paletteView,
  pathText,
  stepIndex,
  firstIndex,
  keyMove,
  emptyBackspace,
  addPick,
  cleanPicks,
  addUse,
  cleanUses,
  countsAsUse,
  dropPick,
  dropUse,
  PICK_UNDO_MS,
  paletteCharOf,
  opensPalette,
} from './paletterules.js';
import { hiddenCounts, hiddenLine, numberHint, footKeys } from './palettehints.js';
import { hasKeyList } from './shortcuts.js';
import { rereadNames } from './rereadnames.js';
import { shownBookmarks, bookmarksIn, saveBookmarks } from './bookmarkstore.js';
import { paletteMarks, ownScope, addBookmark, fromRow } from './bookmarkrules.js';

const PICKS_KEY = 'sode-palette-picks';
// How often each row was picked, per user name ('' a Guest): { [name]: uses } (paletterules.js cleanUses). This browser
// only for now, also for a signed-in user.
const USES_KEY = 'sode-palette-uses';
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
let panelsHidden = 0; // panels the server did not list for this sign-in (palettePanels hidden)
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
const uses = () => cleanUses(readJSON(USES_KEY, {})?.[profileName()]);
function saveUses(next) {
  const all = readJSON(USES_KEY, {});
  writeJSON(USES_KEY, { ...(all && typeof all === 'object' && !Array.isArray(all) ? all : {}), [profileName()]: next });
}
function savePicks(next) {
  if (profileName()) saveProfile({ palettePicks: next });
  else writeJSON(PICKS_KEY, next);
}
// The last first pick, in memory only: a page that opens on a pick starts with none (PICK_UNDO_MS).
let lastFirst = null;
function notePick(key) {
  if (!countsAsUse(key)) return;
  const first = !uses()[key];
  saveUses(addUse(uses(), key));
  savePicks(addPick(picks(), key));
  lastFirst = first ? { key, name: profileName(), at: Date.now() } : null;
}
function undoFirstPick() {
  const f = lastFirst;
  lastFirst = null;
  if (!f || f.name !== profileName() || Date.now() - f.at > PICK_UNDO_MS) return;
  saveUses(dropUse(uses(), f.key));
  savePicks(dropPick(picks(), f.key));
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
    protectDevice: S.cfg?.protectDevice || 0,
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
    keyList: hasKeyList(),
  });
}

// The rows with the settings and panels read: Bookmarks and Home's handoff use it, since the Palette reads them only on
// open.
export async function paletteRowsReady() {
  settings = await settingsRows();
  await readPanels();
  return paletteRows();
}

// The page's own refusal, or '': under the Lock, on a Kiosk panel and during setup. The Home key (homekey.js) has it too.
export function pageRefusal() {
  const root = document.documentElement;
  if (lockCovers(lockState()) || document.querySelector('.sode-lock.is-on')) return 'lock';
  if (root.hasAttribute('data-sode-kiosk')) return 'kiosk';
  if (root.hasAttribute('data-sode-setup')) return 'setup';
  return '';
}

// Why the Palette does not open now, or ''. Under the Lock, on a Kiosk panel and over unsaved edits it does nothing.
export function paletteRefusal() {
  return pageRefusal() || (unsavedEdits() ? 'unsaved' : '');
}

// A dialog or view with edits answers through the bus (settingssave.js, presets.js, filters.js, bookmarkeditor.js).
export function unsavedEdits() {
  const ask = { unsaved: false };
  emit('unsavedAsk', ask);
  return ask.unsaved;
}

const input = el('input', {
  class: 'pb-input sode-palette__q',
  type: 'text',
  role: 'combobox',
  'aria-expanded': 'true',
  'aria-controls': 'palette-list',
  'aria-autocomplete': 'list',
  'aria-label': 'Search the Palette',
  placeholder: 'Search, or start with panel, preset, page or setting',
  spellcheck: false,
  autocomplete: 'off',
});
const where = el('div', 'sode-palette__where');
const list = el('ul', { class: 'sode-palette__list', id: 'palette-list', role: 'listbox', 'aria-label': 'Palette' });
const why = el('div', { class: 'sode-palette__why', role: 'status', hidden: true });
const note = el('p', { class: 'sode__hint sode-palette__note', role: 'status' });
// The key hint: short words, each key in a kbd. It shows only the keys that do something on the item under the cursor
// (footKeys in palettehints.js).
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
          'data-pb-tip': `Bookmark (${keyLabel('alt+b')})`,
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
  items = [...paletteView(rows, { path, query: input.value, last: pickFn ? [] : picks(), marks, uses: pickFn ? {} : uses() }), ...extraLines(marks)];
  if (!keep || !items[cur] || items[cur].type === 'head') cur = firstIndex(items, { empty: !input.value.trim() });
  where.textContent = pathText(path, rows, input.value);
  where.hidden = !where.textContent;
  list.replaceChildren(
    ...items.map((it, i) => {
      if (it.type === 'head') return el('li', { role: 'presentation', class: it.note ? 'sode-palette__none' : 'sode-palette__head' }, it.label);
      if (it.type === 'hidden')
        return el(
          'li',
          { id: `palette-opt-${i}`, role: 'option', class: 'sode-palette__row sode-palette__row--hidden', 'data-i': i },
          icon(it.act === 'stepIn' ? 'user' : 'key-round'),
          el('span', 'sode-palette__label', it.label),
        );
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
  if (!items.length) list.append(el('li', { role: 'presentation', class: 'sode-palette__none' }, 'Nothing found'));
  mark();
}

// The rows that the list shows at one time, for PageUp and PageDown.
function pageRows() {
  const row = list.querySelector('.sode-palette__row');
  return Math.max(1, Math.floor(list.clientHeight / (row?.offsetHeight || 1)) - 1);
}

// The lines after the rows: a hint for a number, and the count of matches that the user cannot use (Enter signs in).
function extraLines(marks) {
  const out = [];
  const hint = path.length ? '' : numberHint(input.value, PAGE);
  if (hint) out.push({ type: 'head', note: true, label: hint });
  if (pickFn || path.length) return out;
  const w = who();
  const line = hiddenLine(hiddenCounts(rows, input.value, { marks, panels: panelsHidden }), {
    signedIn: !!w.user,
    canSignIn: rows.some((r) => r.key === 'action:signIn'),
    canStepIn: rows.some((r) => r.key === 'action:stepIn'),
  });
  if (line) out.push({ type: 'hidden', ...line });
  return out;
}

function mark() {
  foot.replaceChildren(...footOf(footKeys(items[cur], { query: input.value, path, pick: !!pickFn, why: !!picked })));
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
  mark();
}
function hideWhy() {
  picked = null;
  why.hidden = true;
  input.focus();
  mark();
}

// An outside web address. A new tab never gets this page as its opener.
export function openUrl(url, newTab) {
  if (newTab) window.open(url, '_blank', 'noopener,noreferrer');
  else location.assign(url);
}

// Pages and panels open in this tab, or in a new one (newTab, from a bookmark). Home's rows run here on Home, else go
// to Home with the row. A row of kind 'url' is a bookmark's web address. press: a bookmark also presses a Settings
// button that the Palette only shows (palettehome.js).
export function runRow(row, { fill = false, offer = '', newTab = false, press = false } = {}) {
  // Off Home there is often no result banner, so the Palette stays open and shows the result itself.
  if (row.key === 'action:reread' && PAGE !== 'main' && dlg.shown()) {
    notePick(row.key);
    // The Palette stays open, so this pick is not one that the user left at once.
    lastFirst = null;
    return rereadInPalette();
  }
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
  say(res.ok ? `${row.label}: bookmarked ${scope === 'user' ? 'for you' : 'in this browser only'}.` : sentence(res.err) || 'Bookmark not saved.');
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

// Re-read with the Palette open: its note line shows the banner's words until the router answers.
let rereadShown = false;
function rereadInPalette() {
  if (!rereadNames()) return say('Not sent: the link to the server is down.');
  rereadShown = true;
}
on('result', (r) => {
  if (!rereadShown || !dlg.shown()) return;
  if (r.kind !== 'info') rereadShown = false;
  say([r.title, typeof r.text === 'string' ? r.text : ''].filter(Boolean).join('. '));
});

function runHere(row) {
  const id = row.target.id;
  if (id === 'reread') rereadNames();
  else if (id === 'shortcuts') emit('openShortcuts');
  else if (id === 'lock') emit('localLock');
  else if (id === 'signIn') emit('login');
  // The Settings Sign out button's own path (lock.js signOutNow).
  else if (id === 'signOut') emit('logout');
  else if (id === 'stepOut') stepOut();
  else if (id === 'theme') {
    const dark = isDark();
    flipTheme();
    showResult('ok', 'Theme changed', `The theme is ${dark ? 'light' : 'dark'} now.`);
  }
}

function pick(fill = false) {
  const it = items[cur];
  if (!it) return;
  if (it.type === 'hidden') return signInFromLine(it.act);
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

// Enter on the hidden matches line: Sign in (or Step in), then the Palette opens again with the same query.
let reopen = null;
const REOPEN_MS = 120_000;
function signInFromLine(act) {
  if (!act) return;
  const step = rows.find((r) => r.key === 'action:stepIn');
  reopen = { query: input.value, user: lockState().user || '', at: Date.now() };
  if (act === 'stepIn' && step) return runRow(step);
  close(false);
  emit('login');
}
on('access', () => {
  if (!reopen) return;
  const L = lockState();
  if (Date.now() - reopen.at > REOPEN_MS) reopen = null;
  // A sign-in that the user stopped: no user and no sign-in screen.
  else if (!L.user && !document.querySelector('.sode-lock.is-on')) reopen = null;
  else if (L.user && L.user !== reopen.user && !pageRefusal()) {
    const q = reopen.query;
    reopen = null;
    openPalette();
    input.value = q;
    draw();
  }
});

// Esc: the why line, then the search, then one level up, then close.
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
  const to = step ? stepIndex(items, cur, step) : keyMove(e, items, cur, pageRows());
  if (to !== null) {
    e.preventDefault();
    // The dialog's own Tab trap must not move the focus out of the box.
    if (e.key === 'Tab') e.stopPropagation();
    if (to >= 0) cur = to;
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
    if (emptyBackspace(path) === 'up' && !picked) {
      path = path.slice(0, -1);
      draw();
    }
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
  panelsHidden = r.ok && Number.isInteger(r.hidden) ? r.hidden : 0;
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

// pick(row): the Bookmarks editor's Add. Enter hands the row there and runs nothing. Grey rows do not show. cancel() runs when
// the Palette closes with no row.
export function openPalette({ from = null, pick = null, cancel = null } = {}) {
  if (dlg.shown()) return;
  stopWatch();
  undoFirstPick();
  origin = from;
  pickFn = pick;
  pickCancel = pick ? cancel : null;
  back = document.activeElement;
  path = [];
  input.value = '';
  picked = null;
  why.hidden = true;
  note.textContent = '';
  rereadShown = false;
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
  const picking = !!pickFn;
  pickFn = null;
  const to = origin || back;
  origin = back = null;
  const cancel = pickCancel;
  pickCancel = null;
  if (cancel) return cancel();
  // A picked row's action runs after this, so it can move the focus again: to a dialog or a field. A row picked for
  // the Bookmarks editor leaves the focus to the editor.
  if (toBox || !picking) giveBack(to);
}

// Focus goes back to where it was before the Palette opened, else to Source on Home. So after an action the next keys
// reach a box. A row can open a dialog or a confirm. That keeps its own focus while it is open. When it closes, the
// focus drops to body, and the watch puts it back. The watch stops after it puts the focus back, or at a pointer press.
// A key outside a dialog stops it too, since the person works on. The next open and WATCH_MS stop it too.
const WATCH_MS = 10 * 60_000;
let watch = 0;
let watchEnd = 0;
let backTo = null;
const shownNode = (n) => !!n?.isConnected && n.getClientRects().length > 0 && !n.closest('[hidden], [inert]');
const homeBox = () => document.getElementById(layout() === 'phone' ? 'm-src' : 'src');
// A dialog or the Lock that is open and has no focus: the focus waits for it to close.
const covered = () =>
  document.documentElement.hasAttribute('data-sode-lock') || [...document.querySelectorAll('[aria-modal="true"], .sode-lock.is-on')].some(shownNode);
function stopWatch() {
  clearInterval(watch);
  clearTimeout(watchEnd);
  watch = watchEnd = 0;
  backTo = null;
  document.removeEventListener('pointerdown', stopWatch, true);
  document.removeEventListener('keydown', keyOutside, true);
}
// A key that reaches body puts the focus back first, so the key reaches the box (a focus change in keydown moves the
// text too).
function keyOutside(e) {
  const t = e.target;
  if (t === document.body || t === document.documentElement) watchTick();
  else if (!(t instanceof Element) || !t.closest('[aria-modal="true"], .sode-lock, #palette')) stopWatch();
}
function watchTick() {
  const a = document.activeElement;
  if ((a && a !== document.body && a !== document.documentElement) || covered()) return;
  const to = shownNode(backTo) ? backTo : homeBox();
  stopWatch();
  if (shownNode(to)) to.focus({ preventScroll: true });
}
function giveBack(to) {
  stopWatch();
  backTo = to && to !== document.body ? to : null;
  if (!backTo && !homeBox()) return;
  if (shownNode(backTo)) backTo.focus({ preventScroll: true });
  watch = setInterval(watchTick, 150);
  watchEnd = setTimeout(stopWatch, WATCH_MS);
  document.addEventListener('pointerdown', stopWatch, true);
  document.addEventListener('keydown', keyOutside, true);
}
export const paletteOpen = () => dlg.shown();
export const closePalette = () => close(false);

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
