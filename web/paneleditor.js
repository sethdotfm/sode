// Runs /panels/editor: the panel picker, the toolbar, the grid and the smart menu, over /api/v1/panels as
// the signed-in user. The grid rules are gridmodel.js and the page text is editorrules.js; this file wires
// them to the DOM and the network. runEditor returns the seams later tickets build on (see the end).
import { $, el } from './core.js';
import { sessionFetch, jsonCall, startSession, signInAgain, onSessionKey } from './sessionkey.js';
import { namesFrom } from './panelrules.js';
import {
  createGrid,
  cellsLost,
  resize,
  select,
  deselect,
  rectOf,
  selCells,
  setKind,
  clearSelection,
  canMerge,
  merge,
  canUnmerge,
  unmerge,
  countTodo,
  nextTodo,
} from './gridmodel.js';
import { editorTarget, groupPanels, newPanel, saveBody, isDirty, kindChoices, typeLabel, todoText } from './editorrules.js';
import { linkFor } from './panellink.js';
import { copyText } from './panellinkrow.js';
import { createEditorGrid } from './editorview.js';
import { createArt } from './panelart.js';
import { createSmartMenu } from './editormenu.js';

const say = (id, text) => {
  const n = $(id);
  n.textContent = text;
  n.hidden = !text;
};

// One call whose failures read "<what> (status)" unless the server says why; down words a dead network.
const edCall = (what, down) => jsonCall(sessionFetch, { failed: (s) => `${what} (${s})`, down });

