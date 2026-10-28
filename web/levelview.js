// Levels on screen and in boxes: labels, typed text, runs, colors, the bar
// and the picker buttons. Pure parsing is levels.js; this reads S.

import { el, S } from './core.js';
import { parseLevelsAs } from './levels.js';

export const allLevels = () => S.levels.map((l) => l.ui);

// Levels are shown and typed per cfg.levelNumbering: "1" (1-17), "0"
// (0-16) or "va" (V, 1-16: "0" with level 0 shown as V). Internally they
// stay 1..N.
// With no mode it's the server's label (Level.Label); a mode is for
// previewing one that isn't saved yet.
export function levelLabel(ui, mode) {
  if (mode === undefined) {
    const l = S.levelBy.get(ui);
    if (l) return l.label;
    mode = S.cfg.levelNumbering;
  }
  if (mode === '0') return String(ui - 1);
  if (mode === 'va') return ui === 1 ? 'V' : String(ui - 1);
  return String(ui);
}

// What a levels box means. Blank is the default (stored as plain 1-based
// levels); none: the picker was cleared to no levels, so blank means none.
export function parseLevels(text, none = false) {
  if (none && !text.trim()) return null;
  if (!text.trim()) return parseLevelsAs(S.cfg.defaultLevels, '1', S.levels.length);
  return parseLevelsAs(text, S.cfg.levelNumbering, S.levels.length);
}

// A levels box's text rewritten for a numbering that changed with the
// last hello, so it keeps meaning the same levels. Text that didn't parse
// before is left as it is.
export function relabelLevels(text) {
  const p = S.prevNumbering;
  if (!p || !text.trim() || p.mode === S.cfg.levelNumbering) return text;
  const list = parseLevelsAs(text, p.mode, p.max);
  return list ? levelInput(list) : text;
}

// The default levels as they'd be typed, for a box's placeholder.
export const defaultLevelsText = () => levelInput(parseLevels('') || []);
// What the default is called by a blank levels box: All when it's every
// level.
export const defaultLevelsWord = () => ((parseLevels('') || []).length === S.levels.length ? 'All' : 'default');
export const defaultLevelsHint = () => `${defaultLevelsText()} (${defaultLevelsWord()})`;

// Collapses a sorted level list into runs: [[1,3],[5,5]].
function levelRuns(list) {
  const out = [];
  for (let i = 0; i < list.length;) {
    let j = i;
    while (j + 1 < list.length && list[j + 1] === list[j] + 1) j++;
    out.push([list[i], list[j]]);
    i = j + 1;
  }
  return out;
}
// "1–3, 5" for display; "1-3,5" for a levels box. Labels follow mode.
const runsText = (list, dash, sep, mode) => levelRuns(list)
  .map(([a, b]) => (a === b ? levelLabel(a, mode) : `${levelLabel(a, mode)}${dash}${levelLabel(b, mode)}`)).join(sep);
export const levelText = (list) => runsText(list, '–', ', ');
export const levelInput = (list, mode) => runsText(list, '-', ',', mode);

// A configured #rrggbb is one color for both themes, so its number gets
// black or white, whichever contrasts more (WCAG relative luminance).
function inkFor(hex) {
  const lin = (i) => {
    const c = parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };
  const y = 0.2126 * lin(1) + 0.7152 * lin(3) + 0.0722 * lin(5);
  return (y + 0.05) / 0.05 > 1.05 / (y + 0.05) ? '#000' : '#fff';
}

// Sets --sode-c (and --sode-ink for a configured color) on a level's
// pick or slot.
export function levelColor(node, l) {
  if (l.color) {
    node.style.setProperty('--sode-c', l.color);
    node.style.setProperty('--sode-ink', inkFor(l.color));
  } else {
    node.style.setProperty('--sode-c', `var(--pb-signal-${l.swatch})`);
    node.style.removeProperty('--sode-ink');
  }
  return node;
}

// Every level in one fixed row: filled when in `on`, outlined otherwise,
// so rows line up like monospace columns. The numbers mean it never
// relies on color alone.
export function levelBar(on) {
  const set = new Set(on);
  const row = el('span', 'sode__bar', ...S.levels.map((l) => {
    const slot = el('span', 'sode__slot' + (set.has(l.ui) ? ' sode__slot--on' : ''), l.label);
    slot.dataset.hint = `Level ${l.label}`; // where the slot is too small for its label
    return levelColor(slot, l);
  }));
  // Two equal rows; an odd first level (0 or V) stands on its own to the left.
  const odd = S.levels.length % 2;
  if (odd) row.classList.add('sode__bar--odd');
  row.style.setProperty('--sode-bar-cols', Math.floor(S.levels.length / 2) + odd);
  const sorted = [...set].sort((a, b) => a - b);
  row.setAttribute('role', 'img');
  row.setAttribute('aria-label', sorted.length ? `Levels ${levelText(sorted)}` : 'No levels');
  return row;
}

// One numbered toggle per level, for a picker. onToggle(ui) on tap.
export function pickButtons(onToggle) {
  return S.levels.map((l) => {
    const b = el('button', 'sode__pick', l.label);
    b.type = 'button';
    b.tabIndex = -1;
    b.dataset.ui = l.ui;
    levelColor(b, l);
    b.setAttribute('aria-label', `Level ${l.label}`);
    b.addEventListener('click', () => onToggle(l.ui));
    return b;
  });
}
