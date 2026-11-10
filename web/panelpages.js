// A panel's Pages (ADR 0007, GLOSSARY.md Page): pages [{ id, name?, enabled, cells }], in order, at most PAGE_MAX.
// A Page's number is its place in the list and its id never changes. The panel page and the editor show Page 1 for
// now, through pageCells. Pure. Limits keep in step with internal/app/panelpagelist.go.
export const PAGE_MAX = 16;
export const PAGE_NAME_MAX = 40;

// A panel saved before Pages has cells and no pages; it reads as one enabled Page, as the server loads it.
export function pagesOf(panel) {
  const pages = panel?.pages;
  if (Array.isArray(pages) && pages.length) return pages;
  return [{ id: '', enabled: true, cells: panel?.cells || [] }];
}

// The cells shown for a panel or a grid: its own cells when it has them (a grid, or a pageView), else Page i's.
export const pageCells = (panel, i = 0) => (Array.isArray(panel?.cells) ? panel.cells : pagesOf(panel)[i]?.cells || []);

// The panel with cells as a view onto Page i, for code that reads a panel's cells.
export const pageView = (panel, i = 0) => ({ ...panel, cells: pagesOf(panel)[i]?.cells || [] });

export const pageLabel = (page, i) => page?.name?.trim() || `Page ${i + 1}`;

// 8 hex digits, the shape the server makes for a Page sent with no id.
export function newPageId() {
  const b = new Uint8Array(4);
  crypto.getRandomValues(b);
  return [...b].map((x) => x.toString(16).padStart(2, '0')).join('');
}

const outside = (x, rows, cols) => x.r > rows || x.c > cols;

// Drops cells that start outside rows by cols and clips a span that only reaches outside, as resize in gridmodel.js and
// fitCells in internal/app/panelpagelist.go do.
export function fitCells(cells, rows, cols) {
  return (cells || [])
    .filter((x) => !outside(x, rows, cols))
    .map((x) => {
      const rs = x.rs > rows - x.r + 1 ? rows - x.r + 1 : x.rs;
      const cs = x.cs > cols - x.c + 1 ? cols - x.c + 1 : x.cs;
      return rs === x.rs && cs === x.cs ? x : { ...x, rs, cs };
    });
}

// How many cells on the Pages after the first a resize to rows by cols drops.
export const lostOnOtherPages = (pages, rows, cols) => pages.slice(1).reduce((n, p) => n + (p.cells || []).filter((x) => outside(x, rows, cols)).length, 0);

// The Page list as a save sends it: Page 1 holds cells (the edited grid, already stored shape) and every other Page is
// fitted to rows by cols, ids, names and enabled kept. A Page with no id gets one from the server.
export const storedPages = (pages, cells, rows, cols) => pages.map((p, i) => ({ ...p, cells: i === 0 ? cells : fitCells(p.cells, rows, cols) }));
