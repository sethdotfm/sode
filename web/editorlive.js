// The Live Editor: the saved panel runs in place of the grid and sidebar (panelrun.js, embedded), over the editor's own
// session. It is not an instance of the panel, so it sends no keepalive. Both panel types get one bar that says takes
// there change the router and holds Back to the editor. The editor's toolbar hides while it runs (panels.css). A Grid panel's bar
// also holds Save and Edit lists. A Grid panel also gets a drawer for the editor's own lists (editorgridlist.js), moved
// in for the run.
import { el, icon } from './core.js';
import { sessionFetch } from './sessionkey.js';
import { runPanel } from './panelrun.js';
import { liveUi } from './panelmount.js';
import { makeTabs } from './tabs.js';
import { btn } from './formbits.js';
import { createLiveArt } from './editorliveart.js';
import { createLiveMenu } from './editorlivemenu.js';

// host holds the live panel, body is the editor it replaces, button is the toolbar's Live Editor button. ed is
// { save, dirty, readOnly, edges, lists, cellTab, edgeLists, leave, editor }. editor is runEditor's seams, for the
// Smart menu (editorlivemenu.js). leave(href) is the unsaved-edits rule of the
// Grid head row's links. edges() is { sources, destinations, rev } as the editor
// has them. lists() gives the two list columns, cellTab() the sidebar's Cell tab and edgeLists() the lists' own apis.
export function createLive(host, body, button, ed) {
  const editor = body.parentElement;
  let run = null;
  let grid = null; // the bar and drawer of a Grid panel's run
  let ui = null;
  const api = {
    on: () => !!run,
    // page is the Page being edited, where the run opens.
    start(id, page = '', isGrid = false) {
      api.stop();
      grid = isGrid ? gridParts(ed) : null;
      ui = liveUi(host, () => api.stop(), grid, grid?.bar || liveBar());
      host.hidden = false;
      body.hidden = true;
      editor.dataset.live = isGrid ? 'grid' : '';
      run = runPanel(id, { ui, call: sessionFetch, page });
      grid?.started(run);
      ui.back.focus();
    },
    stop() {
      if (!run) return;
      run.stop();
      run = null;
      ui.stopScreen();
      ui = null;
      grid?.stop();
      grid = null;
      host.replaceChildren();
      host.hidden = true;
      body.hidden = false;
      delete editor.dataset.live;
      button.focus({ preventScroll: true });
    },
    // A note in the Grid bar's state, in place of "Unsaved changes" until the next change.
    note: (text) => grid?.note(text),
    // The editor's fields or save state changed.
    changed: () => grid?.changed(),
    // A list's Artwork button: true when the open drawer shows the artwork.
    openArt: () => !!grid?.openArt(),
  };
  return api;
}

// The bar over a run: "Live Editor", the badge that says takes change the router, then end, all from the left.
// liveUi (panelmount.js) adds Back to the editor.
function liveBar(...end) {
  const title = el('h2', 'sode-live__title', 'Live Editor');
  title.id = 'ed-live-title';
  const bar = el('div', 'pb-banner sode-live__bar', title, el('span', 'pb-badge pb-badge--warn', 'Live: takes change the router'), ...end);
  bar.id = 'ed-live-bar';
  bar.setAttribute('role', 'toolbar');
  bar.setAttribute('aria-labelledby', title.id);
  return bar;
}

function gridParts(ed) {
  const [srcs, dsts] = ed.lists();
  const home = srcs.parentNode;
  const ro = ed.readOnly();

  const state = el('span', 'sode-live__state');
  state.id = 'ed-live-state';
  state.setAttribute('role', 'status');
  const save = btn('sode-btn--primary pb-btn--sm', 'Save', async () => {
    save.disabled = true;
    const err = await ed.save();
    save.disabled = false;
    paint(err);
  });
  save.id = 'ed-live-save';
  // Back to the editor goes to its left (panelmount.js).
  const open = btn('pb-btn--outline pb-btn--lg', '', () => setOpen(!opened));
  open.id = 'ed-live-lists';
  let openIcon = icon('panel-right-open');
  open.append(openIcon, el('span', null, 'Edit lists'));
  open.setAttribute('aria-controls', 'ed-live-drawer');
  open.hidden = ro;
  const bar = liveBar(state, save, open);

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
    open.dataset.pbTip = on ? 'Close the lists' : 'Order, pin, add, remove';
  }

  // Grid picks to list selection. The Cell tab edits one list at a time (editorside.js), so the kind picked last wins.
  const lists = ro ? null : ed.edgeLists();
  const KINDS = ['srcs', 'dsts'];
  let had = { srcs: [], dsts: [] };
  let kind = null; // the kind whose grid picks the list holds
  const same = (a, b) => a.length === b.length && a.every((x, i) => x === b[i]);
  function onPicks(p) {
    if (!lists) return menu.picks(p);
    const changed = KINDS.filter((k) => !same(p[k], had[k]));
    had = { srcs: p.srcs, dsts: p.dsts };
    if (!changed.length) return;
    const next = changed.filter((k) => p[k].length).pop() ?? KINDS.find((k) => p[k].length) ?? null;
    if (next) lists[next].select(p[next]);
    else if (kind) lists[kind].clearPicks();
    kind = next;
    menu.picks(p);
  }
  const menu = createLiveMenu(ed.editor(), lists, {
    kind: () => kind,
    openArt: () => api.openArt(),
    drawer: () => (opened ? aside : null),
  });
  setOpen(false);

  let run = null;
  let shown = ed.edges();
  function paint(err = '') {
    const dirty = ed.dirty();
    save.hidden = !dirty;
    state.textContent = err || (dirty ? 'Unsaved changes' : '');
  }
  paint();

  const api = {
    bar,
    open,
    aside,
    // The run puts its status slot and Full screen in the editor's page header (gridpanelheadview.js).
    topBar: document.querySelector('.sode-ed > .sode-bar'),
    onPicks,
    leave: ed.leave,
    editKey: (e, ctx) => menu.key(e, ctx),
    useRun: (r, node) => menu.useRun(r, node),
    note: paint,
    started(r) {
      run = r;
    },
    changed() {
      paint();
      menu.changed();
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
      menu.stop();
      kind = null;
      home.append(srcs, dsts);
    },
  };
  return api;
}
