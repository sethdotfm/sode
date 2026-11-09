// Runs /panels/editor: the panel picker, the toolbar, the grid and the smart menu, over /api/v1/panels as
// the signed-in user. The grid rules are gridmodel.js and the page text is editorrules.js; this file wires
// them to the DOM and the network. runEditor returns the seams later tickets build on (see the end).
import { $, el } from './core.js';
import { sessionFetch, startSession, signInAgain, onSessionKey } from './sessionkey.js';
import { namesFrom } from './panelrules.js';
import {
  ROWS_MAX,
  COLS_MAX,
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
  countUnassigned,
  nextUnassigned,
} from './gridmodel.js';
import { editorTarget, groupPanels, newPanel, saveBody, isDirty, kindChoices } from './editorrules.js';
import { createEditorGrid } from './editorview.js';
import { createSmartMenu } from './editormenu.js';

const say = (id, text) => {
  const n = $(id);
  n.textContent = text;
  n.hidden = !text;
};

async function failText(res, what) {
  const body = await res.json().catch(() => ({}));
  return body.error?.message || `${what} (${res.status})`;
}

export async function runEditor() {
  let g = createGrid();
  let loaded = null; // the stored panel being edited; null for a new one
  let list = [];
  let names = namesFrom({});
  let discardAsked = ''; // the panel the user tried to leave unsaved; asking again discards
  const menuSections = []; // (grid) => items: where Find and later actions plug in
  const changeWatchers = [];
  const readOnly = () => !!loaded && !loaded.canEdit;

  const nameBox = $('ed-name');
  const dirty = () => isDirty(loaded, nameBox.value, g);

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

  function onKey(e) {
    if (readOnly() || e.ctrlKey || e.metaKey || e.altKey) return;
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
      { label: 'Merge', icon: 'combine', disabled: !canMerge(g), run: () => setGrid(merge(g)) },
      { label: 'Unmerge', icon: 'split', disabled: !canUnmerge(g), run: () => setGrid(unmerge(g)) },
      {
        label: 'Type',
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
    grid.paint(g, names, { readOnly: ro });
    menu.show(ro ? null : grid.selectionBox(), menuItems());
    $('ed-rows').value = String(g.rows);
    $('ed-cols').value = String(g.cols);
    for (const id of ['ed-rows', 'ed-cols', 'ed-rows-dec', 'ed-rows-inc', 'ed-cols-dec', 'ed-cols-inc', 'ed-name']) $(id).disabled = ro;
    const todo = countUnassigned(g);
    $('ed-todo').textContent = todo ? `${todo} unassigned` : 'All assigned';
    $('ed-next').disabled = ro || !todo;
    $('ed-save').disabled = ro || !dirty();
    $('ed-banner').hidden = !ro;
    say('ed-state', ro ? '' : dirty() ? 'Unsaved changes' : loaded ? 'Saved' : '');
    const open = $('ed-open');
    open.hidden = !loaded;
    if (loaded) open.href = `panels/${loaded.id}`;
  }

  function stepSize(which, delta) {
    const rows = which === 'rows' ? g.rows + delta : g.rows;
    const cols = which === 'cols' ? g.cols + delta : g.cols;
    typeSize(rows, cols);
  }
  function typeSize(rows, cols) {
    const lost = cellsLost(g, rows, cols);
    setGrid(resize(g, rows, cols));
    say('ed-status', lost ? `Removed ${lost} cell${lost > 1 ? 's' : ''} that no longer fit.` : '');
  }
  for (const [which, max] of [
    ['rows', ROWS_MAX],
    ['cols', COLS_MAX],
  ]) {
    $(`ed-${which}-dec`).addEventListener('click', () => stepSize(which, -1));
    $(`ed-${which}-inc`).addEventListener('click', () => stepSize(which, 1));
    $(`ed-${which}`).max = String(max);
    $(`ed-${which}`).addEventListener('change', (e) => {
      const v = e.target.value;
      typeSize(which === 'rows' ? v : g.rows, which === 'cols' ? v : g.cols);
    });
  }
  nameBox.addEventListener('input', refresh);
  $('ed-next').addEventListener('click', () => {
    setGrid(nextUnassigned(g));
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
    const res = await sessionFetch('api/v1/panels');
    if (!res.ok) throw new Error(await failText(res, 'The panel list could not be read'));
    list = (await res.json()).panels || [];
    drawPicker();
  }

  function show(panel) {
    loaded = panel;
    g = createGrid(panel || newPanel());
    nameBox.value = (panel || newPanel()).name;
    discardAsked = '';
    say('ed-status', '');
    $('ed-pick').value = panel?.id || '';
    document.title = panel ? `${panel.name} (editor)` : 'Sode Panel Editor';
    refresh();
    for (const fn of changeWatchers) fn(g);
  }

  async function openPanel(id, push) {
    if (!id) {
      if (push) history.pushState(null, '', 'panels/editor?new');
      return show(null);
    }
    const res = await sessionFetch(`api/v1/panels/${id}`);
    if (!res.ok) {
      say('ed-status', res.status === 404 ? 'No panel here that you can use.' : await failText(res, 'The panel could not be opened'));
      drawPicker();
      return;
    }
    if (push) history.pushState(null, '', `panels/editor?panel=${id}`);
    show(await res.json());
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
    const body = JSON.stringify(saveBody(loaded || newPanel(), nameBox.value, g));
    const res = await sessionFetch(loaded ? `api/v1/panels/${loaded.id}` : 'api/v1/panels', {
      method: loaded ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json' },
      body,
    }).catch(() => null);
    if (!res || !res.ok) {
      say('ed-status', res ? await failText(res, 'The panel was not saved') : 'Sode did not answer, so the panel was not saved.');
      return refresh();
    }
    const made = !loaded;
    loaded = await res.json();
    if (made) history.replaceState(null, '', `panels/editor?panel=${loaded.id}`);
    nameBox.value = loaded.name;
    say('ed-status', 'Saved.');
    await loadList().catch(() => {});
    refresh();
  }
  $('ed-save').addEventListener('click', save);

  $('ed-copy').addEventListener('click', async () => {
    const res = await sessionFetch(`api/v1/panels/${loaded.id}/copy`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' }).catch(
      () => null,
    );
    if (!res || !res.ok) return say('ed-status', res ? await failText(res, 'The copy was not saved') : 'Sode did not answer, so no copy was made.');
    const copy = await res.json();
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
    panel: () => loaded,
    readOnly,
    names: () => names,
    addMenuSection: (fn) => menuSections.push(fn),
    onChange: (fn) => changeWatchers.push(fn),
    refresh,
    status: (text) => say('ed-status', text),
  };
}
