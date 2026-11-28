// Bookmarks in the page (GLOSSARY.md, Bookmark): the strip above History in both layouts ([data-presets]), what a
// bookmark does when picked, and the Alt+1-9 slots (presetkeys.js). The lists are bookmarkstore.js, the rules
// bookmarkrules.js and the editor bookmarkeditor.js.
//
// A bookmark opens its Palette row through the Palette's own runRow, so it has the same guards as the row. A preset
// asks for its Confirm, Clear History still confirms and Step in still asks for the PIN. A grey one shows its why in
// the Palette (explainRow), with Step in or Approval.

import { el, on, emit, icon, srcItem, dstItem } from './core.js';
import { keyLabel, ariaKeys } from './keys.js';
import { showResult } from './plans.js';
import { paletteRows, paletteRowsReady, runRow, explainRow, openUrl } from './palette.js';
import { shownBookmarks } from './bookmarkstore.js';
import { bookmarkView, canNewTab, STATE_TEXT } from './bookmarkrules.js';
import { allPresets, presetTip } from './presetstore.js';
import { statusBadge, presetStatus } from './brokenview.js';

// Until the catalogue has its settings and panels, a bookmark to one would read as Missing: none shows it yet.
let ready = false;
const rowsNow = () => (ready ? paletteRows() : null);

async function refreshRows() {
  await paletteRowsReady();
  ready = true;
  renderStrips();
}

// fill: Shift-click or Alt+Shift+1-9. Only a preset fills the Patch boxes; any other bookmark does nothing then.
export async function openBookmark(b, { fill = false } = {}) {
  if (b.url) return fill ? undefined : openUrl(b.url, b.newTab);
  const v = bookmarkView(b, ready ? paletteRows() : await paletteRowsReady());
  if (v.state === 'missing') return showResult('warn', `Bookmark ${b.label} is missing`, 'What it opens is gone. Edit Bookmarks to remove it.');
  if (fill) return v.row.kind === 'preset' && v.state === 'ok' ? runRow(v.row, { fill: true }) : undefined;
  if (v.state === 'noAccess') return explainRow(v.row);
  runRow(v.row, { newTab: b.newTab && canNewTab(b, v.row), press: true });
}

// Alt+1-9: the first nine of the shown list.
export function openSlot(i, fill = false) {
  const b = shownBookmarks().list[i];
  if (b) openBookmark(b, { fill });
}

const BADGE = {
  missing: ['pb-badge--warn', 'triangle-alert'],
  noAccess: ['', 'ban'],
  signIn: ['', 'lock'],
};

const presetOf = (row) => allPresets().find((x) => x.scope === row.target.scope && x.id === row.target.id) || null;

function tipOf(b, v) {
  if (b.url) return b.url;
  if (v.state === 'missing') return 'What this bookmark opens is gone';
  if (!v.row) return b.label;
  if (v.state === 'noAccess') return v.row.access.why;
  if (v.row.kind === 'preset') {
    const p = presetOf(v.row);
    if (p)
      return presetTip(
        p,
        (n) => srcItem(n).label,
        (n) => dstItem(n).label,
      );
  }
  return v.row.hint || v.row.label;
}

function chip(b, i, rows) {
  const v = bookmarkView(b, rows);
  const off = v.state === 'missing' || v.state === 'noAccess';
  const c = el('button', { type: 'button', class: `pb-chip sode__set${off ? ' sode__set--off' : ''}`, 'data-bookmark': i, 'data-state': v.state }, v.label);
  if (b.url && b.newTab) c.append(icon('external-link'));
  if (BADGE[v.state]) {
    const [cls, ic] = BADGE[v.state];
    c.append(el('span', `pb-badge ${cls} sode__status-badge`, icon(ic), STATE_TEXT[v.state]));
  }
  // A preset's Broken or Name moved status, as its chip showed it before bookmarks.
  const p = v.row?.kind === 'preset' && presetOf(v.row);
  const broken = p && statusBadge(presetStatus(p).status);
  if (broken) c.append(broken);
  // Alt+1-9 open the first nine: the key slides in over the chip while Option is held.
  if (i < 9) {
    const hint = el('span', { class: 'sode__chip-key', 'aria-hidden': 'true' }, el('kbd', 'sode__key-note', keyLabel(`alt+${i + 1}`)));
    c.append(hint);
    c.setAttribute('aria-keyshortcuts', ariaKeys(`alt+${i + 1}`));
  }
  // Names change (re-read, late load): the tip is worked out when the pointer or focus arrives.
  const tip = () => {
    c.dataset.pbTip = tipOf(b, bookmarkView(b, rowsNow()));
  };
  tip();
  c.addEventListener('pointerenter', tip);
  c.addEventListener('focus', tip);
  c.addEventListener('click', (e) => openBookmark(b, { fill: e.shiftKey }));
  return c;
}

export function renderStrips() {
  const { list } = shownBookmarks();
  const rows = rowsNow();
  for (const box of document.querySelectorAll('[data-presets]')) {
    const edit = el('button', { type: 'button', class: 'pb-btn pb-btn--ghost pb-btn--sm sode__chipbtn', 'aria-haspopup': 'dialog' }, 'Edit');
    edit.addEventListener('click', () => emit('bookmarkEditor'));
    // Edit and All presets are the same button, beside the heading, so the strip is one row with or without bookmarks.
    // All presets has the name of the Presets popup it opens: the popup lists presets, not bookmarks.
    const more = el(
      'button',
      { type: 'button', class: 'pb-btn pb-btn--ghost pb-btn--sm sode__chipbtn sode__presets-all', 'aria-haspopup': 'dialog' },
      icon('list'),
      'All presets',
    );
    if (document.documentElement.dataset.pbLayout !== 'phone') more.dataset.pbTip = `All presets (${keyLabel('mod+g')})`;
    more.addEventListener('click', () => emit('presetPopup'));
    const sep = el('span', { class: 'sode__sets-sep', 'aria-hidden': 'true' }, '|');
    const chips = list.map((b, i) => chip(b, i, rows));
    box.replaceChildren(
      el('span', 'sode__label', 'Bookmarks'),
      edit,
      more,
      el('div', 'sode__sets', ...(chips.length ? [sep] : []), el('div', 'sode__sets-run', ...chips)),
    );
  }
}

for (const ev of ['bookmarks', 'presets', 'filters', 'paletteRows', 'names']) on(ev, renderStrips);
// A sign-in changes Me, the Rights and the panels that the Palette rows reach.
on('access', refreshRows);
renderStrips();
