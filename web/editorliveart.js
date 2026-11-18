// The Artwork panel in the Live Editor's list drawer (editorlive.js). It shows the sidebar's own Cell tab, moved in while
// open and back on close. So the artwork editor and its Icon B run are the editor's. It lies over the drawer, not
// over the grid, so the changed names stay in view.
import { el } from './core.js';
import { btn } from './formbits.js';

// tab() is the Cell tab, { node, openPicker } (editorside.js), or null before the sidebar is made. onClose runs after
// each close.
export function createLiveArt(tab, onClose = () => {}) {
  const body = el('div', 'sode-live__art-body');
  const done = btn('pb-btn--outline pb-btn--sm', 'Done', () => api.close());
  done.id = 'ed-live-art-done';
  const node = el(
    'section',
    'sode-live__art',
    el('div', 'sode-live__drawer-head', Object.assign(el('h3', 'sode-live__drawer-h', 'Artwork'), { id: 'ed-live-art-h' }), done),
    body,
  );
  node.id = 'ed-live-art';
  node.setAttribute('role', 'dialog');
  node.setAttribute('aria-labelledby', 'ed-live-art-h');
  node.hidden = true;
  // Esc closes this panel only; the drawer's own Esc (editorlive.js) skips a handled key.
  node.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || e.defaultPrevented) return;
    e.preventDefault();
    api.close();
  });

  let moved = null; // the Cell tab's children while they are here
  let back = null; // focus goes back here on close
  const api = {
    node,
    // False when there is no Cell tab to show.
    open() {
      const t = tab();
      if (!t) return false;
      if (!moved) {
        back = document.activeElement;
        moved = { home: t.node, kids: [...t.node.children] };
        body.append(...moved.kids);
      }
      node.hidden = false;
      t.openPicker();
      done.focus({ preventScroll: true });
      return true;
    },
    close() {
      if (!moved) return;
      moved.home.append(...moved.kids);
      moved = null;
      node.hidden = true;
      const to = back;
      back = null;
      onClose();
      if (to?.isConnected && !to.closest('[inert]')) to.focus({ preventScroll: true });
    },
  };
  return api;
}