export async function runEditor() {
  let g = createGrid();
  let loaded = null; // the stored panel being edited; null for a new one
  let list = [];
  let names = namesFrom({});
  let discardAsked = ''; // the panel the user tried to leave unsaved; asking again discards
  const menuSections = []; // (grid) => items: where Find and later actions plug in
  const keyHooks = []; // (event) => true when a grid key was handled (Find takes typing and Enter)
  const changeWatchers = [];
  const fieldWatchers = [];
  let art = null; // createArt for the loaded panel: blob URLs of its uploaded artwork
  let edits = {}; // the sidebar's unsaved panel fields: {settings, acts, clientRights}, each once edited
  const paintWatchers = []; // (grid, selection box) after every repaint, for things that sit over the squares
  const readOnly = () => !!loaded && !loaded.canEdit;
  const fields = () => ({
    settings: edits.settings ?? loaded?.settings ?? {},
    acts: edits.acts ?? loaded?.acts ?? 'session',
    slug: edits.slug ?? loaded?.slug ?? '',
    clientRights: edits.clientRights,
  });
  const fieldsChanged = () => fieldWatchers.forEach((fn) => fn());

  const nameBox = $('ed-name');
  const dirty = () => isDirty(loaded, nameBox.value, g, edits);

  const setGrid = (next) => {
    if (next === g) return;
    g = next;
    refresh();
    for (const fn of changeWatchers) fn(g);
  };

  const grid = createEditorGrid($('ed-grid'), {
    select(r, c, extend) {
      if (readOnly()) return;
      setGrid(select(g, r, c, extend));
    },
    key: onKey,
  });
  const menu = createSmartMenu($('ed-grid'), { onEscape: () => $('ed-grid').focus() });

  // The menu goes with the selection when a click or Tab lands outside the grid, the menu and the sidebar (which
  // edits the selected cell). Not a blur: the menu rebuilds itself on every change and drops focus for a moment.
  const inEditor = (t) => t instanceof Node && [$('ed-grid'), menu.root, $('ed-side')].some((n) => n.contains(t));
  const leave = (e) => {
    if (g.sel && !inEditor(e.target)) setGrid(deselect(g));
  };
  document.addEventListener('pointerdown', leave);
  document.addEventListener('focusin', leave);

  function onKey(e) {
    if (readOnly()) return;
    if (menu.run(e)) return void e.preventDefault();
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
      setGrid(clearSelection(g));
    } else if (e.key === 'Escape') {
      setGrid(deselect(g));
    } else if (e.key === 'Enter' && q) {
      e.preventDefault();
      menu.focus();
    }
  }

  function menuItems() {
    const cells = selCells(g);
    const items = [
      // One item that toggles, so Ctrl or Cmd+M merges a group and unmerges a merged square.
      canUnmerge(g)
        ? { chord: 'm', label: 'Unmerge', icon: 'split', run: () => setGrid(unmerge(g)) }
        : { chord: 'm', label: 'Merge', icon: 'combine', disabled: !canMerge(g), run: () => setGrid(merge(g)) },
      {
        label: typeLabel(cells),
        icon: 'type',
        sub: kindChoices().map(({ kind, label }) => ({
          label,
          current: kind === '' ? !cells.length : cells.length > 0 && cells.every((c) => c.kind === kind),
          run: () => setGrid(setKind(g, kind)),
        })),
      },
    ];
    for (const section of menuSections) items.push(...section(g));
    return items;
  }

  function refresh() {
    const ro = readOnly();
    grid.paint(g, names, { readOnly: ro, art });
    const box = ro ? null : grid.selectionBox();
    menu.show(box, menuItems());
    for (const fn of paintWatchers) fn(g, box);
    nameBox.disabled = ro;
    const todo = countTodo(g);
    $('ed-todo').textContent = todoText(todo);
    $('ed-next').disabled = ro || !todo;
    $('ed-save').disabled = ro || !dirty();
    $('ed-banner').hidden = !ro;
    say('ed-state', ro ? '' : dirty() ? 'Unsaved changes' : loaded ? 'Saved' : '');
    const open = $('ed-open');
    open.hidden = !loaded;
    if (loaded) open.href = `panels/${loaded.id}`;
    // A panel with its own API client has no link without its key (Share, Link with key).
    const copy = $('ed-copylink');
    copy.hidden = !loaded || loaded.acts === 'api';
  }

  // The Panel tab's Rows and Columns call this; a bad value keeps the current size.
  function setSize(which, value) {
    const rows = which === 'rows' ? value : g.rows;
    const cols = which === 'cols' ? value : g.cols;
    const lost = cellsLost(g, rows, cols);
    setGrid(resize(g, rows, cols));
    say('ed-status', lost ? `Removed ${lost} cell${lost > 1 ? 's' : ''} that no longer fit.` : '');
    refresh();
  }
  nameBox.addEventListener('input', refresh);
  $('ed-next').addEventListener('click', () => {
    setGrid(nextTodo(g));
    $('ed-grid').focus();
  });

  // ---- panels

  function drawPicker() {
    const pick = $('ed-pick');
    const { mine, shared } = groupPanels(list);
    const group = (label, panels) => {
      if (!panels.length) return null;
      const og = el('optgroup');
      og.label = label;
      for (const p of panels) {
        const o = el('option', '', p.name);
        o.value = p.id;
        og.append(o);
      }
      return og;
    };
    const fresh = el('option', '', 'New panel');
    fresh.value = '';
    pick.replaceChildren(...[group('My panels', mine), group('Shared with me (use only)', shared), fresh].filter(Boolean));
    pick.value = loaded?.id || '';
  }

  async function loadList() {
    list = (await edCall('The panel list could not be read')('api/v1/panels')).panels || [];
    drawPicker();
  }

  function show(panel) {
    loaded = panel;
    edits = {};
    art = panel ? createArt(sessionFetch, panel.id) : null;
    g = createGrid(panel || newPanel());
    nameBox.value = (panel || newPanel()).name;
    discardAsked = '';
    say('ed-status', '');
    $('ed-pick').value = panel?.id || '';
    document.title = panel ? `${panel.name} (editor)` : 'Sode Panel Editor';
    refresh();
    for (const fn of changeWatchers) fn(g);
    fieldsChanged();
  }

  async function openPanel(id, push) {
    if (!id) {
      if (push) history.pushState(null, '', 'panels/editor?new');
      return show(null);
    }
    let panel;
    try {
      panel = await edCall('The panel could not be opened')(`api/v1/panels/${id}`);
    } catch (e) {
      if (e.status === undefined) throw e;
      say('ed-status', e.status === 404 ? 'No panel here that you can use.' : e.message);
      drawPicker();
      return;
    }
    if (push) history.pushState(null, '', `panels/editor?panel=${id}`);
    show(panel);
  }

  $('ed-pick').addEventListener('change', (e) => {
    const id = e.target.value;
    if (!readOnly() && dirty() && discardAsked !== id) {
      discardAsked = id;
      e.target.value = loaded?.id || '';
      say('ed-status', 'This panel has unsaved changes. Choose again to discard them.');
      return;
    }
    openPanel(id, true);
  });
  window.addEventListener('popstate', () => openPanel(editorTarget(location.search).id, false));
  window.addEventListener('beforeunload', (e) => {
    if (!readOnly() && dirty()) e.preventDefault();
  });

  async function save() {
    $('ed-save').disabled = true;
    say('ed-status', 'Saving.');
    const body = JSON.stringify(saveBody(loaded || newPanel(), nameBox.value, g, edits));
    let saved;
    try {
      saved = await edCall('The panel was not saved', 'Sode did not answer, so the panel was not saved.')(
        loaded ? `api/v1/panels/${loaded.id}` : 'api/v1/panels',
        {
          method: loaded ? 'PUT' : 'POST',
          headers: { 'Content-Type': 'application/json' },
          body,
        },
      );
    } catch (e) {
      say('ed-status', e.message);
      refresh();
      return e.message;
    }
    const made = !loaded;
    loaded = saved;
    edits = {};
    if (made) history.replaceState(null, '', `panels/editor?panel=${loaded.id}`);
    nameBox.value = loaded.name;
    say('ed-status', 'Saved.');
    await loadList().catch(() => {});
    refresh();
    fieldsChanged();
    return '';
  }
  $('ed-save').addEventListener('click', save);

  // The live panel is the saved panel, and it leaves unassigned cells out (panelrules.js cellsIn), so unsaved
  // edits are saved first. Open panels follow a save by themselves.
  $('ed-live').addEventListener('click', async () => {
    if (!readOnly() && dirty()) await save();
    if (loaded && !dirty()) window.open(`panels/${loaded.id}`, '_blank');
  });

  $('ed-copylink').addEventListener('click', async () => {
    say('ed-status', (await copyText(linkFor(loaded))) ? 'Link copied.' : 'Copy failed. Use the Link field in Share instead.');
  });

  $('ed-copy').addEventListener('click', async () => {
    let copy;
    try {
      copy = await edCall('The copy was not saved', 'Sode did not answer, so no copy was made.')(`api/v1/panels/${loaded.id}/copy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      });
    } catch (e) {
      return say('ed-status', e.message);
    }
    history.pushState(null, '', `panels/editor?panel=${copy.id}`);
    await loadList().catch(() => {});
    show(copy);
    say('ed-status', 'Saved a copy. You own it and can change it.');
  });

  // ---- start

  $('panels-signin').addEventListener('click', signInAgain);
  onSessionKey((k, step) => {
    $('panels-signin').hidden = step !== 'sign-in';
    if (k) say('panels-note', '');
    else if (step === 'sign-in') say('panels-note', 'Sign in with your PIN to edit panels.');
    else if (step === 'no-users') say('panels-note', 'Panels need a user, and Sode has none.');
  });
  startSession();
  show(null);
  try {
    await loadList();
    names = namesFrom(await sessionFetch('api/v1/names').then((r) => (r.ok ? r.json() : {})));
    await openPanel(editorTarget(location.search).id, false);
  } catch (err) {
    say('panels-note', err.message);
  }

  // Seams for later tickets: Find adds menu items, the sidebars and live update read and replace the grid.
  return {
    grid: () => g,
    setGrid,
    setSize,
    panel: () => loaded,
    readOnly,
    // Saves now; answers '' when stored, else the refusal text.
    save,
    names: () => names,
    addMenuSection: (fn) => menuSections.push(fn),
    addKeyHook: (fn) => keyHooks.push(fn),
    onChange: (fn) => changeWatchers.push(fn),
    // The sidebar's panel fields. onFields runs when the panel loads, saves or a field changes.
    fields,
    art: () => art,
    setFields(patch) {
      edits = { ...edits, ...patch };
      refresh();
      fieldsChanged();
    },
    // A panel stored elsewhere (the Share modal): the grid and unsaved fields stay as they are.
    replaceLoaded(panel) {
      loaded = panel;
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
