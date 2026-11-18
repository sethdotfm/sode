// The Buttons panel body of a panel run (panelrun.js): cells, the state model, the Undo cell and the hold guard.
// ctx is the run's adapter: { ui, art, embedded, page, id, call, callJSON, post, poll, ready, instance, touched }.
import { envFrom, namesFrom, drawnCells, errorText, undoneText, undoMoved } from './panelrules.js';
import {
  createState,
  pressDestination,
  pressSource,
  pressPreset,
  clear,
  startTake,
  startInstant,
  finishTake,
  failTake,
  pressUndo,
  finishUndo,
  applyTally,
  carryView,
  showPage,
  pressPage,
  errorOf,
  undoneOf,
  guardDown,
  guardHeld,
  guardCancel,
  GUARD_HOLD_MS,
} from './panelmodel.js';
import { buildGrid, paintGrid } from './panelview.js';
import { withDefault, defaultsBy } from './panellook.js';
import { pagesOf } from './panelpages.js';
import { NAV_KINDS, landingPage, pageIndex, wrapOf } from './panelnav.js';
import { savedPage, keepPage } from './pagekeep.js';

const UNDO_MOVED = 'What Undo would undo has changed; press it again';

const say = (n, text) => {
  n.textContent = text;
  n.hidden = !text;
};

// The current Page's cells; a live update or a Page change makes them again.
const cellsOf = (p, page) => drawnCells(p, pageIndex(pagesOf(p), page)).map((c) => withDefault(c, defaultsBy(p.artDefaults)));
// Every Page's cells, so a live update keeps a Destination viewed on another Page.
const everyCell = (p) => pagesOf(p).flatMap((_, i) => drawnCells(p, i));
const stateOf = (p) => {
  const set = p.settings || {};
  return createState({
    mode: set.mode,
    multi: set.multi,
    guard: set.guard,
    resultText: set.resultText,
    levels: Array.isArray(set.levels) && set.levels.length ? set.levels : null,
    fixedDests: set.fixedDests,
  });
};
const hasUndo = (p) => pagesOf(p).some((_, i) => drawnCells(p, i).some((c) => c.kind === 'undo'));

