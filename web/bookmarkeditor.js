// The Bookmarks editor (the strip's Edit): one list at a time, This browser, Me or Everyone. The edits are a draft until
// Save. Order is by drag or Alt+arrows on a row. Add from the Palette… picks a row there; a web address is typed here.
// The count shows only near the cap, since a short list needs no number. Everyone's list needs a Trusted user or an Administrator, and the server checks that again.

import { el, S, on, icon, sentence } from './core.js';
import { chord, keyLabel } from './keys.js';
import { modal } from './modal.js';
import { btn } from './formbits.js';
import { labeled } from './widgets.js';
import { scopeLabel, scopeOptions } from './scopes.js';
import { profileName } from './profile.js';
import { lockState } from './lock.js';
import { openPalette, paletteRowsReady } from './palette.js';
import { bookmarksIn, saveBookmarks, shownBookmarks } from './bookmarkstore.js';
import {
  BOOKMARKS_MAX,
  LABEL_MAX,
  addBookmark,
  bookmarkProblem,
  urlProblem,
  bookmarkView,
  canEditEveryone,
  canNewTab,
  fromRow,
  fromUrl,
  moveBookmark,
  STATE_TEXT,
} from './bookmarkrules.js';

const scopeIn = el('select', { class: 'pb-select', id: 'bm-scope', 'aria-label': 'List' });
const COUNT_FROM = BOOKMARKS_MAX - 5;
const countEl = el('span', { class: 'sode__hint sode-bm__count', id: 'bm-count', hidden: true });
const listEl = el('ol', { class: 'sode-bm__list', id: 'bm-list', 'aria-label': 'Bookmarks' });
const noteEl = el('span', { class: 'sode__hint', 'aria-live': 'polite', id: 'bm-note' });
const urlIn = el('input', { class: 'pb-input', id: 'bm-url', type: 'url', placeholder: 'https://', autocomplete: 'off', spellcheck: false });
const urlName = el('input', { class: 'pb-input', id: 'bm-url-name', maxLength: LABEL_MAX, autocomplete: 'off' });
const addRowBtn = btn('pb-btn--outline', [icon('bookmark-plus'), 'Add from the Palette…'], () => addFromPalette(), 'bm-add-row');
const addUrlBtn = btn('pb-btn--outline', 'Add', () => addUrl(), 'bm-add-url');
const saveBtn = btn('pb-btn--commit', 'Save', () => save(), 'bm-save');
const cancelBtn = btn('pb-btn--outline', 'Cancel', () => close());
const roNote = el('p', { class: 'sode__hint', id: 'bm-readonly', hidden: true }, 'Only a Trusted user or an Administrator can change Everyone’s bookmarks.');
const fullNote = el('p', { class: 'sode__hint', id: 'bm-full', hidden: true }, 'This list is full. Remove a bookmark to add one.');
const addCtl = [addRowBtn, urlIn, urlName, addUrlBtn];
for (const c of addCtl) c.setAttribute('aria-describedby', 'bm-full');
const addBox = el(
  'div',
  'sode-bm__add',
  el('div', 'sode-bm__add-row', el('div', 'sode__fld sode-bm__pick', el('span', 'sode__label', 'From the Palette'), addRowBtn)),
  el('div', 'sode-bm__add-row', labeled('Web address', urlIn, 'sode-bm__url'), labeled('Name', urlName), addUrlBtn),
  fullNote,
);

const dlg = modal({
  titleId: 'bm-title',
  title: 'Edit Bookmarks',
  cls: 'sode__filters sode-bm',
  dialogId: 'bm-dialog',
  body: el(
    'div',
    'pb-modal__body sode__filters-body',
    el('div', 'sode-bm__add-row', labeled('List', el('span', 'pb-select-wrap', scopeIn), 'sode-bm__scope'), countEl),
    roNote,
    addBox,
    listEl,
  ),
  foot: [noteEl, cancelBtn, saveBtn],
  onEsc: () => close(),
});

let open = false;
let back = null;
let scope = 'local';
let draft = [];
let dirty = false;
let discard = false;
let rows = null;
let busy = false;

