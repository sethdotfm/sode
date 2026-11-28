// Presets popup (Ctrl+G / Cmd+G): every preset with search; Enter fires through presets.js. The Bookmarks strip and
// the Palette hold the presets a person uses most.

import { el, on, icon } from './core.js';
import { chord, listStep, keyLabel } from './keys.js';
import { modal } from './modal.js';
import { btn } from './formbits.js';
import { fire, fillBoxes, openPresets } from './presets.js';
import { allPresets, rankPresets } from './presetstore.js';
import { scopeLabel } from './scopes.js';
import { statusBadge, presetStatus } from './brokenview.js';
import { inboxUser } from './inbox.js';
import { sendCopy } from './sendcopy.js';

const input = el('input', {
  class: 'pb-input',
  type: 'search',
  placeholder: 'Search presets',
  autocomplete: 'off',
  spellcheck: false,
  'aria-label': 'Search presets',
  role: 'combobox',
  'aria-controls': 'preset-pop-list',
  'aria-expanded': 'true',
});
const list = el('div', { class: 'sode__preset-list', id: 'preset-pop-list', role: 'listbox', 'aria-label': 'Presets' });
const noteEl = el('span', { class: 'sode__hint', 'aria-live': 'polite' });
const editorBtn = btn('pb-btn--outline pb-btn--sm sode__head-link', 'Edit Presets', () => {
  close();
  openPresets();
});
const dlg = modal({
  titleId: 'preset-pop-title',
  title: 'All presets',
  cls: 'sode__presets-dlg sode__preset-pop',
  head: [editorBtn],
  body: el('div', 'pb-modal__body sode__filters-body', input, list, noteEl),
  foot: [btn('sode-btn--primary', 'Close', () => close())],
  onEsc() {
    if (input.value) {
      input.value = '';
      hi = 0;
      draw();
      input.focus();
    } else close();
  },
});
const { dialog } = dlg;

let open = false,
  back = null,
  hi = 0,
  shownList = [];
const setNote = (t, error = false) => {
  noteEl.textContent = t;
  noteEl.classList.toggle('sode__error-text', error);
};
const scopeText = (s) => scopeLabel(s.scope);

function rowBtn(text, ic, tip, fn) {
  const b = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', icon(ic), text);
  b.type = 'button';
  if (tip) b.dataset.pbTip = tip;
  b.addEventListener('click', (e) => {
    e.stopPropagation();
    fn();
  });
  return b;
}

function draw() {
  // A redraw replaces the buttons: keep focus on the same one.
  const f = document.activeElement;
  const at =
    list.contains(f) && f.closest('.sode__preset-item')
      ? [f.closest('.sode__preset-item').id, [...f.closest('.sode__preset-item').querySelectorAll('button')].indexOf(f)]
      : null;
  shownList = rankPresets(allPresets(), input.value);
  hi = Math.max(0, Math.min(hi, shownList.length - 1));
  list.replaceChildren(
    ...shownList.map((s, i) => {
      const main = el(
        'button',
        'sode__preset-main',
        el('strong', null, s.name),
        el('span', 'sode__hint', ` ${scopeText(s)}`),
        statusBadge(presetStatus(s).status) || '',
      );
      main.type = 'button';
      main.dataset.pbTip = `${keyLabel('shift')}: fill the patch boxes`;
      main.tabIndex = -1;
      main.addEventListener('click', (e) => pressPreset(s, e.shiftKey));
      const tools = inboxUser() ? [rowBtn('Send a copy', 'send', '', () => sendCopy({ presets: [s] }, s.name))] : [];
      const row = el('div', `sode__preset-item${i === hi ? ' is-hi' : ''}`, main, el('span', 'sode__preset-tools', ...tools));
      row.id = `preset-pop-${i}`;
      row.setAttribute('role', 'option');
      row.setAttribute('aria-selected', String(i === hi));
      return row;
    }),
  );
  if (!shownList.length) list.append(el('span', 'sode__hint', allPresets().length ? 'No preset matches.' : 'No presets'));
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

function pressPreset(s, fill = false) {
  if (!s) return;
  close();
  if (fill) fillBoxes(s.scope, s.name);
  else fire(s.scope, s.name);
}

input.addEventListener('input', () => {
  hi = 0;
  setNote('');
  draw();
});
// A click on empty space must not steal focus from the search box.
dialog.addEventListener('mousedown', (e) => {
  if (e.target.closest('button, input')) return;
  e.preventDefault();
  input.focus();
});
dialog.addEventListener('click', (e) => {
  if (!e.target.closest('button, input')) input.focus();
});
const MOVE = { down: 1, up: -1 };
dialog.addEventListener('keydown', (e) => {
  const onButton = !!e.target.closest?.('button');
  const c = chord(e);
  const step = MOVE[c] || listStep(e);
  if (step) {
    e.preventDefault();
    move(step);
  } else if ((c === 'enter' || c === 'shift+enter') && !onButton) {
    e.preventDefault();
    pressPreset(shownList[hi], e.shiftKey);
  } else return;
  if (!onButton) input.focus();
});

// at: {scope, id} highlights that preset (an Item link), so Enter fires it.
function openPopup(at = null) {
  if (open) return;
  back = document.activeElement;
  open = true;
  input.value = '';
  hi = 0;
  setNote('');
  draw();
  if (at) {
    hi = Math.max(
      0,
      shownList.findIndex((s) => s.scope === at.scope && s.id === at.id),
    );
    draw();
  }
  dlg.show();
  input.focus();
}

// Focus returns where it was; a confirm opened next remembers it.
function close() {
  if (!open) return;
  open = false;
  dlg.hide();
  if (back?.isConnected) back.focus();
}

on('presets', () => {
  if (open) draw();
});
on('presetPopup', openPopup);
on('locking', close);
