// Presets popup (Ctrl+G / Cmd+G): every preset with search; Enter fires via presets.js.
// Pins here are per browser or in the user's profile; pinning for everyone is in the editor.

import { el, on, emit, icon, sentence } from './core.js';
import { chord, mac } from './keys.js';
import { trapDialog } from './widgets.js';
import { profileSaver } from './globalsave.js';
import { fire, fillBoxes, openPresets } from './presets.js';
import { allPresets, effectivePin, pinOverrides, setLocalPin, withUserPin, rankPresets, presetKey, movePin, pinnedPresets } from './presetstore.js';
import { scopeLabel } from './scopes.js';

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
const editorBtn = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode__head-link', 'Preset Editor');
editorBtn.type = 'button';
editorBtn.addEventListener('click', () => { close(); openPresets(); });
const done = el('button', 'pb-btn pb-btn--ok', 'Close');
done.type = 'button';
done.addEventListener('click', () => close());

dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'All Presets'), { id: 'preset-pop-title' }), editorBtn),
  el('div', 'pb-modal__body sode__filters-body', input, list, noteEl,
    el('span', 'sode__hint sode__pop-help', mac
      ? 'Navigate with ↑ ↓ or ⌘j/k, use ⌥↑ ⌥↓ or ⌥⌘j/k to reorder, ⌥p to pin, and ⇧Enter or ⇧click to fill the boxes without taking.'
      : 'Navigate with ↑ ↓ or Ctrl+j/k, use Alt+↑ Alt+↓ or Ctrl+Alt+j/k to reorder, Alt+p to pin, and Shift+Enter or Shift+click to fill the boxes without taking.')),
  el('div', 'pb-modal__footer', done));
root.append(dialog);
document.body.append(root);

let open = false, back = null, hi = 0, shownList = [];
const setNote = (t, error = false) => {
  noteEl.textContent = t;
  noteEl.classList.toggle('sode__error-text', error);
};
const scopeText = (s) => scopeLabel(s.scope).toLowerCase();
// Pins of the user's own (Me) presets save to their profile: no PIN row.
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
  // Pinned first in the strip's order, then the rest; search ranks within each.
  const ranked = rankPresets(allPresets(), input.value);
  const isPinned = (s) => effectivePin(s, ov);
  const searching = !!input.value.trim();
  const pins = searching ? ranked.filter(isPinned) : pinnedPresets();
  shownList = [...pins, ...ranked.filter((s) => !isPinned(s))];
  hi = Math.max(0, Math.min(hi, shownList.length - 1));
  const reorder = !searching && pins.length > 1;
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

function togglePin(s) {
  const pinned = effectivePin(s, pinOverrides());
  if (s.scope === 'user') profSaver.change({ presets: withUserPin(s.name, !pinned) }, () => {});
  else setLocalPin(s, !pinned);
}

// ---- reordering the pinned presets: drag a row, or Alt+↑/↓ on the highlighted one
let dragKey = '';
// The order shows at once; a failed save restores the real order and says why.
function reorder(saved) {
  saved?.then((r) => {
    if (r.ok) return;
    emit('presets'); // the strip and this list, from the order that really is
    setNote(sentence(r.err) || 'The order was not saved.', true);
  });
  return !!saved;
}
function dragRow(row, s) {
  const key = presetKey(s);
  row.draggable = true;
  row.addEventListener('dragstart', (e) => { dragKey = key; e.dataTransfer.effectAllowed = 'move'; e.dataTransfer.setData('text/plain', s.name); row.classList.add('is-dragging'); });
  row.addEventListener('dragend', () => { dragKey = ''; row.classList.remove('is-dragging'); for (const r of list.querySelectorAll('.is-over')) r.classList.remove('is-over'); });
  row.addEventListener('dragover', (e) => { if (dragKey && dragKey !== key) { e.preventDefault(); row.classList.add('is-over'); } });
  row.addEventListener('dragleave', () => row.classList.remove('is-over'));
  row.addEventListener('drop', (e) => { e.preventDefault(); row.classList.remove('is-over'); reorder(movePin(dragKey, { before: key })); dragKey = ''; });
}
function nudge(d) {
  const s = shownList[hi];
  if (!s || input.value.trim() || !effectivePin(s, pinOverrides())) return;
  hi += d; // pins are the top rows, so the row follows; set before the move redraws the list
  if (!reorder(movePin(presetKey(s), { by: d }))) hi -= d;
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
// A click on empty space must not steal focus from the search box; a draggable row is left alone on mousedown so its drag still starts.
dialog.addEventListener('mousedown', (e) => {
  if (e.target.closest('button, input, [draggable="true"]')) return;
  e.preventDefault();
  input.focus();
});
dialog.addEventListener('click', (e) => { if (!e.target.closest('button, input')) input.focus(); });
const NUDGE = { 'alt+up': -1, 'alt+down': 1, 'alt+meta+k': -1, 'alt+meta+j': 1, 'ctrl+alt+k': -1, 'ctrl+alt+j': 1 };
const MOVE = { down: 1, up: -1, 'ctrl+j': 1, 'ctrl+k': -1, 'meta+j': 1, 'meta+k': -1 };
dialog.addEventListener('keydown', (e) => {
  const onButton = !!e.target.closest?.('button');
  const c = chord(e);
  const slot = /^alt\+(shift\+)?([1-9])$/.exec(c);
  if (c in NUDGE) { e.preventDefault(); nudge(NUDGE[c]); return; }
  if (c === 'alt+p') { e.preventDefault(); if (!e.repeat && shownList[hi]) togglePin(shownList[hi]); return; }
  if (slot) { e.preventDefault(); if (!e.repeat) pressPreset(pinnedPresets()[+slot[2] - 1], !!slot[1]); return; }
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
  setNote('');
  draw();
  dlg.show();
  input.focus();
}

// Focus returns where it was; a confirm opened next remembers it.
function close() {
  if (!open) return;
  open = false;
  dlg.hide();
  profSaver.close();
  if (back?.isConnected) back.focus();
}

const dlg = trapDialog(root, dialog, () => {
  if (input.value) { input.value = ''; hi = 0; draw(); input.focus(); } else close();
});

on('presets', () => { if (open) draw(); });
on('presetPopup', openPopup);
