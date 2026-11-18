// The Live Editor: the saved panel runs in place of the grid and sidebar (panelrun.js, embedded), over
// the editor's own session. It is not an instance of the panel, so it sends no keepalive.
// A Grid panel gets a bar over it that marks it as a preview. Its drawer holds the editor's own Sources and
// Destinations lists (editorgridlist.js), moved in for the run and back after it. A list's Artwork button shows the
// Cell tab's artwork over it (editorliveart.js). Their edits show at once. Names picked on the grid are that list's
// selection. Its selection bar moves into the bar while the drawer is closed, but the bar keeps it hidden for now
// (docs/agents/panels.md).
import { el, icon } from './core.js';
import { sessionFetch } from './sessionkey.js';
import { runPanel } from './panelrun.js';
import { liveUi } from './panelmount.js';
import { makeTabs } from './tabs.js';
import { btn } from './formbits.js';
import { createLiveArt } from './editorliveart.js';

// host holds the live panel, body is the editor it replaces, button is the toolbar's Live Editor button. ed is
// { save, dirty, readOnly, edges, lists, cellTab, edgeLists }. edges() is { sources, destinations, rev } as the editor
// has them. lists() gives the two list columns, cellTab() the sidebar's Cell tab and edgeLists() the lists' own apis.
export function createLive(host, body, button, ed) {
  const label = button.textContent;
  let run = null;
  let grid = null; // the bar and drawer of a Grid panel's run
  const api = {
    on: () => !!run,
    // page is the Page being edited, where the run opens.
    start(id, page = '', isGrid = false) {
      api.stop();
      grid = isGrid ? gridParts(ed) : null;
      const ui = liveUi(host, () => api.stop(), grid);
      host.hidden = false;
      body.hidden = true;
      button.textContent = 'Back to editing';
      run = runPanel(id, { ui, call: sessionFetch, page });
      grid?.started(run);
      ui.back.focus();
    },
    stop() {
      if (!run) return;
      run.stop();
      run = null;
      grid?.stop();
      grid = null;
      host.replaceChildren();
      host.hidden = true;
      body.hidden = false;
      button.textContent = label;
    },
    // The editor's fields or save state changed.
    changed: () => grid?.changed(),
    // A list's Artwork button: true when the open drawer shows the artwork.
    openArt: () => !!grid?.openArt(),
  };
  return api;
}