const editable = () => scope !== 'global' || canEditEveryone({ role: lockState().role, noUsers: !(S.users || []).length });

function setNote(t, error = false) {
  noteEl.textContent = t;
  noteEl.classList.toggle('sode__error-text', error);
}
function touch() {
  dirty = true;
  discard = false;
  setNote('');
  draw();
}

function rowNode(b, i) {
  const v = bookmarkView(b, rows);
  const ro = !editable();
  const name = b.url
    ? el('input', { class: 'pb-input sode-bm__name', value: b.label, maxLength: LABEL_MAX, 'aria-label': `Bookmark ${i + 1} name`, disabled: ro })
    : el('span', 'sode-bm__name', v.label);
  if (b.url)
    name.addEventListener('change', () => {
      draft[i] = { ...draft[i], label: name.value.trim() };
      touch();
    });
  const tick = (text, key, on) => {
    const input = el('input', { class: 'pb-check__input', type: 'checkbox', disabled: ro });
    input.checked = !!b[key];
    input.dataset.opt = key;
    input.addEventListener('change', () => {
      draft[i] = { ...draft[i], [key]: input.checked };
      touch();
    });
    // An empty cell keeps the columns lined up from row to row.
    return on ? el('label', 'pb-check', input, el('span', 'pb-check__label', text)) : el('span');
  };
  const state = v.state !== 'ok' ? el('span', 'pb-badge sode__status-badge', STATE_TEXT[v.state]) : null;
  const what = el('span', 'sode__hint sode-bm__what', b.url || v.row?.hint || (v.row ? '' : 'Palette row'));
  const tool = (ic, label, fn, dis) => {
    const x = el(
      'button',
      { type: 'button', class: 'pb-btn pb-btn--ghost pb-btn--sm pb-btn--icon', 'aria-label': label, 'data-pb-tip': label, disabled: ro || dis },
      icon(ic),
    );
    x.addEventListener('click', fn);
    return x;
  };
  const li = el(
    'li',
    { class: 'sode-bm__row', tabIndex: 0, 'data-i': i, draggable: !ro },
    el('span', 'sode-bm__grip', icon('grip-vertical')),
    el('span', 'sode__preset-key', i < 9 ? keyLabel(`alt+${i + 1}`) : ''),
    el('span', 'sode-bm__main', name, state, what),
    el('span', 'sode-bm__opts', tick('Open in a new tab', 'newTab', canNewTab(b, v.row)), tick('Show in Palette', 'inPalette', true)),
    el(
      'span',
      'sode-bm__tools',
      tool('arrow-up', `Move up (${keyLabel('alt+up')})`, () => move(i, { by: -1 }), i === 0),
      tool('arrow-down', `Move down (${keyLabel('alt+down')})`, () => move(i, { by: 1 }), i === draft.length - 1),
      tool('trash-2', 'Remove', () => {
        draft.splice(i, 1);
        touch();
      }),
    ),
  );
  li.addEventListener('dragstart', (e) => {
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', String(i));
    li.classList.add('is-dragging');
  });
  li.addEventListener('dragend', () => li.classList.remove('is-dragging'));
  li.addEventListener('dragover', (e) => {
    if (!ro) e.preventDefault();
  });
  li.addEventListener('drop', (e) => {
    e.preventDefault();
    move(+e.dataTransfer.getData('text/plain'), { before: i });
  });
  return li;
}

function move(i, where) {
  const next = moveBookmark(draft, i, where);
  if (!next) return;
  draft = next;
  touch();
  const to = 'by' in where ? i + where.by : where.before > i ? where.before - 1 : where.before;
  listEl.querySelector(`[data-i="${to}"]`)?.focus();
}

