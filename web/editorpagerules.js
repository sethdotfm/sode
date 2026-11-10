// The editor's Pages bar rules: Add page, Move up and down, rename, enable, Delete page, Copy to and which Pages are
// open. Pure; editorpagebar.js draws them and paneleditor.js holds the list. None of these is an undo step except
// Copy to (docs/panels-design.md). Pages are named by id, which never changes, so Go to Page cells keep their target.
import { PAGE_MAX, PAGE_NAME_MAX, newPageId } from './panelpages.js';

// Pages open side by side at most.
export const OPEN_MAX = 4;

export const addPage = (pages, id = newPageId()) => (pages.length >= PAGE_MAX ? pages : [...pages, { id, enabled: true, cells: [] }]);

// to out of range (Move up on the first Page, Move down on the last) changes nothing.
export function movePage(pages, from, to) {
  if (to < 0 || to >= pages.length || from === to) return pages;
  const out = [...pages];
  const [p] = out.splice(from, 1);
  out.splice(to, 0, p);
  return out;
}

export function renamePage(pages, i, text) {
  const name = String(text ?? '')
    .trim()
    .slice(0, PAGE_NAME_MAX);
  return pages.map((p, j) => {
    if (j !== i) return p;
    const { name: _, ...rest } = p;
    return name ? { ...rest, name } : rest;
  });
}

export const enablePage = (pages, i, on) => pages.map((p, j) => (j === i ? { ...p, enabled: !!on } : p));

// A panel keeps at least one Page. Go to Page cells to a deleted Page stay as they are and read as missing (panelnav.js).
export const deletePage = (pages, i) => (pages.length <= 1 ? pages : pages.filter((_, j) => j !== i));

// How many Go to Page cells, on any Page, lead to the Page with this id.
export const leadsTo = (pages, id) => pages.reduce((n, p) => n + (p.cells || []).filter((c) => c.kind === 'goToPage' && c.page === id).length, 0);

// Page from's cells onto Page to, which keeps its id, name and enabled. replaced counts the cells it had; null for a
// Page onto itself.
export function copyPage(pages, from, to) {
  if (from === to || !pages[from] || !pages[to]) return null;
  const cells = structuredClone(pages[from].cells || []);
  return { pages: pages.map((p, j) => (j === to ? { ...p, cells } : p)), replaced: (pages[to].cells || []).length };
}

// A plain click opens one Page. Cmd or Ctrl-click adds a Page or takes one away, up to OPEN_MAX and never the last.
export function openClick(open, id, multi) {
  if (!multi) return [id];
  if (open.includes(id)) return open.length > 1 ? open.filter((x) => x !== id) : open;
  return open.length >= OPEN_MAX ? open : [...open, id];
}
