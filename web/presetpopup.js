// The Presets popup (Ctrl+G, or ⌘G on a Mac; a button on the phone): every
// preset, pinned or not, with a search box. Enter presses the highlighted
// one, which opens the usual confirm (presets.js `fire`). Each row can pin
// or unpin the preset for this browser; for everyone's presets there is also
// "Pin for everyone", which saves through the settings PIN like the editor.

import { el, S, on, icon } from './core.js';
import { trapDialog } from './widgets.js';
import { globalSaver } from './globalsave.js';
import { profileSaver } from './profilesaver.js';
import { fire, presetShown } from './presets.js';
import { shareRow, shareLink, sharePresetDoc } from './transfer.js';
import { allPresets, effectivePin, pinOverrides, setLocalPin, withGlobalPin, withUserPin, clearPinOverride, rankPresets, PRESET_SCOPES } from './presetstore.js';

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
    el('span', 'sode__hint', `↑/↓ or ${mac ? '⌘' : 'Ctrl+'}j/k to move, Enter to fire, Esc to close. Pinned presets are the buttons above Recent.`)),
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

const scopeText = (s) => PRESET_SCOPES[s.scope].toLowerCase();
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
  shownList = rankPresets(allPresets(), input.value);
  hi = Math.max(0, Math.min(hi, shownList.length - 1));
  const ov = pinOverrides();
  list.replaceChildren(...shownList.map((s, i) => {
    const pinned = effectivePin(s, ov);
    const main = el('button', 'sode__preset-main', el('strong', null, s.name),
      el('span', 'sode__hint', ` ${scopeText(s)}${pinned ? '' : ' · not pinned'}`));
    main.type = 'button';
    main.tabIndex = -1;
    main.addEventListener('click', () => pressPreset(s));
    const tools = [btn(pinned ? 'Unpin' : 'Pin', pinned ? 'pin-off' : 'pin',
      s.scope === 'global' ? 'Pin or unpin on this browser only' : 'Pin or unpin',
      () => (s.scope === 'user' ? profSaver.change({ presets: withUserPin(s.name, !pinned) }, () => {}) : setLocalPin(s, !pinned)))];
    tools.push(btn('Share', 'share-2', 'Copy a link to this preset', () => shareLink(sharePresetDoc(s), shareUI)));
    if (s.scope === 'global') {
      const flag = s.pinned !== false;
      tools.push(btn(flag ? 'Unpin for everyone' : 'Pin for everyone', 'users', 'Changes the preset for every browser (settings PIN)', () => {
        saver.change({ presets: withGlobalPin(s.name, !flag) }, () => clearPinOverride(s.name));
      }));
    }
    const row = el('div', `sode__preset-item${i === hi ? ' is-hi' : ''}`, main, el('span', 'sode__preset-tools', ...tools));
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

function move(d) {
  if (!shownList.length) return;
  hi = (hi + d + shownList.length) % shownList.length;
  draw();
}

function pressPreset(s) {
  if (!s) return;
  close();
  fire(s.scope, s.name);
}

input.addEventListener('input', () => { hi = 0; setNote(''); draw(); });
// A click on empty space inside the popup (#155, #156) must not take focus
// away from the search box; buttons and the box itself behave as usual.
dialog.addEventListener('mousedown', (e) => {
  if (e.target.closest('button, input')) return;
  e.preventDefault();
  input.focus();
});
dialog.addEventListener('keydown', (e) => {
  // A focused button keeps its own Enter/Space; anywhere else the keys drive the list.
  const onButton = !!e.target.closest?.('button');
  const jk = (e.ctrlKey !== e.metaKey) && !e.altKey && !e.shiftKey ? { j: 1, k: -1 }[e.key.toLowerCase()] : 0;
  if (e.key === 'ArrowDown' || jk === 1) { e.preventDefault(); move(1); }
  else if (e.key === 'ArrowUp' || jk === -1) { e.preventDefault(); move(-1); }
  else if (e.key === 'Enter' && !onButton && !e.ctrlKey && !e.metaKey && !e.altKey) { e.preventDefault(); pressPreset(shownList[hi]); }
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
  root.hidden = false;
  input.focus();
}

// Focus goes back where it was; a confirm opened next remembers that place.
function close() {
  if (!open) return;
  open = false;
  root.hidden = true;
  saver.close();
  profSaver.close();
  if (back?.isConnected) back.focus();
}

// Esc: the typed text first, then the popup.
trapDialog(root, dialog, () => {
  if (input.value) { input.value = ''; hi = 0; draw(); input.focus(); } else close();
});

on('presets', () => { if (open) draw(); });
on('presetPopup', openPopup);

// A modal, Settings or the lock screen over the page: not now.
const blocked = () =>
  document.documentElement.dataset.view === 'settings' ||
  !!document.querySelector('.pb-modal-backdrop:not([hidden]), .sode-lock.is-on, #m-sheet:not([hidden])');

// Ctrl+G on every OS, ⌘G on a Mac (the browser's find-next is stopped).
// Every check excludes the other modifiers.
document.addEventListener('keydown', (e) => {
  if (e.code !== 'KeyG' || e.shiftKey || e.altKey || e.repeat) return;
  if (!(e.ctrlKey !== e.metaKey) || (e.metaKey && !mac)) return;
  if (!S.cfg || !presetShown()) return;
  e.preventDefault();
  if (blocked()) return;
  openPopup();
});