function draw() {
  const f = document.activeElement;
  const at = listEl.contains(f) ? f.closest('[data-i]')?.dataset.i : null;
  listEl.replaceChildren(...draft.map(rowNode));
  if (!draft.length) listEl.append(el('li', 'sode__hint sode-bm__empty', 'No bookmarks in this list.'));
  const full = draft.length >= BOOKMARKS_MAX;
  countEl.hidden = draft.length < COUNT_FROM;
  countEl.textContent = `${draft.length} of ${BOOKMARKS_MAX} bookmarks in this list`;
  const ro = !editable();
  roNote.hidden = !ro;
  addBox.hidden = ro;
  for (const c of addCtl) c.disabled = full;
  fullNote.hidden = !full;
  saveBtn.disabled = ro || busy;
  saveBtn.classList.toggle('sode-btn--primary', dirty && !ro);
  saveBtn.classList.toggle('pb-btn--commit', !dirty || ro);
  if (at != null) listEl.querySelector(`[data-i="${at}"]`)?.focus();
}

function load(sc) {
  scope = sc;
  draft = bookmarksIn(sc).map((b) => ({ ...b }));
  dirty = false;
  discard = false;
  setNote(sc === shownBookmarks().scope ? 'The strip shows this list.' : `The strip shows ${scopeLabel(shownBookmarks().scope)}’s list, not this one.`);
  draw();
}

function add(b) {
  const r = addBookmark(draft, b);
  if (r.err) return setNote(sentence(r.err), true);
  draft = r.list;
  touch();
}

function addFromPalette() {
  dlg.hide();
  const back = () => {
    if (!open) return;
    dlg.show();
    addRowBtn.focus();
  };
  openPalette({
    pick: (row) => {
      back();
      add(fromRow(row));
    },
    cancel: back,
  });
}

function addUrl() {
  const b = fromUrl(urlIn.value, urlName.value);
  const why = urlProblem(b.url) || bookmarkProblem(b);
  if (why) {
    urlIn.classList.add('pb-input--invalid');
    setNote(sentence(why), true);
    return urlIn.focus();
  }
  urlIn.classList.remove('pb-input--invalid');
  urlIn.value = '';
  urlName.value = '';
  add(b);
}

async function save() {
  if (!editable() || busy) return;
  for (const b of draft) {
    const why = bookmarkProblem(b);
    if (why) return setNote(`${b.label || 'A bookmark'}: ${sentence(why)}`, true);
  }
  busy = true;
  setNote('Saving…');
  draw();
  const r = await saveBookmarks(scope, draft);
  busy = false;
  if (!r.ok) {
    draw();
    return setNote(sentence(r.err) || 'Bookmarks not saved.', true);
  }
  dirty = false;
  close();
}

export function openBookmarkEditor() {
  back = document.activeElement;
  open = true;
  scopeIn.replaceChildren(...scopeOptions(!!profileName()).map(([id, label]) => new Option(label, id)));
  scopeIn.value = shownBookmarks().scope;
  if (scopeIn.value !== shownBookmarks().scope) scopeIn.value = 'local';
  load(scopeIn.value);
  dlg.show();
  dlg.dialog.focus();
  paletteRowsReady().then((r) => {
    rows = r;
    if (open) draw();
  });
}

function close() {
  if (open && dirty && !discard) {
    discard = true;
    return setNote('Unsaved changes. Close again to discard them, or Save.', true);
  }
  open = false;
  dlg.hide();
  if (back?.isConnected) back.focus();
}

scopeIn.addEventListener('change', () => {
  if (dirty && !discard) {
    const was = scope;
    discard = true;
    scopeIn.value = was;
    return setNote('Unsaved changes. Pick the list again to discard them, or Save.', true);
  }
  load(scopeIn.value);
});
urlIn.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    e.preventDefault();
    addUrl();
  }
});
listEl.addEventListener('keydown', (e) => {
  const li = e.target.closest?.('[data-i]');
  if (!li || e.target !== li) return;
  const by = { 'alt+up': -1, 'alt+down': 1 }[chord(e)];
  if (!by) return;
  e.preventDefault();
  move(+li.dataset.i, { by });
});

on('bookmarkEditor', openBookmarkEditor);
on('bookmarks', () => {
  if (open && !dirty) load(scope);
});
// The Palette does not open over unsaved edits here.
on('unsavedAsk', (a) => {
  if (open && dirty) a.unsaved = true;
});
on('locking', () => {
  if (!open) return;
  discard = true;
  close();
});
