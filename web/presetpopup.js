// The Presets popup (Ctrl+G, or ⌘G on a Mac; a button on the phone): every
// preset, pinned or not, with a search box. Enter presses the highlighted
// one, which opens the usual confirm (presets.js `fire`). Each row can pin
// or unpin the preset for this browser; for everyone's presets there is also
// "Pin for everyone", which saves through the settings PIN like the editor.

import { el, on, icon } from './core.js';
import { chord } from './keys.js';
import { trapDialog } from './widgets.js';
import { globalSaver, profileSaver } from './globalsave.js';
import { fire, fillBoxes } from './presets.js';
import { shareRow, shareLink, sharePresetDoc } from './transfer.js';
import { allPresets, effectivePin, pinOverrides, setLocalPin, withGlobalPin, withUserPin, clearPinOverride, rankPresets, presetKey, setPinOrder, moveKey, pinnedPresets } from './presetstore.js';
import { scopeLabel } from './scopes.js';

const mac = /Mac|iPhone|iPad/.test(navigator.platform);

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal sode__presets-dlg sode__preset-pop');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'preset-pop-title');
dialog.tabIndex = -1;

const input = el('input', 'pb-input');
input.type = 'search';
input.placeholder = 'Search presets';
input.autocomplete = 'off';
input.spellcheck = false;
input.setAttribute('aria-label', 'Search presets');
input.setAttribute('role', 'combobox');
input.setAttribute('aria-controls', 'preset-pop-list');
input.setAttribute('aria-expanded', 'true');
const list = el('div', 'sode__preset-list');
list.id = 'preset-pop-list';
list.setAttribute('role', 'listbox');
list.setAttribute('aria-label', 'Presets');
const noteEl = el('span', 'sode__hint');
noteEl.setAttribute('aria-live', 'polite');
const shareUI = shareRow();
const pinIn = el('input', 'pb-input');
pinIn.type = 'password';
pinIn.placeholder = 'PIN';
pinIn.autocomplete = 'current-password';
pinIn.setAttribute('aria-label', 'Settings PIN');
const pinOk = el('button', 'pb-btn pb-btn--ok pb-btn--sm', 'Unlock');
pinOk.type = 'button';
const pinCancel = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Cancel');
pinCancel.type = 'button';
const pinRow = el('div', 'sode__filters-set', pinIn, pinOk, pinCancel);
pinRow.hidden = true;
const done = el('button', 'pb-btn pb-btn--ok', 'Close');
done.type = 'button';
done.addEventListener('click', () => close());

dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Presets'), { id: 'preset-pop-title' })),
  el('div', 'pb-modal__body sode__filters-body', input, list, pinRow, noteEl, shareUI.box,
    el('span', 'sode__hint', mac ? '↑ ↓ or ⌘J/K to move · ⌥↑ ⌥↓ or ⌥⌘J/K to reorder · ⌥P to pin or unpin · ⇧Enter or ⇧click to fill the boxes' : '↑ ↓ or Ctrl+J/K to move · Alt+↑ Alt+↓ or Ctrl+Alt+J/K to reorder · Alt+P to pin or unpin · Shift+Enter or Shift+click to fill the boxes')),
  el('div', 'pb-modal__footer', done));
root.append(dialog);
document.body.append(root);

let open = false, back = null, hi = 0, shownList = [];
const setNote = (t, error = false) => {
  noteEl.textContent = t;
  noteEl.classList.toggle('sode__error-text', error);
};
const saver = globalSaver({
  t: 'presets', resultT: 'presetsResult', pinIn, pinRow, pinOk, setNote,
  isOpen: () => open, failTitle: 'Presets not saved', okNote: 'Saved for everyone.',
  render: () => draw(),
});
pinCancel.addEventListener('click', () => saver.cancel());

const scopeText = (s) => scopeLabel(s.scope).toLowerCase();
// Me presets save to the user's profile: no PIN row.
const profSaver = profileSaver({
  setNote, isOpen: () => open, failTitle: 'Presets not saved', okNote: 'Saved.',
  render: () => draw(),
});

function btn(text, ic, tip, fn) {
  const b = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', icon(ic), text);
  b.type = 'button';
  b.title = tip;
  b.addEventListener('click', (e) => { e.stopPropagation(); fn(); });
  return b;
}

