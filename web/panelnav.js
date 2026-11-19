// Pages are named by id, never by number, so a reorder in a live update keeps the current Page and every Go to Page target.
import { pageLabel } from './panelpages.js';

export const NAV_KINDS = ['goToPage', 'nextPage', 'previousPage'];

// On unless stored false. Governs only Next Page and Previous Page.
export const wrapOf = (panel) => panel?.settings?.wrapPages !== false;

const isOn = (p) => p.enabled !== false;

// The Page to show: want when it is there and enabled, else the first enabled Page. With every Page disabled it is
// Page 1, so the panel still has a grid.
export function landingPage(pages, want) {
  const hit = pages.find((p) => p.id === want);
  if (hit && isOn(hit)) return hit.id;
  return (pages.find(isOn) || pages[0])?.id ?? '';
}

export const pageIndex = (pages, id) =>
  Math.max(
    0,
    pages.findIndex((p) => p.id === id),
  );

// null when there is no other: past an end without wrap, or the only enabled Page.
export function stepPage(pages, id, dir, wrap) {
  const n = pages.length;
  const i = pageIndex(pages, id);
  for (let k = 1; k < n; k++) {
    let j = i + dir * k;
    if (j < 0 || j >= n) {
      if (!wrap) return null;
      j = (j + n) % n;
    }
    if (isOn(pages[j])) return pages[j].id;
  }
  return null;
}

// unassigned (no target), missing (its Page was deleted), disabled, empty (an enabled Page with no cells, which still
// works) or ok.
export function goToState(pages, cell) {
  if (typeof cell.page !== 'string' || !cell.page) return 'unassigned';
  const p = pages.find((x) => x.id === cell.page);
  if (!p) return 'missing';
  if (!isOn(p)) return 'disabled';
  return p.cells?.length ? 'ok' : 'empty';
}

export function navTarget(pages, current, cell, wrap) {
  if (cell.kind === 'nextPage') return stepPage(pages, current, 1, wrap);
  if (cell.kind === 'previousPage') return stepPage(pages, current, -1, wrap);
  const s = goToState(pages, cell);
  return s === 'ok' || s === 'empty' ? cell.page : null;
}

const WORDS = { goToPage: 'Go to Page', nextPage: 'Next Page', previousPage: 'Previous Page' };

export function navText(cell, pages) {
  if (cell.caption) return cell.caption;
  if (cell.kind !== 'goToPage') return WORDS[cell.kind];
  const i = pages.findIndex((p) => p.id === cell.page);
  return i < 0 ? WORDS.goToPage : pageLabel(pages[i], i);
}

// A Page with no id yet (an unsaved panel) cannot be a target.
export const targetChoices = (pages) =>
  pages.map((p, i) => {
    const text = p.name?.trim() ? `Page ${i + 1}, ${p.name.trim()}` : `Page ${i + 1}`;
    if (!p.id) return { value: '', text: `${text} (save the panel first)`, disabled: true };
    return { value: p.id, text: isOn(p) ? text : `${text} (disabled)`, disabled: false };
  });

// first is Page 1's cells as stored (the editor's storedGrid).
export function targetWarning(pages, cell, first) {
  const now = pages.map((p, i) => (i === 0 ? { ...p, cells: first } : p));
  const n = pages.findIndex((p) => p.id === cell.page) + 1;
  switch (goToState(now, cell)) {
    case 'missing':
      return 'Target Page deleted. Choose another Page.';
    case 'disabled':
      return `Page ${n} is disabled: the cell shows greyed.`;
    case 'empty':
      return `Page ${n} has no cells.`;
    default:
      return '';
  }
}