function gridParts(ed) {
  const [srcs, dsts] = ed.lists();
  const home = srcs.parentNode;
  const ro = ed.readOnly();

  const state = el('span', 'sode-live__state');
  state.id = 'ed-live-state';
  state.setAttribute('role', 'status');
  const save = btn('pb-btn--ok pb-btn--sm', 'Save', async () => {
    save.disabled = true;
    const err = await ed.save();
    save.disabled = false;
    paint(err);
  });
  save.id = 'ed-live-save';
  // A toggle: pressed while the drawer is open (setOpen). Back to editing goes to its left (panelmount.js).
  const open = btn('pb-btn--lg', '', () => setOpen(!opened));
  open.id = 'ed-live-lists';
  let openIcon = icon('panel-right-open');
  open.append(openIcon, el('span', null, 'Edit lists'));
  open.setAttribute('aria-controls', 'ed-live-drawer');
  open.hidden = ro;
  // The selection bar of the list whose names are picked on the grid. Hidden on the owner's request; the drawer keeps
  // its own copy of the actions.
  const picked = el('div', 'sode-live__picked');
  picked.id = 'ed-live-picked';
  picked.hidden = true;
  const title = el('h2', 'sode-live__title', 'Panel Live Editor');
  title.id = 'ed-live-title';
  const bar = el(
    'div',
    'pb-banner pb-banner--info sode-live__bar',
    title,
    el('span', 'pb-badge pb-badge--info', 'Preview'),
    picked,
    el('span', 'sode-panels__spacer'),
    state,
    save,
    open,
  );
  bar.id = 'ed-live-bar';
  bar.setAttribute('role', 'toolbar');
  bar.setAttribute('aria-labelledby', title.id);

  const close = btn('pb-btn--ghost pb-btn--icon pb-btn--sm', '', () => {
    setOpen(false);
    open.focus();
  });
  close.id = 'ed-live-close';
  close.append(icon('x'));
  close.setAttribute('aria-label', 'Close the lists');
  close.dataset.pbTip = 'Close the lists';
  const holder = (col) => el('div', 'sode-live__tab', col);
  const tabs = makeTabs([
    { id: 'ed-live-srcs', text: 'Sources', panel: holder(srcs) },
    { id: 'ed-live-dsts', text: 'Destinations', panel: holder(dsts) },
  ]);
  const panes = [srcs.parentNode, dsts.parentNode];
  const aside = el(
    'aside',
    'pb-drawer pb-drawer--right pb-drawer--wide sode-live__drawer',
    el('div', 'pb-drawer__body', el('div', 'sode-live__drawer-head', el('h2', 'sode-live__drawer-h', 'Edit lists'), close), tabs.node, ...panes),
  );
  // Artwork from the bar opens the drawer for the Artwork panel alone, and its close closes the drawer again.
  let artOnly = false;
  const art = createLiveArt(ed.cellTab, () => {
    if (!artOnly) return;
    artOnly = false;
    setOpen(false);
  });
  aside.append(art.node);
  aside.id = 'ed-live-drawer';
  aside.setAttribute('aria-label', 'Sources and destinations');
  // Esc in a list drops its selection (editorgridlist.js); Esc anywhere else in the drawer closes it.
  aside.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || e.defaultPrevented) return;
    e.preventDefault();
    setOpen(false);
    open.focus();
  });

  let opened = false;
  function setOpen(on) {
    if (!on) art.close();
    opened = on;
    aside.classList.toggle('pb-drawer--open', on);
    // A closed drawer is zero wide, but its controls could still take focus.
    aside.inert = !on;
    open.setAttribute('aria-expanded', String(on));
    open.setAttribute('aria-pressed', String(on));
    const next = icon(on ? 'panel-right-close' : 'panel-right-open');
    openIcon.replaceWith(next);
    openIcon = next;
    open.dataset.pbTip = on ? 'Close the lists' : 'Change the order of the sources and destinations, pin, add and remove them';
    placeActs();
  }

  // Grid picks to list selection. The Cell tab edits one list at a time (editorside.js), so the kind picked last wins.
  const lists = ro ? null : ed.edgeLists();
  const KINDS = ['srcs', 'dsts'];
  let had = { srcs: [], dsts: [] };
  let kind = null; // the kind whose grid picks the list holds
  const same = (a, b) => a.length === b.length && a.every((x, i) => x === b[i]);
  function onPicks(p) {
    if (!lists) return;
    const changed = KINDS.filter((k) => !same(p[k], had[k]));
    had = { srcs: p.srcs, dsts: p.dsts };
    if (!changed.length) return;
    const next = changed.filter((k) => p[k].length).pop() ?? KINDS.find((k) => p[k].length) ?? null;
    if (next) lists[next].select(p[next]);
    else if (kind) lists[kind].clearPicks();
    kind = next;
    placeActs();
  }
  // The list's own bar, so Move, Pin and Artwork are the drawer's buttons. It goes back into the list on a drawer open.
  const actsHome = new Map();
  function placeActs() {
    if (!lists) return;
    const show = kind && (!opened || artOnly) ? lists[kind].acts : null;
    for (const k of KINDS) {
      const a = lists[k].acts;
      if (a === show || !picked.contains(a)) continue;
      actsHome.get(a).prepend(a);
    }
    if (show && !picked.contains(show)) {
      actsHome.set(show, show.parentNode);
      picked.append(show);
    }
  }
  setOpen(false);

  let run = null;
  let shown = ed.edges();
  function paint(err = '') {
    const dirty = ed.dirty();
    save.hidden = !dirty;
    state.textContent = err || (dirty ? 'Unsaved changes' : '');
  }
  paint();

  return {
    bar,
    open,
    aside,
    onPicks,
    started(r) {
      run = r;
    },
    changed() {
      paint();
      const now = ed.edges();
      if (now.sources === shown.sources && now.destinations === shown.destinations && now.rev === shown.rev) return;
      const saved = now.rev !== shown.rev;
      shown = now;
      // A rev goes with the lists only after a save, so the run marks that copy as seen and shows no banner for it.
      run?.show({ sources: now.sources, destinations: now.destinations, ...(saved ? { rev: now.rev } : {}) });
    },
    openArt() {
      if (!opened) {
        if (!kind) return false;
        artOnly = true;
        setOpen(true);
      }
      if (art.open()) return true;
      if (artOnly) {
        artOnly = false;
        setOpen(false);
      }
      return false;
    },
    stop() {
      art.close();
      kind = null;
      placeActs();
      home.append(srcs, dsts);
    },
  };
}
