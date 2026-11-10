// Moving between a panel's Pages (GLOSSARY.md Go to Page, Next Page / Previous Page). Pure. Pages are named by id,
// never by number, so a reorder in a live update keeps the current Page and every Go to Page target.
import { pageLabel } from './panelpages.js';

export const NAV_KINDS = ['goToPage', 'nextPage', 'previousPage'];

// settings.wrapPages, on unless stored false. It governs only Next Page and Previous Page.
export const wrapOf = (panel) => panel?.settings?.wrapPages !== false;

const isOn = (p) => p.enabled !== false;

// The Page to show: want when it is there and enabled, else the first enabled Page. With every Page disabled it is
// Page 1, so the panel still has a grid.
export function landingPage(pages, want) {
  const hit = pages.find((p) => p.id === want);
  if (hit && isOn(hit)) return hit.id;
  return (pages.find(isOn) || pages[0])?.id ?? '';
}

// 0 for an id not in the list.
export const pageIndex = (pages, id) =>
  Math.max(
    0,
    pages.findIndex((p) => p.id === id),
  );

// The next enabled Page one way (dir 1 or -1), or null when there is none other: past an end without wrap, or the only
// enabled Page.
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

// The Page id a press goes to, or null when the cell is greyed and the press does nothing.
export function navTarget(pages, current, cell, wrap) {
  if (cell.kind === 'nextPage') return stepPage(pages, current, 1, wrap);
  if (cell.kind === 'previousPage') return stepPage(pages, current, -1, wrap);
  const s = goToState(pages, cell);
  return s === 'ok' || s === 'empty' ? cell.page : null;
}

const WORDS = { goToPage: 'Go to Page', nextPage: 'Next Page', previousPage: 'Previous Page' };

// A Go to Page cell's caption replaces its Page's name.
export function navText(cell, pages) {
  if (cell.caption) return cell.caption;
  if (cell.kind !== 'goToPage') return WORDS[cell.kind];
  const i = pages.findIndex((p) => p.id === cell.page);
  return i < 0 ? WORDS.goToPage : pageLabel(pages[i], i);
}

// The editor's Page list for a Go to Page cell, by number and name. A Page with no id yet (an unsaved panel) cannot be
// a target until the server gives it one.
export const targetChoices = (pages) =>
  pages.map((p, i) => {
    const text = p.name?.trim() ? `Page ${i + 1}, ${p.name.trim()}` : `Page ${i + 1}`;
    if (!p.id) return { value: '', text: `${text} (save the panel first)`, disabled: true };
    return { value: p.id, text: isOn(p) ? text : `${text} (disabled)`, disabled: false };
  });

// The Cell tab's warning for a Go to Page cell, '' when none. first is Page 1's cells as the editor holds them, since
// the Page list's copy is stale while Page 1 is the grid.
export function targetWarning(pages, cell, first) {
  const now = pages.map((p, i) => (i === 0 ? { ...p, cells: first } : p));
  const n = pages.findIndex((p) => p.id === cell.page) + 1;
  switch (goToState(now, cell)) {
    case 'missing':
      return 'The Page this cell went to was deleted, so the panel shows it greyed out. Choose another Page.';
    case 'disabled':
      return `Page ${n} is disabled, so the panel shows this cell greyed out.`;
    case 'empty':
      return `Page ${n} has no cells yet. The cell still works.`;
    default:
      return '';
  }
}