function draw() {
  // A redraw replaces the buttons: keep focus on the same one.
  const f = document.activeElement;
  const at = list.contains(f) && f.closest('.sode__preset-item')
    ? [f.closest('.sode__preset-item').id, [...f.closest('.sode__preset-item').querySelectorAll('button')].indexOf(f)] : null;
  const ov = pinOverrides();
  // Pinned presets first, in the strip's order (the saved one), then the rest;
  // a search keeps its ranking inside each part.
  const ranked = rankPresets(allPresets(), input.value);
  const isPinned = (s) => effectivePin(s, ov);
  const searching = !!input.value.trim();
  const pins = searching ? ranked.filter(isPinned) : pinnedPresets();
  shownList = [...pins, ...ranked.filter((s) => !isPinned(s))];
  hi = Math.max(0, Math.min(hi, shownList.length - 1));
  const reorder = !searching && pins.length > 1;
  // Option/Alt+1-9 fire the first nine pinned presets, in the strip's order.
  const keySlot = new Map(pinnedPresets().map((s, i) => [presetKey(s), i]));
  list.replaceChildren(...shownList.map((s, i) => {
    const pinned = isPinned(s);
    const main = el('button', 'sode__preset-main', el('strong', null, s.name),
      el('span', 'sode__hint', ` ${scopeText(s)}${pinned ? '' : ' · not pinned'}`));
    main.type = 'button';
    main.tabIndex = -1;
    main.addEventListener('click', (e) => pressPreset(s, e.shiftKey));
    const tools = [btn(pinned ? 'Unpin' : 'Pin', pinned ? 'pin-off' : 'pin',
      `${s.scope === 'global' ? 'Pin or unpin on this browser only' : 'Pin or unpin'} (${mac ? '⌥' : 'Alt+'}P)`,
      () => togglePin(s))];
    tools.push(btn('Share', 'share-2', 'Show a link to share this preset', () => shareLink(sharePresetDoc(s), shareUI)));
    if (s.scope === 'global') {
      const flag = s.pinned !== false;
      tools.push(btn(flag ? 'Unpin for everyone' : 'Pin for everyone', 'users', 'Changes the preset for every browser (settings PIN)', () => {
        saver.change({ presets: withGlobalPin(s.name, !flag) }, () => clearPinOverride(s.name));
      }));
    }
    const slot = keySlot.get(presetKey(s));
    const keyCell = el('span', 'sode__preset-key', slot < 9 ? `${mac ? '⌥' : 'Alt+'}${slot + 1}` : '');
    const grip = pinned && reorder ? el('span', 'sode__preset-grip', icon('grip-vertical')) : null;
    if (grip) grip.dataset.pbTip = 'Drag to reorder (Alt+↑/↓)';
    const row = el('div', `sode__preset-item${i === hi ? ' is-hi' : ''}`, keyCell, ...(grip ? [grip] : []), main, el('span', 'sode__preset-tools', ...tools));
    if (grip) dragRow(row, s);
    row.id = `preset-pop-${i}`;
    row.setAttribute('role', 'option');
    row.setAttribute('aria-selected', String(i === hi));
    return row;
  }));
  if (!shownList.length) list.append(el('span', 'sode__hint', allPresets().length ? 'No preset matches.' : 'No presets yet. Make one with Edit above Recent.'));
  if (shownList.length) input.setAttribute('aria-activedescendant', `preset-pop-${hi}`);
  else input.removeAttribute('aria-activedescendant');
  list.querySelector('.is-hi')?.scrollIntoView({ block: 'nearest' });
  if (at) (document.getElementById(at[0])?.querySelectorAll('button')[at[1]] || input).focus();
}

// Pins or unpins s: a preset of the user's own in their profile, any other for this browser.
function togglePin(s) {
  const pinned = effectivePin(s, pinOverrides());
  if (s.scope === 'user') profSaver.change({ presets: withUserPin(s.name, !pinned) }, () => {});
  else setLocalPin(s, !pinned);
}

