// Runs /panels/editor as the signed-in user. runEditor returns the seams the other editor modules use (see the end).
import { $, on, S } from './core.js';
import { newPanelActs, actsSavedNote } from './panelsrules.js';
import { sessionFetch, jsonCall, startSession } from './sessionkey.js';
import { namesFrom, fixedDestsOf, artLeftOut } from './panelrules.js';
import { defaultsBy } from './panellook.js';
import { createGrid, cellsLost, resize, select, deselect, selectAll, rectOf, clearSelection, setAside, putBack, heldThatFit, saved } from './gridmodel.js';
import { editorTarget, newPanel, saveBody, isDirty, menuArea, saveChord, openFailNote, editorAccess } from './editorrules.js';
import { copyText, itemLink } from './panellinkrow.js';
import { createEditorGrid } from './editorview.js';
import { editorSignIn } from './editorsignin.js';
import { drawPicker, paintToolbar } from './editortoolbar.js';
import { menuItems } from './editormenuitems.js';
import { createArt } from './panelart.js';
import { createSmartMenu } from './editormenu.js';
import { gridCommand } from './gridchords.js';
import { createLive } from './editorlive.js';
import { copyBlock, blockText, readBlock, pastePlan, applyPaste, pasteNote } from './gridclip.js';
import { writeClip, createPasteReader } from './editorclip.js';
import { emptyUndo, record, undo, redo, sameCells, pageSnap, samePages, restorePages, rekeySaved } from './gridundo.js';
import { edgeChoices, edgeNote } from './gridedge.js';
import { askPasteEdge } from './editorpasteedge.js';
import { pagesAfterSave, pagesOf, pageView, fitPages } from './panelpages.js';
import { addPage, movePage, renamePage, enablePage, deletePage, copyPage, openClick, settle } from './editorpagerules.js';
import { createPageBar } from './editorpagebar.js';
import { createPageView } from './editorpageview.js';
import { askCopyTo } from './editorpagecopy.js';
import { isGridPanel } from './gridpaneledges.js';
import { edgesIn } from './gridedgerefs.js';

const GRID_EDGE = Object.freeze({ members: 'all' });

const say = (id, text) => {
  const n = $(id);
  n.textContent = text;
  n.hidden = !text;
};

const LEFT = 'Signed out. Not saved.';

// One call whose failures read "<what> (status)" unless the server says why; down words a dead network.
const edCall = (what, down) => jsonCall(sessionFetch, { failed: (s) => `${what} (${s})`, down });

