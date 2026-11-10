// The open Pages side by side, up to four. #ed-grid always draws the focused Page, so Find, the grip, the menu and
// the paste reader keep working on one node; the other open Pages draw in a fixed pool of grids made once (each
// createEditorGrid adds document listeners). CSS order puts every frame at its Page's place, so a focus change
// moves no node.
import { el } from './core.js';
import { createEditorGrid } from './editorview.js';
import { createGrid, setAside } from './gridmodel.js';
import { pageLabel } from './panelpages.js';
import { OPEN_MAX } from './editorpagerules.js';

// frame holds #ed-grid and title its heading. on.focus(id, r, c) when a square of another open Page is pressed, and
// on.key(id, event) for a key while one of those grids has focus.
export function createPageView(frame, title, on) {
  const host = frame.parentElement;
  const pool = Array.from({ length: OPEN_MAX - 1 }, () => {
    const grid = el('div', 'sode-ed__grid');
    const head = el('h2', 'sode-ed__frame-title');
    const node = el('section', 'sode-ed__frame', head, grid);
    node.hidden = true;
    host.append(node);
    const slot = { node, head, grid, id: null };
    slot.view = createEditorGrid(grid, {
      // A drag that began here carries on over #ed-grid once it holds this Page, so only the press counts.
      select: (r, c, extend) => !extend && slot.id !== null && on.focus(slot.id, r, c),
      key: (e) => slot.id !== null && on.key(slot.id, e),
    });
    return slot;
  });

  const heading = (pages, id) => {
    const i = pages.findIndex((p) => p.id === id);
    const off = pages[i]?.enabled === false ? ' (disabled)' : '';
    return `Page ${i + 1}${pages[i]?.name?.trim() ? `, ${pageLabel(pages[i], i)}` : ''}${off}`;
  };

  return {
    // pages as stored (the focused one included); open is the open ids, focus the focused id. look is the rest of
    // what the editor grid's paint takes, plus rows, cols, names and fixed (Fixed destination on).
    paint(pages, open, focus, { rows, cols, names, fixed, ...look }) {
      const shown = pages.filter((p) => open.includes(p.id));
      const order = (id) => String(shown.findIndex((p) => p.id === id));
      host.dataset.open = String(shown.length);
      frame.style.order = order(focus);
      title.textContent = heading(pages, focus);
      frame.classList.toggle('is-focus', shown.length > 1);
      const rest = shown.filter((p) => p.id !== focus);
      pool.forEach((slot, i) => {
        const p = rest[i];
        slot.id = p ? p.id : null;
        slot.node.hidden = !p;
        if (!p) return slot.grid.replaceChildren();
        slot.node.style.order = order(p.id);
        slot.head.textContent = heading(pages, p.id);
        slot.grid.setAttribute('aria-label', `${heading(pages, p.id)} grid`);
        const g = createGrid({ rows, cols, cells: p.cells });
        slot.view.paint(fixed ? setAside(g, 'destination').grid : g, names, { ...look, pages });
      });
    },
  };
}