// ---- reordering the pinned presets: drag a row, or Alt+↑/↓ on the highlighted one
let dragKey = '';
const pinKeys = () => pinnedPresets().map(presetKey);
// Dropped on another pinned row: it goes before that one.
function dropAt(from, to) {
  if (!from || from === to) return;
  setPinOrder(moveKey(pinKeys(), from, pinKeys().filter((k) => k !== from).indexOf(to)));
}
function dragRow(row, s) {
  const key = presetKey(s);
  row.draggable = true;
  row.addEventListener('dragstart', (e) => { dragKey = key; e.dataTransfer.effectAllowed = 'move'; e.dataTransfer.setData('text/plain', s.name); row.classList.add('is-dragging'); });
  row.addEventListener('dragend', () => { dragKey = ''; row.classList.remove('is-dragging'); for (const r of list.querySelectorAll('.is-over')) r.classList.remove('is-over'); });
  row.addEventListener('dragover', (e) => { if (dragKey && dragKey !== key) { e.preventDefault(); row.classList.add('is-over'); } });
  row.addEventListener('dragleave', () => row.classList.remove('is-over'));
  row.addEventListener('drop', (e) => { e.preventDefault(); row.classList.remove('is-over'); dropAt(dragKey, key); dragKey = ''; });
}
// Moves the highlighted pinned preset one place up or down among the pins.
function nudge(d) {
  const s = shownList[hi];
  if (!s || input.value.trim() || !effectivePin(s, pinOverrides())) return;
  const keys = pinKeys();
  const i = keys.indexOf(presetKey(s));
  if (i < 0 || i + d < 0 || i + d >= keys.length) return;
  hi = i + d; // pins are the top rows, so the row index follows
  setPinOrder(moveKey(keys, keys[i], i + d));
}

function move(d) {
  if (!shownList.length) return;
  hi = (hi + d + shownList.length) % shownList.length;
  draw();
}

function pressPreset(s, fill = false) {
  if (!s) return;
  close();
  if (fill) fillBoxes(s.scope, s.name);
  else fire(s.scope, s.name);
}

input.addEventListener('input', () => { hi = 0; setNote(''); draw(); });
// A click on empty space inside the popup (#155, #156) must not take focus
// away from the search box; buttons and the box itself behave as usual.
// A draggable row (a pinned preset) is left alone on mousedown: cancelling it
// would also cancel a drag that starts on its icon or key.
dialog.addEventListener('mousedown', (e) => {
  if (e.target.closest('button, input, [draggable="true"]')) return;
  e.preventDefault();
  input.focus();
});
dialog.addEventListener('click', (e) => { if (!e.target.closest('button, input')) input.focus(); });
// Keys while the popup is open, by chord. A focused button keeps its own
// Enter/Space; anywhere else the keys drive the list.
const NUDGE = { 'alt+up': -1, 'alt+down': 1, 'alt+meta+k': -1, 'alt+meta+j': 1, 'ctrl+alt+k': -1, 'ctrl+alt+j': 1 };
const MOVE = { down: 1, up: -1, 'ctrl+j': 1, 'ctrl+k': -1, 'meta+j': 1, 'meta+k': -1 };
dialog.addEventListener('keydown', (e) => {
  const onButton = !!e.target.closest?.('button');
  const c = chord(e);
  const slot = /^alt\+([1-9])$/.exec(c);
  if (c in NUDGE) { e.preventDefault(); nudge(NUDGE[c]); return; }
  if (c === 'alt+p') { e.preventDefault(); if (!e.repeat && shownList[hi]) togglePin(shownList[hi]); return; }
  if (slot) { e.preventDefault(); if (!e.repeat) pressPreset(pinnedPresets()[+slot[1] - 1]); return; }
  if (c in MOVE) { e.preventDefault(); move(MOVE[c]); }
  else if ((c === 'enter' || c === 'shift+enter') && !onButton) { e.preventDefault(); pressPreset(shownList[hi], e.shiftKey); }
  else return;
  if (!onButton) input.focus();
});

export function openPopup() {
  if (open) return;
  back = document.activeElement;
  open = true;
  input.value = '';
  hi = 0;
  pinRow.hidden = true;
  shareUI.box.hidden = true;
  setNote('');
  draw();
  dlg.show();
  input.focus();
}

// Focus goes back where it was; a confirm opened next remembers that place.
function close() {
  if (!open) return;
  open = false;
  dlg.hide();
  saver.close();
  profSaver.close();
  if (back?.isConnected) back.focus();
}

// Esc: the typed text first, then the popup.
const dlg = trapDialog(root, dialog, () => {
  if (input.value) { input.value = ''; hi = 0; draw(); input.focus(); } else close();
});

on('presets', () => { if (open) draw(); });
on('presetPopup', openPopup);