export async function runEditor() {
  let g = createGrid();
  let loaded = null; // the stored panel being edited; null for a new one
  // The panel's Pages. g is the focused Page (focus, its id), so that entry's cells here are stale: livePages() has
  // them fresh. open is the ids of the Pages shown side by side. Only an unsaved Page 1 has id ''.
  let pages = pagesOf(newPanel());
  let focus = '';
  let open = [''];
  let list = [];
  let names = namesFrom({});
  let routerNames = {}; // GET /names as read, for the Grid panel's list tabs
  let discardAsked = ''; // the panel the user tried to leave unsaved; a second try discards.
  let leaving = false; // the access the editor opened with is gone (leaveEditing): nothing more is saved.
  const menuSections = []; // (grid) => items: Find adds its items here.
  const keyHooks = []; // (event) => true when a grid key was handled (Find handles typed text and Enter).
  const changeWatchers = [];
  const fieldWatchers = [];
  let art = null; // createArt for the loaded panel: blob URLs of its uploaded artwork.
  let defaults = new Map(); // Default artwork by item (defaultsBy): drawn only, never put into g.
  let edits = {}; // the sidebar's unsaved panel fields: {settings, acts, clientRights}, each after its first edit.
  // The save's state for the toolbar: busy while a save runs, failed holds why the last save failed until one works.
  const saveState = { busy: false, failed: '' };
  const paintWatchers = []; // (grid, selection box) after every repaint, for things that sit over the squares
  const readOnly = () => !!loaded && !loaded.canEdit;
  const fields = () => ({
    settings: edits.settings ?? loaded?.settings ?? {},
    acts: edits.acts ?? loaded?.acts ?? newPanelActs(S.cfg),
    slug: edits.slug ?? loaded?.slug ?? '',
    clientRights: edits.clientRights,
    // A Grid panel's type is chosen once, before the first save; the server keeps a stored one.
    type: edits.type ?? loaded?.type ?? 'buttons',
    sources: edits.sources ?? loaded?.sources ?? GRID_EDGE,
    destinations: edits.destinations ?? loaded?.destinations ?? GRID_EDGE,
  });
  const fieldsChanged = () => fieldWatchers.forEach((fn) => fn());

  // While Fixed destination is on, Destination cells live in held, out of every grid edit, and go back into the
  // grid when it turns off. whole() is the grid as stored; saves and the dirty check use it. A held cell whose
  // squares get filled or that no longer fits is dropped at once and counted, as resize drops a visible cell.
  let held = [];
  const fixedOn = () => fixedDestsOf({ settings: fields().settings }).length > 0;
  const holdOrPutBack = (next) => {
    const all = held;
    if (!fixedOn()) {
      held = [];
      return { shown: putBack(next, all), dropped: all.length - heldThatFit(next, all).length };
    }
    const aside = setAside(next, 'destination');
    held = heldThatFit(aside.grid, [...all, ...aside.held]);
    return { shown: aside.grid, dropped: all.length + aside.held.length - held.length };
  };
  const droppedNote = (n) => (n ? `Removed ${n} Destination cell${n > 1 ? 's' : ''}: other cells fill their squares.` : '');
  const sayDropped = (n) => n && say('ed-status', droppedNote(n));
  const whole = () => putBack(g, held);
  const fi = () =>
    Math.max(
      0,
      pages.findIndex((p) => p.id === focus),
    );
  const livePages = () => pages.map((p, i) => (i === fi() ? { ...p, cells: saved(whole()).cells } : p));
  // Page 1 as stored, which saves, the dirty check and templates read beside the Page list.
  const firstGrid = () => createGrid({ rows: g.rows, cols: g.cols, cells: livePages()[0].cells });
  // g from the focused Page's stored cells. held starts empty so holdOrPutBack sets its Destination cells aside again.
  const showFocused = () => {
    held = [];
    g = holdOrPutBack(createGrid({ rows: g.rows, cols: g.cols, cells: pages[fi()].cells })).shown;
  };
  const changed = () => {
    refresh();
    for (const fn of changeWatchers) fn(g);
  };

  const nameBox = $('ed-name');
  const dirty = () => isDirty(loaded, nameBox.value, firstGrid(), edits, livePages());

  // Every cell edit, on any Page, passes here, so this is where undo records (gridundo.js): one stack for the
  // panel. what names the step on the status line. join makes a run of edits one step. list marks a step that
  // replaces the Page list (a template). A selection move or a Page op changes no cells, so it records nothing.
  let steps = emptyUndo();
  const snap = (list = false) => pageSnap(livePages(), g.rows, g.cols, { list, focus, sel: g.sel });
  function edit(what, run, { join, list = false } = {}) {
    const before = snap(list);
    const out = run();
    if (!samePages(before, snap())) steps = record(steps, { what, before, join });
    return out;
  }
  // An edit of the focused Page. Returns how many held cells it dropped.
  const setGrid = (next, { what = 'edit', join } = {}) => {
    const was = g;
    const dropped = edit(
      what,
      () => {
        const { shown, dropped } = holdOrPutBack(next);
        sayDropped(dropped);
        g = shown;
        return dropped;
      },
      { join },
    );
    if (g !== was) changed();
    return dropped;
  };
  // Not through edit, which would record the step back. Cells go back by Page id, so Page names, order and
  // deletions stay as they are; the focused Page stays focused while it exists.
  function step(back) {
    const top = (back ? steps.past : steps.future).at(-1);
    if (!top) return say('ed-status', back ? 'Nothing to undo.' : 'Nothing to redo.');
    const s = (back ? undo : redo)(steps, snap(!!top.grid.list));
    steps = s.undo;
    ({ pages, focus, open } = settle({ pages: restorePages(livePages(), s.grid), focus, open }));
    g = createGrid({ rows: s.grid.rows, cols: s.grid.cols });
    showFocused();
    if (s.grid.focus === focus && s.grid.sel) g = { ...g, sel: s.grid.sel };
    changed();
    say('ed-status', `${back ? 'Undid' : 'Redid'} ${s.what}.`);
  }

  // Only Copy to is an undo step (editorpagebar.js).

  function focusPage(id) {
    if (id === focus) return;
    ({ pages, focus, open } = settle({ pages: livePages(), focus: id, open }));
    showFocused();
    menuAway = false;
  }
  const pageActs = {
    open(id, multi) {
      const next = openClick(open, id, multi);
      const added = !open.includes(id);
      open = next;
      if (!multi || added) focusPage(id);
      else if (!open.includes(focus)) focusPage(open[0]);
      changed();
    },
    add() {
      const next = addPage(livePages());
      if (next.length === pages.length) return;
      pages = next;
      focusPage(next.at(-1).id);
      open = [focus];
      changed();
      say('ed-status', `Added Page ${next.length}.`);
    },
    rename(i, text) {
      pages = renamePage(livePages(), i, text);
      refresh();
    },
    enable(i, on) {
      const live = livePages();
      pages = enablePage(live, i, on);
      refresh();
      if (pages !== live || on) return true;
      say('ed-status', 'At least one Page stays on, so the panel always has a Page to show.');
      return false;
    },
    move(i, to) {
      pages = movePage(livePages(), i, to);
      refresh();
    },
    remove(i) {
      const live = livePages();
      const gone = live[i].id;
      pages = deletePage(live, i);
      if (pages === live) return say('ed-status', 'Nothing deleted: at least one Page stays on, so turn another Page on first.');
      const left = open.filter((id) => id !== gone);
      const next = focus !== gone ? focus : (left.length ? pages.find((p) => left.includes(p.id)) : pages[Math.min(i, pages.length - 1)]).id;
      ({ pages, focus, open } = settle({ pages, focus: next, open: left }));
      showFocused();
      changed();
      say('ed-status', `Deleted Page ${i + 1}.`);
    },
    async copyTo(i) {
      const to = await askCopyTo(livePages(), i, document.activeElement);
      if (to === null || readOnly()) return;
      edit('Copy to', () => {
        const c = copyPage(livePages(), i, to);
        if (!c) return;
        pages = c.pages;
        if (to === fi()) showFocused();
      });
      changed();
      say('ed-status', `Copied Page ${i + 1} to Page ${to + 1}.`);
    },
  };
  const bar = createPageBar($('ed-pages'), pageActs);
  // A press on another open Page focuses it, then acts as a press on #ed-grid, which now draws that Page.
  const openView = createPageView($('ed-frame'), $('ed-frame-title'), {
    focus(id, r, c) {
      if (readOnly()) return;
      focusPage(id);
      setGrid(select(g, r, c));
      changed();
      $('ed-grid').focus({ preventScroll: true });
    },
    key(id, e) {
      if (readOnly()) return;
      focusPage(id);
      changed();
      $('ed-grid').focus({ preventScroll: true });
      onKey(e);
    },
  });

  const grid = createEditorGrid($('ed-grid'), {
    select(r, c, extend) {
      if (readOnly()) return;
      menuAway = false;
      setGrid(select(g, r, c, extend)); // always a new grid, so the menu comes back
    },
    key: onKey,
  });
  let cellTab = null; // the sidebar's Cell tab, { node, openPicker } (editorside.js)
  let edgeLists = null; // the Sources and Destinations lists, { srcs, dsts } (editorgridlist.js)
  const live = createLive($('ed-live-view'), document.querySelector('.sode-ed__body'), $('ed-live'), {
    save: () => save(),
    dirty: () => !readOnly() && dirty(),
    readOnly,
    edges: () => ({ sources: fields().sources, destinations: fields().destinations, rev: loaded?.rev }),
    lists: () => [$('ed-sources-col'), $('ed-destinations-col')],
    cellTab: () => cellTab,
    edgeLists: () => edgeLists,
  });
  fieldWatchers.push(() => live.changed());
  // The menu and its flyouts keep off the sidebar when it sits beside the grid (menuArea).
  const menu = createSmartMenu($('ed-grid'), {
    onEscape: () => $('ed-grid').focus(),
    area: () =>
      menuArea(
        { w: document.documentElement.clientWidth, h: document.documentElement.clientHeight },
        $('ed-side').getBoundingClientRect(),
        $('ed-grid').getBoundingClientRect(),
      ),
  });

  // A click or Tab outside the grid, the menu and the sidebar drops the selection; a click in the sidebar only hides
  // the menu. Not blur: the menu rebuilds on every change and drops focus. The event path, not the target: a repaint
  // detaches the target.
  let menuAway = false;
  const leave = (e) => {
    if (!g.sel) return;
    const path = e.composedPath();
    // The paste past the edge dialog and the Icon library dialog act on the selection. So focus in them does not
    // leave the grid. A press on another open Page moves the selection there itself.
    if ([$('ed-open-pages'), menu.root].some((n) => path.includes(n)) || path.some((n) => n.id === 'ed-edge' || n.id === 'ed-lib-dialog')) return;
    if (path.includes($('ed-side'))) {
      if (menuAway) return;
      menuAway = true;
      return refresh();
    }
    setGrid(deselect(g));
  };
  document.addEventListener('pointerdown', leave);
  document.addEventListener('focusin', leave);

  async function copyCells() {
    const block = copyBlock(g, loaded?.id);
    if (!block) return say('ed-status', 'Nothing copied: no cells selected.');
    const n = block.cells.length;
    const ok = await writeClip(blockText(block));
    say('ed-status', ok ? `Copied ${n} cell${n > 1 ? 's' : ''}.` : 'Nothing copied: the browser did not let Sode use the clipboard.');
    return ok;
  }

  // Copy, then one setGrid removes the selection, the step an undo wraps. Held Destination cells are not in g, so
  // they are neither copied nor removed. A failed copy or a grid that moved during the write keeps every cell.
  async function cutCells() {
    const snap = g;
    const n = copyBlock(g, loaded?.id)?.cells.length;
    if (!(await copyCells())) return;
    if (g !== snap) return say('ed-status', 'Copied. Nothing removed: the grid changed.');
    setGrid(clearSelection(g), { what: 'cut' });
    say('ed-status', `Cut ${n} cell${n > 1 ? 's' : ''}.`);
  }

  // One paste is one setGrid, the step an undo wraps. Text that is not a Sode block changes nothing.
  function pasteText(text) {
    if (readOnly()) return;
    if (text === null) return say('ed-status', 'Nothing pasted: the browser did not let Sode read the clipboard.');
    const block = readBlock(text);
    if (!block) return say('ed-status', 'Nothing pasted: no Sode cells on the clipboard.');
    const plan = pastePlan(g, block, { panel: loaded?.id, fixed: fixedOn() });
    if (!plan) return say('ed-status', 'Nothing pasted: select a square first.');
    if (!plan.fits) return void pasteOverflow(plan);
    const dropped = setGrid(applyPaste(g, plan.placed), { what: 'paste' });
    say('ed-status', `${pasteNote(plan)} ${droppedNote(dropped)}`.trim());
  }
  // A paste that runs past the grid's edge asks Clip, Wrap or Cancel; a choice is one setGrid, so one undo step.
  // Find's fill order is read at paste time (fillOrder, set by Find).
  let fillOrder = () => 'across';
  async function pasteOverflow(plan) {
    const snap = whole();
    const before = livePages();
    const choices = edgeChoices(g, plan, fillOrder(), { pages: before, from: fi() });
    const answer = await askPasteEdge(choices, plan.outside, $('ed-grid'));
    if (!answer) return say('ed-status', 'Nothing pasted.');
    if (readOnly() || !sameCells(snap, whole()) || !samePages(pageSnap(before, g.rows, g.cols), pageSnap(livePages(), g.rows, g.cols)))
      return say('ed-status', 'Nothing pasted: the grid changed.');
    const choice = choices[answer];
    // A cascade onto later Pages and the paste here are one undo step, so the Page list changes inside the same edit.
    const dropped = edit('paste', () => {
      for (const o of choice.onto || []) pages = pages.map((p, i) => (i === o.index ? { ...p, cells: o.cells } : p));
      const out = holdOrPutBack(applyPaste(g, choice.placed));
      sayDropped(out.dropped);
      g = out.shown;
      return out.dropped;
    });
    changed();
    say('ed-status', `${edgeNote(plan, choice)} ${droppedNote(dropped)}`.trim());
  }
  const paste = createPasteReader($('ed-grid'), pasteText);

  // One entry per command name in gridchords.js GRID_CHORDS. One that returns false leaves its key to the browser.
  const gridCommands = {
    selectAll: () => setGrid(selectAll(g)),
    copy: () => void copyCells(),
    cut: () => void cutCells(),
    paste: () => (paste.key(), false),
    undo: () => step(true),
    redo: () => step(false),
  };

  function onKey(e) {
    if (readOnly()) return;
    if (menuAway) {
      menuAway = false;
      refresh();
    }
    if (menu.run(e)) return void e.preventDefault();
    const run = gridCommands[gridCommand(e)];
    if (run) {
      if (run() !== false) e.preventDefault();
      return;
    }
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (keyHooks.some((fn) => fn(e))) return;
    const arrow = { ArrowUp: [-1, 0], ArrowDown: [1, 0], ArrowLeft: [0, -1], ArrowRight: [0, 1] }[e.key];
    const q = rectOf(g);
    if (arrow) {
      e.preventDefault();
      if (!q) return void setGrid(select(g, 1, 1));
      const [dr, dc] = arrow;
      const h = g.sel.head;
      // From the selection's edge in the direction of travel, so a merged cell is stepped over, not stuck on.
      const r = dr > 0 ? q.r2 : dr < 0 ? q.r1 : h.r;
      const c = dc > 0 ? q.c2 : dc < 0 ? q.c1 : h.c;
      setGrid(select(g, Math.max(1, Math.min(g.rows, r + dr)), Math.max(1, Math.min(g.cols, c + dc)), e.shiftKey));
    } else if (e.key === 'Delete' || e.key === 'Backspace') {
      e.preventDefault();
      setGrid(clearSelection(g), { what: 'Delete' });
    } else if (e.key === 'Escape') {
      setGrid(deselect(g));
    } else if (e.key === 'Enter' && q) {
      e.preventDefault();
      menu.focus();
    }
  }

  // A key without the panels scope, or a failed read, leaves the defaults as they were.
  async function loadDefaults() {
    const r = await sessionFetch('api/v1/default-artwork').catch(() => null);
    const got = r?.ok ? await r.json().catch(() => null) : null;
    if (!got) return;
    defaults = defaultsBy(got.defaults);
    refresh();
  }

  function refresh() {
    const ro = readOnly();
    // Grid panels have no cells, size or Pages: CSS hides the grid, the Pages bar and Templates.
    document.querySelector('.sode-ed').dataset.type = isGridPanel(fields()) ? 'grid' : 'buttons';
    const live = livePages();
    grid.paint(g, names, { readOnly: ro, art, defaults, pages: live, page: focus });
    openView.paint(live, open, focus, { rows: g.rows, cols: g.cols, names, fixed: fixedOn(), readOnly: ro, art, defaults });
    bar.paint({ pages: live, open, focus, readOnly: ro });
    const box = ro ? null : grid.selectionBox();
    menu.show(
      menuAway ? null : box,
      menuItems(() => g, fixedOn(), menuSections, setGrid),
    );
    for (const fn of paintWatchers) fn(g, box);
    paintToolbar(ro, saveState, dirty, loaded);
  }

  // The Panel tab's Rows and Columns call this; a bad value keeps the current size.
  function setSize(which, value) {
    const rows = which === 'rows' ? value : g.rows;
    const cols = which === 'cols' ? value : g.cols;
    const next = resize(g, rows, cols);
    // Every Page at once, one undo step.
    const lost = edit('resize', () => {
      const f = fi();
      const here = cellsLost(g, rows, cols); // held cells that no longer fit come in kept.dropped
      const fit = fitPages(
        pages.map((p, i) => (i === f ? { ...p, cells: [] } : p)),
        next.rows,
        next.cols,
      );
      pages = fit.pages.map((p, i) => (i === f ? pages[i] : p));
      const kept = holdOrPutBack(next);
      g = kept.shown;
      return here + fit.lost + kept.dropped;
    });
    say('ed-status', lost ? `Removed ${lost} cell${lost > 1 ? 's' : ''} that no longer fit.` : '');
    changed();
  }
  nameBox.addEventListener('input', refresh);

  async function loadList() {
    list = (await edCall('The panel list could not be read', 'Sode did not answer. Reload.')('api/v1/panels')).panels || [];
    drawPicker(list, loaded);
  }

  function show(panel) {
    live.stop();
    loaded = edgesIn(panel);
    edits = {};
    art?.revoke();
    art = panel ? createArt(sessionFetch, panel.id) : null;
    held = [];
    steps = emptyUndo();
    const first = pagesOf(panel || newPanel());
    ({ pages, focus, open } = settle({ pages: first, focus: first[0].id, open: [] }));
    g = holdOrPutBack(createGrid(pageView(panel || newPanel()))).shown;
    nameBox.value = (panel || newPanel()).name;
    discardAsked = '';
    say('ed-status', '');
    $('ed-pick').value = panel?.id || '';
    document.title = panel ? `${panel.name} (editor)` : 'Sode Panel Editor';
    refresh();
    for (const fn of changeWatchers) fn(g);
    fieldsChanged();
  }

  // Only the newest open counts: a slow answer for a panel picked earlier must not replace a later pick.
  let opening = 0;
  async function openPanel(id, push) {
    const mine = ++opening;
    if (!id) {
      if (push) history.pushState(null, '', 'panels/editor?new');
      return show(null);
    }
    let panel;
    try {
      panel = await edCall('The panel could not be opened', '')(`api/v1/panels/${id}`);
    } catch (e) {
      if (mine !== opening) return;
      if (e.status === undefined) throw e;
      say('ed-status', openFailNote(e));
      drawPicker(list, loaded);
      return;
    }
    if (mine !== opening) return;
    if (push) history.pushState(null, '', `panels/editor?panel=${id}`);
    show(panel);
  }

  $('ed-pick').addEventListener('change', (e) => {
    const id = e.target.value;
    if (!readOnly() && dirty() && discardAsked !== id) {
      discardAsked = id;
      e.target.value = loaded?.id || '';
      say('ed-status', 'Unsaved changes. Choose again to discard.');
      return;
    }
    openPanel(id, true);
  });
  // Back and Forward leave unsaved edits only on a second try, as the picker does.
  window.addEventListener('popstate', () => {
    const id = editorTarget(location.search).id;
    if (!readOnly() && dirty() && id !== (loaded?.id || '') && discardAsked !== id) {
      discardAsked = id;
      history.pushState(null, '', loaded ? `panels/editor?panel=${loaded.id}` : 'panels/editor');
      say('ed-status', 'Unsaved changes. Go back again to discard.');
      return;
    }
    openPanel(id, false);
  });
  window.addEventListener('beforeunload', (e) => {
    if (!leaving && !readOnly() && dirty()) e.preventDefault();
  });

  // One save at a time. A second Save, Ctrl+S or Live Editor press during a save gets the same answer. So a new panel is
  // never made twice.
  let saving = null;
  function save() {
    if (leaving) return Promise.resolve(LEFT);
    saving ??= saveNow().finally(() => {
      saving = null;
    });
    return saving;
  }

  // A PUT that got no answer or a server error tries once more by itself. A PUT sends the whole panel, so a second
  // one is safe. A POST does not try again: a lost answer can hide a panel that it made.
  const RETRY_MS = 1000;
  async function saveNow() {
    saveState.busy = true;
    refresh();
    const sentEdits = edits;
    const sentName = nameBox.value;
    const sentPages = livePages();
    const body = JSON.stringify(saveBody(loaded || { ...newPanel(), acts: fields().acts }, sentName, firstGrid(), edits, sentPages));
    const send = () =>
      edCall('The panel was not saved', 'Sode did not answer. Not saved.')(loaded ? `api/v1/panels/${loaded.id}` : 'api/v1/panels', {
        method: loaded ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body,
        // Signed out, a save is refused at once: waiting would send it under the next sign-in, maybe someone else's.
        noWait: true,
      });
    let stored;
    try {
      stored = await send().catch(async (e) => {
        if (!loaded || (e.status !== 0 && !(e.status >= 500))) throw e;
        await new Promise((ok) => setTimeout(ok, RETRY_MS));
        return send();
      });
    } catch (e) {
      saveState.busy = false;
      saveState.failed = e.message;
      say('ed-status', `${e.message} Changes kept.`);
      refresh();
      return e.message;
    }
    const made = !loaded;
    loaded = edgesIn(stored);
    // Pages edited during the save stay as they are now; the rest take the stored copy. The first save gives an
    // unsaved Page 1 its id.
    const after = pagesAfterSave(sentPages, livePages(), pagesOf(stored));
    const rekey = (id) => after.ids[id] ?? id;
    ({ pages, focus, open } = settle({ pages: after.pages, focus: rekey(focus), open: open.map(rekey) }));
    steps = rekeySaved(
      steps,
      sentPages.map((p) => p.id),
      sentPages.map((p) => rekey(p.id)),
    );
    // The server fills router references (internal/app/panelrefs.go). A focused Page not edited during the save takes
    // them, or it reads as Unsaved changes.
    if (pagesOf(stored).includes(pages[fi()])) {
      const sel = g.sel;
      showFocused();
      g = { ...g, sel };
    }
    // A field changed while the save was in flight is a newer edit: it stays, and the editor says Unsaved changes.
    edits = Object.fromEntries(Object.entries(edits).filter(([k, v]) => v !== sentEdits[k]));
    if (made) history.replaceState(null, '', `panels/editor?panel=${loaded.id}`);
    if (nameBox.value === sentName) nameBox.value = loaded.name;
    if (saveState.failed) say('ed-status', '');
    // A new Public panel asks no Confirm, since it ends no keys, so its first save says who can open it.
    if (made && loaded.acts === 'public') say('ed-status', actsSavedNote('public'));
    saveState.busy = false;
    saveState.failed = '';
    await loadList().catch(() => {});
    refresh();
    fieldsChanged();
    return '';
  }
  $('ed-save').addEventListener('click', save);
  // Capture, so it also works with focus in a field; the browser's own Save page never opens here.
  document.addEventListener(
    'keydown',
    (e) => {
      if (!saveChord(e)) return;
      e.preventDefault();
      if (!$('ed-save').disabled) save();
    },
    true,
  );

  // The live panel is the saved panel, and it leaves unassigned cells out (panelrules.js cellsIn), so unsaved
  // edits are saved first. Open panels follow a save by themselves.
  $('ed-live').addEventListener('click', async () => {
    if (live.on()) return live.stop();
    // A second click while the save runs waits for the same save.
    if (!readOnly() && (saving || dirty())) await save();
    if (loaded && !dirty() && !live.on()) live.start(loaded.id, focus, isGridPanel(loaded));
  });

  $('ed-copylink').addEventListener('click', async () => {
    say('ed-status', (await copyText(itemLink(loaded))) ? 'Link copied.' : 'Copy failed: use Share > Link.');
  });

  $('ed-copy').addEventListener('click', async () => {
    let copy;
    try {
      copy = await edCall('The copy was not saved', 'Sode did not answer. No copy made.')(`api/v1/panels/${loaded.id}/copy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
        noWait: true,
      });
    } catch (e) {
      return say('ed-status', e.message);
    }
    history.pushState(null, '', `panels/editor?panel=${copy.id}`);
    await loadList().catch(() => {});
    show(copy);
    say('ed-status', `Saved a copy. You own it. ${artLeftOut(copy.artSkipped)}`.trim());
  });

  const bootFailed = editorSignIn(say);
  // What is open belongs to the user who opened it: a sign-out, another user or another Role starts the page again.
  // The edits go with it, so the unsaved-changes prompt must not hold the page in the editor.
  let owner = null;
  on('access', (a) => {
    const next = editorAccess(owner, a);
    owner = next.owner;
    if (next.leave) leaveEditing();
  });
  function leaveEditing() {
    if (leaving) return;
    leaving = true;
    live.stop();
    document.querySelector('.sode-ed').hidden = true;
    location.reload();
  }
  // Names come from the REST read, which applies the user's Rights. The socket says when the router's names change,
  // so the cells and the Grid lists show a rename at once, as an open panel does.
  // Returns false when the read was refused; the names already shown stay.
  async function readNames() {
    const next = await sessionFetch('api/v1/names').then((r) => (r.ok ? r.json() : null));
    if (!next) return false;
    routerNames = next;
    names = namesFrom(routerNames);
    return true;
  }
  const namesFailed = () => say('ed-status', 'Router names did not load. Old names shown.');
  on('names', () =>
    readNames()
      .then((ok) => {
        if (!ok) return namesFailed();
        refresh();
        fieldsChanged();
      })
      .catch(namesFailed),
  );
  // A blank new panel's mode comes from the hello's config, which can come after the first paint.
  on('helloCfg', () => !loaded && fieldsChanged());
  startSession();
  show(null);
  // Not awaited: the seams below must exist while the page still waits for a sign-in. Then the sidebar and Find
  // get built, and nothing is left half drawn.
  (async () => {
    try {
      await loadList();
      await readNames();
      await loadDefaults();
      await openPanel(editorTarget(location.search).id, false);
    } catch (err) {
      bootFailed(err);
    }
  })();

  // Seams: Find adds menu items; the sidebars and live update read and replace the grid.
  return {
    // The focused Page's grid; Find, the grip, the art tool and the Cell tab work on it.
    grid: () => g,
    // Page 1 as stored, with any Destination cells Fixed destination set aside, for templates and the Cell tab's
    // Go to Page warning beside pages().
    storedGrid: firstGrid,
    setGrid,
    // The Page list with every Page's cells as they stand.
    pages: livePages,
    // A template apply: the whole Page list and Page 1's grid at once, one undo step that brings the old list back.
    replacePages(list, first) {
      edit(
        'template',
        () => {
          ({ pages, focus, open } = settle({ pages: list, focus: list[0].id, open: [] }));
          held = [];
          g = holdOrPutBack(first).shown;
        },
        { list: true },
      );
      changed();
    },
    setSize,
    panel: () => loaded,
    readOnly,
    // Saves now; answers '' when stored, else the refusal text.
    save,
    names: () => names,
    routerNames: () => routerNames,
    addMenuSection: (fn) => menuSections.push(fn),
    addKeyHook: (fn) => keyHooks.push(fn),
    // Find's fill order, which Wrap follows.
    useFillOrder: (fn) => (fillOrder = fn),
    fillOrder: () => fillOrder(),
    // The sidebar's Cell tab, which the Live Editor shows in its drawer; openArt is true when the drawer showed it.
    useCellTab: (t) => (cellTab = t),
    useEdgeLists: (l) => (edgeLists = l),
    openArt: () => live.openArt(),
    onChange: (fn) => changeWatchers.push(fn),
    // The sidebar's panel fields. onFields runs when the panel loads, saves or a field changes.
    fields,
    art: () => art,
    defaults: () => defaults,
    loadDefaults,
    setFields(patch) {
      edits = { ...edits, ...patch };
      const { shown, dropped } = holdOrPutBack(g);
      sayDropped(dropped);
      if (shown !== g) {
        g = shown;
        for (const fn of changeWatchers) fn(g);
      }
      refresh();
      fieldsChanged();
    },
    fixedOn,
    // A panel stored elsewhere (the Share modal): the grid and unsaved fields stay as they are.
    replaceLoaded(panel) {
      loaded = edgesIn(panel);
      loadList().catch(() => {});
      refresh();
      fieldsChanged();
    },
    onFields: (fn) => fieldWatchers.push(fn),
    onPaint: (fn) => paintWatchers.push(fn),
    refresh,
    status: (text) => say('ed-status', text),
  };
}
