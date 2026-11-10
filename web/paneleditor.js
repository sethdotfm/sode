// Runs /panels/editor: the panel picker, the toolbar, the grid and the smart menu, over /api/v1/panels as
// the signed-in user. The grid rules are gridmodel.js and the page text is editorrules.js; this file wires
// them to the DOM and the network. runEditor returns the seams later tickets build on (see the end).
import { $, el, on } from './core.js';
import { sessionFetch, jsonCall, startSession, signInAgain, onSessionKey } from './sessionkey.js';
import { namesFrom, fixedDestsOf, artLeftOut } from './panelrules.js';
import { defaultsBy } from './panellook.js';
import {
  createGrid,
  cellsLost,
  resize,
  select,
  deselect,
  selectAll,
  rectOf,
  selCells,
  setKind,
  clearSelection,
  canMerge,
  merge,
  canUnmerge,
  unmerge,
  setAside,
  putBack,
  heldThatFit,
} from './gridmodel.js';
import {
  editorTarget,
  groupPanels,
  newPanel,
  saveBody,
  isDirty,
  kindChoices,
  typeLabel,
  menuArea,
  saveChord,
  signInNote,
  openFailNote,
} from './editorrules.js';
import { linkFor } from './panellink.js';
import { copyText } from './panellinkrow.js';
import { createEditorGrid } from './editorview.js';
import { createArt } from './panelart.js';
import { createSmartMenu } from './editormenu.js';
import { gridCommand } from './gridchords.js';
import { createLive } from './editorlive.js';
import { copyBlock, blockText, readBlock, pastePlan, applyPaste, pasteNote } from './gridclip.js';
import { writeClip, createPasteReader } from './editorclip.js';

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
  let defaults = new Map(); // Default artwork by item (defaultsBy): drawn only, never put into g
  let trusted = false; // only a Trusted user can make a client, so a new panel starts as Both for them and Session for the rest
  on('access', (a) => (trusted = !!a.trusted && !!a.user));
  let edits = {}; // the sidebar's unsaved panel fields: {settings, acts, clientRights}, each once edited
  const paintWatchers = []; // (grid, selection box) after every repaint, for things that sit over the squares
  const readOnly = () => !!loaded && !loaded.canEdit;
  const fields = () => ({
    settings: edits.settings ?? loaded?.settings ?? {},
    acts: edits.acts ?? loaded?.acts ?? (trusted ? 'both' : 'session'),
    slug: edits.slug ?? loaded?.slug ?? '',
    clientRights: edits.clientRights,
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
  const droppedNote = (n) => (n ? `Removed ${n} Destination cell${n > 1 ? 's' : ''} set aside by Fixed destination, as other cells now fill the squares.` : '');
  const sayDropped = (n) => n && say('ed-status', droppedNote(n));
  const whole = () => putBack(g, held);

  const nameBox = $('ed-name');
  const dirty = () => isDirty(loaded, nameBox.value, whole(), edits);

  // fresh: next is a whole new layout (a template), so cells held from the old one do not come back.
  // Returns how many held cells it dropped.
  const setGrid = (next, { fresh = false } = {}) => {
    if (fresh) held = [];
    const { shown, dropped } = holdOrPutBack(next);
    sayDropped(dropped);
    if (shown === g) return dropped;
    g = shown;
    refresh();
    for (const fn of changeWatchers) fn(g);
    return dropped;
  };

  const grid = createEditorGrid($('ed-grid'), {
    select(r, c, extend) {
      if (readOnly()) return;
      menuAway = false;
      setGrid(select(g, r, c, extend)); // always a new grid, so the menu comes back
    },
    key: onKey,
  });
  const live = createLive($('ed-live-view'), document.querySelector('.sode-ed__body'), $('ed-live'));
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

  // The menu goes with the selection when a click or Tab lands outside the grid, the menu and the sidebar. The
  // sidebar edits the selected cell, so a click there keeps the selection and only puts the menu away (it sat over
  // the sidebar's fields); the next grid click or key brings it back. Not a blur: the menu rebuilds itself on every
  // change and drops focus for a moment. The event path, not the target: a click on a square repaints the grid,
  // which detaches that target.
  let menuAway = false;
  const leave = (e) => {
    if (!g.sel) return;
    const path = e.composedPath();
    if ([$('ed-grid'), menu.root].some((n) => path.includes(n))) return;
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
    if (!block) return say('ed-status', 'Nothing copied: the selection holds no cells.');
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
    if (g !== snap) return say('ed-status', 'Copied, but the grid changed meanwhile, so nothing was removed.');
    setGrid(clearSelection(g));
    say('ed-status', `Cut ${n} cell${n > 1 ? 's' : ''}.`);
  }

  // One paste is one setGrid, the step an undo wraps. Text that is not a Sode block changes nothing.
  function pasteText(text) {
    if (readOnly()) return;
    if (text === null) return say('ed-status', 'Nothing pasted: the browser did not let Sode read the clipboard.');
    const block = readBlock(text);
    if (!block) return say('ed-status', 'Nothing pasted: the clipboard holds no Sode cells.');
    const plan = pastePlan(g, block, { panel: loaded?.id, fixed: fixedOn() });
    if (!plan) return say('ed-status', 'Nothing pasted: select the square to paste at first.');
    if (!plan.fits) return pasteOverflow(plan);
    const dropped = setGrid(applyPaste(g, plan.placed));
    say('ed-status', `${pasteNote(plan)} ${droppedNote(dropped)}`.trim());
  }
  // A paste that runs past the grid's edge: the grid stays as it is and the status line says why. The Clip and
  // Wrap dialog takes over here.
  const pasteOverflow = (plan) => say('ed-status', pasteNote(plan));
  const paste = createPasteReader($('ed-grid'), pasteText);

  // One entry per command name in gridchords.js GRID_CHORDS. One that returns false leaves its key to the browser.
  const gridCommands = {
    selectAll: () => setGrid(selectAll(g)),
    copy: () => void copyCells(),
    cut: () => void cutCells(),
    paste: () => (paste.key(), false),
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
      // One item that toggles, so Ctrl+M merges a group and unmerges a merged square.
      canUnmerge(g)
        ? { chord: 'm', label: 'Unmerge', icon: 'split', run: () => setGrid(unmerge(g)) }
        : { chord: 'm', label: 'Merge', icon: 'combine', disabled: !canMerge(g), run: () => setGrid(merge(g)) },
      {
        label: typeLabel(cells),
        icon: 'type',
        sub: kindChoices({ fixed: fixedOn() }).map(({ kind, label, disabled, hint }) => ({
          label,
          disabled,
          hint,
          current: kind === '' ? !cells.length : cells.length > 0 && cells.every((c) => c.kind === kind),
          run: () => setGrid(setKind(g, kind)),
        })),
      },
    ];
    for (const section of menuSections) items.push(...section(g));
    return items;
  }

  // A key without the panels scope, or a failed read, leaves the defaults as they were.
  async function loadDefaults() {
    const r = await sessionFetch('api/v1/default-artwork').catch(() => null);
    if (!r?.ok) return;
    defaults = defaultsBy((await r.json()).defaults);
    refresh();
  }

  function refresh() {
    const ro = readOnly();
    grid.paint(g, names, { readOnly: ro, art, defaults });
    const box = ro ? null : grid.selectionBox();
    menu.show(menuAway ? null : box, menuItems());
    for (const fn of paintWatchers) fn(g, box);
    nameBox.disabled = ro;
    $('ed-save').disabled = ro || !dirty();
    $('ed-banner').hidden = !ro;
    // Not say(): the slot keeps its size whatever it holds, so the toolbar never re-wraps (panels.css).
    $('ed-state').textContent = ro ? '' : dirty() ? 'Unsaved changes' : loaded ? 'Saved' : '';
    const open = $('ed-open');
    open.hidden = !loaded;
    if (loaded) open.href = `panels/${loaded.id}`;
    // A panel that only acts as its API client has no link without its key (Share, Link with key); a Both panel's plain link opens in Session mode.
    const copy = $('ed-copylink');
    copy.hidden = !loaded || loaded.acts === 'api';
  }

  // The Panel tab's Rows and Columns call this; a bad value keeps the current size.
  function setSize(which, value) {
    const rows = which === 'rows' ? value : g.rows;
    const cols = which === 'cols' ? value : g.cols;
    const lost = cellsLost(g, rows, cols) + setGrid(resize(g, rows, cols));
    say('ed-status', lost ? `Removed ${lost} cell${lost > 1 ? 's' : ''} that no longer fit.` : '');
    refresh();
  }
  nameBox.addEventListener('input', refresh);

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
    list =
      (
        await edCall(
          'The panel list could not be read',
          'Sode did not answer, so the panel list could not be read. Check the network, then reload.',
        )('api/v1/panels')
      ).panels || [];
    drawPicker();
  }

  function show(panel) {
    live.stop();
    loaded = panel;
    edits = {};
    art?.revoke();
    art = panel ? createArt(sessionFetch, panel.id) : null;
    held = [];
    g = holdOrPutBack(createGrid(panel || newPanel())).shown;
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
      panel = await edCall('The panel could not be opened', '')(`api/v1/panels/${id}`);
    } catch (e) {
      if (e.status === undefined) throw e;
      say('ed-status', openFailNote(e));
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
    const body = JSON.stringify(saveBody(loaded || newPanel(), nameBox.value, whole(), edits));
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
    if (!readOnly() && dirty()) await save();
    if (loaded && !dirty()) live.start(loaded.id);
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
    say('ed-status', `Saved a copy. You own it and can change it. ${artLeftOut(copy.artSkipped)}`.trim());
  });

  // ---- start

  // The editor never sits blank: each way it cannot reach the panel puts a note and, where it helps, a button.
  let signInFix = signInAgain;
  let bootFail = ''; // why the panel list or panel did not load; a later sign-in event must not wipe it
  let lastStep = '';
  const note = (step, err) => {
    lastStep = step;
    const { text, button } = signInNote(step, err);
    say('panels-note', text || bootFail);
    const b = $('panels-signin');
    b.hidden = !button;
    b.textContent = button || 'Sign in';
    signInFix = step === 'ended' ? () => location.reload() : signInAgain;
  };
  $('panels-signin').addEventListener('click', () => signInFix());
  onSessionKey((k, step, err) => note(k ? 'ready' : step, err));
  on('wsclose', () => note('offline'));
  startSession();
  show(null);
  // Not awaited: the seams below must exist while the page still waits for a sign-in, so the sidebar and Find are
  // built and nothing is left half drawn.
  (async () => {
    try {
      await loadList();
      names = namesFrom(await sessionFetch('api/v1/names').then((r) => (r.ok ? r.json() : {})));
      await loadDefaults();
      await openPanel(editorTarget(location.search).id, false);
    } catch (err) {
      // An ended or refused key already has its note from onSessionKey.
      if (lastStep === 'ended' || lastStep === 'refused') return;
      bootFail = openFailNote(err.status === undefined ? { status: 0 } : err);
      say('panels-note', bootFail);
    }
  })();

  // Seams for later tickets: Find adds menu items, the sidebars and live update read and replace the grid.
  return {
    grid: () => g,
    // The grid as stored, with any Destination cells Fixed destination set aside.
    storedGrid: whole,
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