export function buttonsBody(ctx, first) {
  const { ui, art, embedded, id } = ctx;
  let panel = first;
  let holdTimer = 0;
  let stopped = false;
  // The Live Editor's run is no instance, so it neither reads nor writes the browser's saved Page.
  let st = showPage(stateOf(panel), landingPage(pagesOf(panel), embedded ? ctx.page : savedPage(id)));
  let cells = cellsOf(panel, st.page);
  const notePage = () => {
    if (!embedded) keepPage(id, st.page);
  };
  notePage();
  let env = { levels: [], tally: new Map(), protectedDsts: new Set() };
  // What env was built from. withEtag gives the same reply objects on a 304, so an unchanged poll keeps env.
  let envOf = [];
  let names = namesFrom({});
  let presets = {};
  // The take this instance's Undo cell would undo (GET /panels/<id>/undo), asked for only while a Page has the cell.
  let undoNext;
  let undoCells = hasUndo(panel);

  const render = () => {
    const now = Date.now();
    paintGrid(grid, { st, env, names, now, panelName: panel.name, instance: ctx.instance(), pages: pagesOf(panel), wrap: wrapOf(panel), undo: undoNext });
    const err = errorOf(st, now);
    const undone = err ? null : undoneOf(st, now);
    say(ui.status, err ? errorText(err, names) : undoneText(undone, names));
    ui.status.classList.toggle('sode-panel__status--ok', !!undone && !undone.failed.length);
  };
  const build = () => buildGrid(ui.grid, panel, cells, onPress, { down: holdDown, cancel: holdCancel }, art);
  let grid = build();
  // Painted now, not at the first poll: an unpainted Take cell is enabled and the cells have no text.
  render();

  // A poll and a finished take can both ask; only the newest ask's answer counts, since an older one can come back last.
  let undoAsk = 0;
  async function askUndo() {
    if (!undoCells || stopped) return;
    const ask = ++undoAsk;
    try {
      const next = (await ctx.callJSON(`api/v1/panels/${id}/undo`)).undo;
      if (ask !== undoAsk) return;
      undoNext = next;
      // History's own Undo, another tab or a rename moved the target: Take would only get 409.
      if (st.undoId !== null && !st.busy && undoNext?.id !== st.undoId) st = failTake(st, 'conflict', UNDO_MOVED, Date.now());
    } catch (e) {
      // A refusal (a key without the take scope) greys the cell; a lost link keeps what it knew.
      if (ask === undoAsk && e.status !== undefined) undoNext = undefined;
    }
    render();
  }

  // A staged undo goes to its own route with the id it was staged from, so the server marks that take undone.
  const takeOpts = (started) =>
    started.undoId !== null
      ? { path: `api/v1/panels/${id}/undo`, body: { id: started.undoId }, finish: finishUndo, word: 'undo', onConflict: UNDO_MOVED }
      : { path: 'api/v1/takes', body: { routes: started.routes }, finish: finishTake, word: 'take' };

  async function send(started) {
    if (!started) return;
    const { path, body, finish, word, onConflict } = takeOpts(started);
    st = started.state;
    render();
    try {
      // Read st after the await: presses made while the POST is out change it.
      const reply = await ctx.post(path, body);
      st = finish(st, reply || {}, Date.now());
      // History moved, so the old answer names an older take; Undo greys until askUndo answers.
      undoNext = undefined;
    } catch (e) {
      const why = (e.code === 'conflict' && onConflict) || e.message;
      st =
        e.status === undefined
          ? failTake(st, 'network', `Sode did not answer, so it is possible that the ${word} was not sent`, Date.now())
          : failTake(st, e.code || String(e.status), why || `The ${word} was refused`, Date.now());
    }
    render();
    ctx.poll();
    askUndo();
  }

  function holdCancel() {
    clearTimeout(holdTimer);
    st = guardCancel(st);
    render();
  }
  function holdDown() {
    if (!ctx.ready() || st.guard !== 'hold' || !startTake(st, env)) return;
    st = guardDown(st, Date.now()).state;
    clearTimeout(holdTimer);
    holdTimer = setTimeout(() => {
      const held = guardHeld(st, Date.now());
      st = held.state;
      if (held.go) send(startTake(st, env));
      render();
    }, GUARD_HOLD_MS);
    render();
  }

  function onPress(cell) {
    // A Page change sends nothing, so it works while the router or Sode is away.
    const nav = NAV_KINDS.includes(cell.kind);
    if (!ctx.ready() && !nav) return;
    ctx.touched();
    if (nav) return showNewPage(pressPage(st, cell, pagesOf(panel), wrapOf(panel)));
    const now = Date.now();
    if (cell.kind === 'take') {
      if (st.guard === 'hold' || !startTake(st, env)) return;
      const down = guardDown(st, now);
      st = down.state;
      if (down.go) send(startTake(st, env));
      return render();
    }
    if (cell.kind === 'undo') st = pressUndo(st, undoNext, env, now);
    else if (cell.kind === 'destination') st = pressDestination(st, cell);
    else if (cell.kind === 'source') st = pressSource(st, cell, env, now);
    else if (cell.kind === 'preset') st = pressPreset(st, cell, env, now);
    else if (cell.kind === 'clear') st = clear(st);
    else return;
    // Instant mode: a Source, Preset or Undo press is the Confirm.
    if (cell.kind === 'source' || cell.kind === 'preset' || cell.kind === 'undo') {
      const started = startInstant(st, env);
      if (started) return void send(started);
    }
    render();
  }

  // A Page change keeps the plan and rebuilds the grid from the new Page's cells.
  function showNewPage(next) {
    if (next === st) return;
    clearTimeout(holdTimer);
    st = next;
    notePage();
    cells = cellsOf(panel, st.page);
    grid = build();
    render();
  }

  return {
    render,
    // One poll's answers. nm and pr are null when names were not read again; first is the first poll that reached Sode.
    data({ tally, prot, nm, pr, first }) {
      if (nm) {
        names = namesFrom(nm);
        presets = pr;
      }
      const was = env;
      const from = [tally, prot, presets, panel.rights];
      if (from.some((x, i) => x !== envOf[i])) env = envFrom(...from);
      envOf = from;
      st = applyTally(st, env);
      // Each ask walks History, so none runs on a timer; it is possible that History moved while Sode was out of reach.
      if (undoMoved(was, env, undoNext) || first) askUndo();
    },
    pending: () => ({ staged: st.staged.length > 0, busy: st.busy }),
    // A newer saved copy: rebuild the grid and the state from it, and keep the viewed destination.
    apply(p) {
      clearTimeout(holdTimer);
      panel = p;
      // The current Page stays unless the edit disabled or deleted it, then the first enabled Page shows.
      st = showPage(carryView(stateOf(p), st, everyCell(p)), landingPage(pagesOf(p), st.page));
      notePage();
      cells = cellsOf(p, st.page);
      undoCells = hasUndo(p);
      askUndo();
      grid = build();
      render();
    },
    renamed(changed) {
      render();
      if (changed) askUndo();
    },
    stop() {
      stopped = true;
      clearTimeout(holdTimer);
    },
  };
}
