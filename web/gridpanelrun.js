// The Grid panel body of a panel run (panelrun.js). It reads and sends only through ctx, the run's /api/v1 adapter.
// So another adapter can drive the same view. ctx is panelrun.js's: { ui, art, embedded, page, id, call, callJSON, post,
// poll, ready, instance, keyed, touched }.
// The rules are pure modules (gridpanel*.js); this file joins them to the DOM.
import { envFrom, namesFrom } from './panelrules.js';
import { arrange } from './gridpanelarrange.js';
import { nameArt } from './gridpanelart.js';
import { defaultsBy } from './panellook.js';
import {
  createStage,
  restage,
  stageRoutes,
  stagedCount,
  confirmRoutes,
  cancelConfirm,
  dropStaged,
  unstageRoute,
  startSend,
  afterTake,
  stageText,
} from './gridpanelstage.js';
import { createPicks, pickName, pickCount, dropPicks, diagonal, extendPick, dragPick, enterGrid } from './gridpanelpick.js';
import { stageLevels, offeredLevels, boxFill, pieOn } from './gridpanellevels.js';
import { otherSources, otherText, stageable, skippedText, refusalText, leftOut, keepLive, leftOutText, leftOutLine } from './gridpanelrights.js';
import { gridKeyAction, findBoxAction, clearAction } from './gridpanelkeys.js';
import { sourceJump, holderJump, jumpText } from './gridpaneljump.js';
import { stageUndo, leaveUndo, afterUndo, undoText } from './gridpanelundo.js';
import { createFind, typed, currentMatch, stepMatch, switchMode, clearFind, stepBox } from './gridpanelfind.js';
import { findBoxes } from './gridpanelfindbox.js';
import { planOf } from './gridpanelplan.js';
import { gridConfirm } from './gridpanelconfirm.js';
import { buildGridPanel, joinPulse } from './gridpanelview.js';
import { pinsOf } from './gridpanelwindow.js';
import { CORNER, MOVES, moveCursor, pointOf, cursorAt, barOf, rangeCursor, rangeKind } from './gridpanelcursor.js';
import { isLine, linePairs, lineShown, lineStage } from './gridpanelline.js';
import { lineGestures } from './gridpanelgesture.js';
import { levelTextOf, parseLevelsIn } from './levels.js';
import { levelBox } from './levelbox.js';
import { el, icon } from './core.js';
import { editHref } from './panelsrules.js';
import { plural } from './gridmodel.js';
import { shortcutList } from './shortcuts.js';
import { restPoint, takeButton, tapMods, pickNote } from './gridpanelphone.js';
import { clearNote } from './gridpanelnote.js';
import { onLayout } from './layout.js';
import { gridHead } from './gridpanelheadview.js';
import { emptyText } from './gridpanelhead.js';
import { gridPosition } from './gridpanelpos.js';

const NOTE_MS = 4000;
const UNDO_MOVED = 'What Undo would undo has changed; press u again';
// Like the moves, these work in an embedded run only with focus in the grid.
const FIND_KEYS = new Set(['findSrcs', 'findDsts', 'nextMatch', 'prevMatch']);
const sameList = (a, b) => a.length === b.length && a.every((x, k) => x === b[k]);
const sameAxes = (a, b) => sameList(a.srcs, b.srcs) && sameList(a.dsts, b.dsts);

const say = (n, text) => {
  if (n.textContent !== text) n.textContent = text;
  n.hidden = !text;
};

// A focused field or header button keeps its own Enter; the crosspoints are the grid's, the search boxes are not.
const inControl = (grid) => {
  const a = document.activeElement;
  const own = grid.contains(a) && !a.closest('.sode-gridpanel__find');
  return !!a && a !== document.body && !own && a.matches('button, input, select, textarea, a[href], [contenteditable]');
};

export function gridBody(ctx, first) {
  const { ui } = ctx;
  let panel = first;
  let st = createStage({ mode: panel.settings?.mode });
  let tally = null;
  let env = envFrom({}, {});
  let names = namesFrom({});
  let namesReply = {};
  let drawn = null; // the { full, axes } the view was last given; null makes the next rebuild redraw
  // The last tally and protected replies, and names reply, as text. A 304 replays the same text, so a poll that
  // changed nothing does no work and the tick paints only the status line.
  let dataSig = '';
  // The reply objects behind dataSig. withEtag gives the same objects on a 304, so the poll skips the stringify.
  let dataOf = null;
  let namesSig = '';
  let dirty = true;
  // Picks and other sources use full (gridAxes). The grid shows axes: full less what a Filter hides.
  let axes = { srcs: [], dsts: [] };
  let full = { srcs: [], dsts: [] };
  // A load starts with no search. A live update keeps the search.
  let find = createFind();
  let matches = { srcs: [], dsts: [] }; // findMatches for each box, best first
  let matched = { srcs: new Set(), dsts: new Set() }; // the same, for the paint of each drawn name
  let boxes = null;
  let shownSrcs = new Set();
  let out = []; // staged routes left out (gridpanelrights.js), as of the last render
  let g = null;
  let picks = createPicks();
  let toldPicks = null;
  let at = null; // the crosspoint under the cursor, whose press the preview shows
  let cur = CORNER;
  // After a pointer move a scroll moves the cursor to what is under the pointer; after a key it does not.
  let ptr = null; // { x, y } in client px while the pointer leads, else null.
  let keyed = false; // true after a key move: only then can the cursor draw its box on a bar.
  let line = null; // A Drawn line { a, b, keys } in crosspoint units (gridpanelline.js). keys is true for a v line.
  let gest = null;
  let pos = null; // the phone's position strips (gridpanelpos.js)
  // "N staged" in the header: staged routes can be scrolled out of view.
  const count = el('span', 'pb-badge pb-badge--info sode-gridpanel__count');
  count.id = 'panel-staged';
  count.hidden = true;
  // The panel page's header has its own boxes (gridpanelheadview.js); the editor's live test keeps its own.
  const head = ctx.embedded || !ui.name ? null : gridHead(ui, count, { onCopy: copyPanel });
  head?.access(first, !ctx.keyed?.());
  if (!head) ui.head?.querySelector('.sode-panels__spacer')?.before(count);
  let note = { text: '', at: 0 }; // a result or refusal, shown for NOTE_MS
  const noteOf = (text, ok = false) => ({ text, at: Date.now(), ok });
  const phone = () => document.documentElement.dataset.layout === 'phone';
  // The Take button (gridpanelphone.js) at the end of the Levels box, since Kiosk mode hides the header, not the box.
  // It is the main page's red Take. An embedded run has neither.
  const take = ctx.embedded ? null : el('button', 'pb-btn pb-btn--danger sode-panel__take sode-gridpanel__take', 'Take');
  if (take) {
    take.type = 'button';
    take.id = 'panel-take';
    take.hidden = true;
    take.addEventListener('click', () => openConfirm());
    ui.levels?.append(take);
  }
  const clearer = take?.parentNode ? clearNote(take.parentNode, take, () => clearStep(clearAction(clearCtx()))) : null;

  // Artwork images load with the panel's key, so a name paints without one until its fetch lands, then paints again.
  const images = new Map(); // artwork id -> { url, type }, or null while it loads or after it failed
  let stopped = false;
  const imageOf = (id) => {
    if (!images.has(id)) {
      images.set(id, null);
      ctx.art?.look(id).then((l) => {
        if (!l || stopped) return;
        images.set(id, l);
        render();
      });
    }
    return images.get(id);
  };
  let defaultsFor = null; // the artDefaults list that defaults was made from
  let defaults = new Map();
  function nameArtOf(kind, n) {
    if (defaultsFor !== panel.artDefaults) {
      defaultsFor = panel.artDefaults;
      defaults = defaultsBy(defaultsFor);
    }
    const art = kind === 'srcs' ? nameArt(panel.sources, 'source', n, defaults) : nameArt(panel.destinations, 'destination', n, defaults);
    if (!art) return null;
    const out = { b: art.b };
    if (art.hide) out.hide = true;
    if (art.a?.icon) out.icon = art.a.icon;
    else if (art.a) {
      const l = imageOf(art.a.id);
      if (l) out[art.a.lib && l.type.startsWith('image/svg+xml') ? 'mask' : 'url'] = l.url;
    }
    return out;
  }

  // What the last polls said, for the staged routes that no longer apply.
  const world = () => ({ env, rights: panel.rights, names: namesReply });
  const staged = (s, d) => st.staged.filter((x) => x.src === s && x.dst === d).map((x) => x.level);
  const view = {
    get names() {
      return names;
    },
    get levels() {
      return tally?.levels || [];
    },
    levelText: (list) => levelTextOf(list, tally?.levels || [], tally?.levelNumbering),
    pie: (s, d) => pieOn(env, s, d),
    staged,
    held: (d) => st.staged.some((x) => x.dst === d),
    picked: (kind, n) => picks[kind].includes(n),
    // Filter shows only matches, so only Find marks them.
    found: (kind, n) => find.mode === 'find' && matched[kind].has(n),
    leftOut: (s, d) => out.some((r) => r.src === s && r.dst === d),
    protected: (d) => env.protectedDsts.has(d),
    art: (kind, n) => nameArtOf(kind, n),
    other(d) {
      const o = otherSources(env, d, shownSrcs);
      return { text: otherText(o, names), tip: o.length ? `Not on this panel: ${o.map((x) => names.source(x)).join(', ')}` : '' };
    },
  };

  // Picks hidden by a Filter still count; the downward run uses what shows.
  const diagonalAt = (xp) => diagonal(picks, xp, { srcs: full.srcs, dsts: full.dsts, shownDsts: axes.dsts });
  // A Diagonal skips protected rows.
  const pairsAt = (xp) => stageable(diagonalAt(xp), env).pairs;

  // What release would do: the routes numbered, protected rows skipped, red past the cap.
  const lineDry = () => {
    const pairs = isLine(line.a, line.b) ? linePairs(picks, line.a, line.b, axes, full) : [];
    const r = lineStage(st, pairs, env, pressLevels());
    const skip = new Set(r.skipped);
    return { pairs, kept: pairs.filter((x) => !skip.has(x)), r };
  };

  // Numbered targets only while names are picked; a press with none is the crosspoint itself. A phone has no hover,
  // so with no crosspoint under the cursor the targets show from the picks' rest point.
  const preview = () => {
    if (line) {
      const { kept, r } = lineDry();
      g.line(lineShown(picks, line.a, line.b, axes));
      return g.preview(kept, r.over, r.skipped);
    }
    g.line(null);
    const from = at || (phone() && restPoint(picks, axes, cur));
    const pairs = from && pickCount(picks) ? pairsAt(from) : [];
    g.preview(pairs, pairs.length > 1 && stageRoutes(st, pairs, pressLevels()).st.error?.code === 'cap');
  };

  const render = () => {
    paintLevels();
    dirty = false;
    out = leftOut(st, world());
    if (g) {
      g.paint(view);
      preview();
    }
    const n = stagedCount(st);
    say(count, n ? `${n} staged` : '');
    if (take) {
      const t = takeButton({ mode: st.mode, staged: n, busy: st.busy });
      if (take.hidden === t.shown) take.hidden = !t.shown;
      if (take.disabled !== t.disabled) take.disabled = t.disabled;
      const starts = t.armed && !take.classList.contains('sode-panel__take--armed');
      take.classList.toggle('sode-panel__take--armed', t.armed);
      if (starts) joinPulse(take);
    }
    paintEmpty();
    clearer?.paint(pickNote({ srcs: picks.srcs.length, dsts: picks.dsts.length, staged: n }), !!clearAction(clearCtx()));
    // The Live Editor only (editorlive.js): the picks are also the lists' selection. picks is replaced on every change.
    if (ui.onPicks && picks !== toldPicks) ui.onPicks((toldPicks = picks));
    boxes?.paint(find, countsOf());
    status();
  };
  // A panel with no items on an axis has no crosspoints, so the grid says so (pb-empty) under the corner.
  let empty = null;
  function paintEmpty() {
    const t = g && emptyText({ srcs: full.srcs.length, dsts: full.dsts.length, canEdit: panel.canEdit && !ctx.embedded });
    if (!t) {
      if (empty) empty.node.hidden = true;
      return;
    }
    if (!empty) {
      const title = el('p', 'pb-empty__title', document.createTextNode(''));
      const msg = el('p', 'pb-empty__msg', document.createTextNode(''));
      const edit = el('a', 'pb-btn pb-btn--outline pb-btn--sm', icon('pencil'), 'Edit panel');
      edit.href = editHref(ctx.id);
      const node = el('div', 'pb-empty sode-gridpanel__empty', title, msg, el('div', 'pb-empty__actions', edit));
      node.setAttribute('role', 'status');
      g.scroll.append(node);
      empty = { node, title, msg, edit };
    }
    say(empty.title, t.title);
    say(empty.msg, t.msg);
    // In the Live Editor (ui.back) the editor is already open, so the note has no link.
    empty.edit.hidden = !t.edit || !!ui.back;
    empty.node.hidden = false;
  }
  // Results and errors fade on the clock, so the status line is what the tick paints each time. On the panel page's
  // desktop layout they show in the banner (gridpanelheadview.js). A note is new by its time. A state's text goes when
  // the state clears.
  const status = () => {
    const held = stageText(st.error) || refusalText(st.error, names) || undoText(st.error) || leftOutText(out, names);
    const fresh = Date.now() - note.at < NOTE_MS && note.text;
    const text = held || fresh || '';
    const ok = !st.error && !!text && note.ok;
    const behind = held && fresh && fresh !== held ? `${note.at}` : undefined;
    const banner = head?.say({ text, kind: ok ? 'ok' : 'error', key: held || `${note.at}`, held: !!held, behind });
    say(ui.status, banner ? '' : text);
    ui.status.classList.toggle('sode-panel__status--ok', ok);
  };
  // The page's Copy: the same copy as /panels and the editor's Save a copy, then the editor opens on it.
  async function copyPanel() {
    try {
      const made = await ctx.callJSON(`api/v1/panels/${ctx.id}/copy`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
      location.assign(editHref(made.id));
    } catch (e) {
      note = noteOf(e.status === undefined ? 'Sode did not answer, so no copy was made.' : e.message || 'The copy was not saved');
      render();
    }
  }
  // From the run after each poll and every second.
  const tick = () => (dirty ? render() : status());

  // The view moves its nodes to the new items only when the shown sources or destinations change.
  const rebuild = () => {
    const next = arrange(panel, namesReply, { find, picks, cur, axes });
    matches = next.matches;
    matched = { srcs: new Set(matches.srcs), dsts: new Set(matches.dsts) };
    if (drawn && sameAxes(drawn.full, next.full) && sameAxes(drawn.axes, next.axes)) return;
    drawn = { full: next.full, axes: next.axes };
    ({ cur, axes, full, picks } = next);
    // A line is in display order, so new axes end it.
    if (line) {
      gest?.cancel();
      line = null;
    }
    shownSrcs = new Set(full.srcs);
    at = null;
    const all = { ...axes, srcPins: pinsOf(panel.sources, axes.srcs), dstPins: pinsOf(panel.destinations, axes.dsts) };
    if (g) g.setAxes(all);
    else {
      g = buildGridPanel(ui.grid, all, { onPress, onName, onPoint, onJump });
      g.onDraw(() => ptr && pointAt(ptr.x, ptr.y));
      if (head) pos = gridPosition(g, g.scroll.parentNode, phone);
      boxes = findBoxes(head?.finds || g.head, { onText: findText, onMode: findMode, onStep: findStep });
      gest = lineGestures(g, {
        start: lineStart,
        move: lineMove,
        end: lineEnd,
        cancel: dropLine,
        name: onTouchName,
        span: onSpan,
        spanEnd: () => (spanBase = null),
      });
    }
    setCursor(cur);
  };

  const countsOf = () => {
    const c = (k) => ({ n: matches[k].length, at: matches[k].indexOf(currentMatch(find, k, matches[k])) + 1 });
    return { srcs: c('srcs'), dsts: c('dsts') };
  };

  // Puts the cursor on the current match of a box, on the same row or column, and scrolls to it.
  function toMatch(kind) {
    const k = axes[kind].indexOf(currentMatch(find, kind, matches[kind]));
    if (k < 0 || !g) return;
    ptr = null;
    setCursor(kind === 'srcs' ? { i: cur.i, j: k } : { i: k, j: cur.j });
    g.glide(kind, k);
  }

  function findText(kind, text) {
    find = typed(find, kind, text);
    rebuild();
    toMatch(kind);
    render();
  }

  function findMode() {
    find = switchMode(find);
    rebuild();
    render();
  }

  function findStep(kind, dir) {
    if (!kind || !find[kind].text.trim()) return;
    find = stepMatch(find, kind, matches[kind], dir);
    toMatch(kind);
    render();
  }

  function findBox(kind, act) {
    if (act === 'nextMatch') findStep(kind, 1);
    else if (act === 'prevMatch') findStep(kind, -1);
    else if (act === 'mode') findMode();
    else if (act === 'swap') boxes.input(kind === 'srcs' ? 'dsts' : 'srcs').focus();
    else if (act === 'clear') {
      find = clearFind(find, kind);
      rebuild();
      render();
      // Back to the cursor. An embedded run takes its keys only with focus in the grid.
      g.scroll.focus({ preventScroll: true });
    }
  }

  // plan is what the confirm showed, or what Take sends with the box off (left: what it left out). An Undo goes to
  // its own route with the id it was staged from, so the server marks that take undone.
  async function send(plan) {
    const { routes, undoId } = plan;
    const undo = undoId !== undefined;
    const word = undo ? 'undo' : 'take';
    // An Undo answer asked before this send names the take before it.
    undoAsk++;
    st = startSend(st);
    render();
    let failed = null;
    try {
      const reply = await (undo ? ctx.post(`api/v1/panels/${ctx.id}/undo`, { id: undoId }) : ctx.post('api/v1/takes', { routes }));
      const bad = (reply?.results || []).filter((r) => !r.ok);
      const n = (reply?.results || routes).length;
      note = bad.length
        ? noteOf(`${undo ? 'Undo' : 'Take'} failed on ${[...new Set(bad.map((r) => names.destination(r.dst)))].join(', ')}`)
        : noteOf(`${undo ? 'Undone' : 'Taken'}: ${plural(n, 'route')}${plan.left ? `. ${plan.left}` : ''}`, !plan.left);
    } catch (e) {
      failed =
        e.status === undefined
          ? `Sode did not answer, so it is possible that the ${word} was not sent`
          : (undo && e.code === 'conflict' && UNDO_MOVED) || e.message || `The ${word} was refused`;
    }
    st = undo ? afterUndo(st, failed) : afterTake(st, failed);
    render();
    ctx.poll();
  }

  // Each ask walks History, so u asks once per press. Only the newest ask's answer counts, and only while nothing
  // else is in progress.
  let undoAsk = 0;
  async function pressUndo() {
    if (!confirm || st.busy || !ctx.ready()) return;
    ctx.touched();
    const ask = ++undoAsk;
    let next;
    let failed = null;
    try {
      next = (await ctx.callJSON(`api/v1/panels/${ctx.id}/undo`)).undo;
    } catch (e) {
      failed = e;
    }
    if (ask !== undoAsk || stopped || st.busy || confirm.isOpen()) return;
    if (failed) {
      note = noteOf(failed.status === undefined ? 'Sode did not answer' : failed.message || 'Undo is refused');
      return render();
    }
    const staged = st.undoId === null ? stagedCount(st) : 0;
    const tried = stageUndo(st, next, env);
    if (tried.error || !staged) return takeUndo(next);
    confirm.open({ verb: 'undo', replace: staged, next });
    render();
  }

  // Instant mode holds nothing staged, so its Undo confirm opens at once.
  function takeUndo(next) {
    st = stageUndo(st, next, env);
    render();
    if (!st.error && st.mode === 'instant') openConfirm();
  }

  // The Levels box (levelbox.js). An embedded run has none, so the panel's levels setting stands in.
  const boxNode = ui.levels;
  let boxKey = '';
  let filledKey = null;
  // The numbering the box text was typed in, so a change of Level numbering relabels it (levelbox.js reset).
  let typedIn = null;
  const boxSource = () =>
    tally && {
      levels: tally.levels,
      offered: offeredLevels(tally, panel.rights),
      numbering: tally.levelNumbering,
      defaultLevels: tally.defaultLevels || '',
      prev: typedIn,
    };
  const lvl = boxNode
    ? levelBox(document.getElementById('panel-lvl'), document.getElementById('panel-lvl-pick'), document.getElementById('panel-lvl-all'), boxSource, {
        hold: 'pointerdown',
        onChange: () => paintLevels(),
      })
    : null;
  if (boxNode) boxNode.hidden = false;

  // The picker is made again when the offered levels or the numbering change. The setting fills the box on load,
  // and again only when a live update changes the setting, so typed text survives other updates.
  function syncBox() {
    if (!lvl || !tally) return;
    const key = JSON.stringify([offeredLevels(tally, panel.rights).map((l) => l.level), tally.levels.length, tally.levelNumbering]);
    if (key !== boxKey) {
      boxKey = key;
      lvl.reset();
      typedIn = { mode: tally.levelNumbering, max: tally.levels.length };
    }
    const fk = JSON.stringify(panel.settings?.levels ?? null);
    if (fk === filledKey) return;
    filledKey = fk;
    const fill = boxFill(panel.settings, tally, panel.rights);
    if (fill) lvl.set(fill);
  }

  const paintLevels = () => head?.paintLevels(levelTextOf(pressLevels(), tally?.levels || [], tally?.levelNumbering));
  const pressLevels = () => {
    if (lvl) return stageLevels(lvl.levels(), tally, panel.rights);
    const src = boxSource();
    return src ? stageLevels(boxFill(panel.settings, tally, panel.rights) || parseLevelsIn('', src) || [], tally, panel.rights) : [];
  };

  const confirm = document.getElementById('confirm')
    ? gridConfirm({
        tally: () => tally,
        settings: () => panel.confirm,
        onTake: (plan) => (plan.replace ? takeUndo(plan.next) : send(plan)),
        onClose: () => {
          st = cancelConfirm(st);
          render();
        },
      })
    : null;

  // Routes that no longer apply are left out with the reason and dropped here. With the box off (st.ask false), this
  // is the Confirm and sends at once; Instant mode always asks.
  function openConfirm() {
    if (!confirm || st.busy) return;
    const ask = st.ask || st.mode === 'instant';
    // The server sends an Undo whole, so nothing is left out of it here.
    if (st.undoId !== null) {
      const routes = confirmRoutes(st);
      if (!ask) return send({ routes, undoId: st.undoId });
      return confirm.open({ ...planOf(routes, names, tally), verb: 'undo', routes, undoId: st.undoId });
    }
    const was = leftOut(st, world());
    st = keepLive(st, world());
    const routes = confirmRoutes(st);
    if (routes && !ask) return send({ routes, left: leftOutText(was, names) });
    if (routes) confirm.open({ ...planOf(routes, names, tally), routes, warnings: was.map((r) => leftOutLine(r, names)) });
    else if (was.length) note = noteOf(leftOutText(was, names));
    render();
  }

  function onPress(xp, e) {
    if (!ctx.ready() || st.busy || confirm?.isOpen()) return;
    if (e?.altKey) return unstageAt(xp);
    ctx.touched();
    st = keepLive(leaveUndo(st), world());
    const all = diagonalAt(xp);
    const { pairs, err } = stageable(all, env);
    if (err) {
      st = { ...st, error: err };
      return render();
    }
    const r = stageRoutes(st, pairs, pressLevels());
    st = r.st;
    const keep = new Set(pairs);
    const skipped = all.filter((x) => !keep.has(x));
    afterStage(r, st.error ? '' : skippedText(skipped, names));
  }

  // After a press or a line. skip names the protected rows it left out. An Instant send writes its own banner when
  // it answers, so skip goes with the send.
  function afterStage(r, skip) {
    if (r.send) return send({ routes: r.send, left: skip });
    if (skip) note = noteOf(skip);
    render();
    if (r.confirm) openConfirm();
  }

  // Alt-click or Delete on a staged crosspoint: only that route goes. Clear and Esc drop them all.
  function unstageAt(xp) {
    if (!view.staged(xp.src, xp.dst).length) return;
    ctx.touched();
    st = unstageRoute(leaveUndo(st), xp);
    render();
  }

  const canStage = () => ctx.ready() && !st.busy && !confirm?.isOpen();

  function lineStart(a) {
    if (!canStage()) return gest.cancel();
    ctx.touched();
    line = { a, b: a, keys: false };
    // A full render, so the Clear button can drop the line.
    render();
  }

  function lineMove(b) {
    if (!line) return;
    line = { ...line, b };
    preview();
  }

  function lineEnd(b) {
    if (!line) return;
    line = { ...line, b };
    stageLine();
  }

  function dropLine() {
    if (!line) return;
    line = null;
    render();
  }

  // Esc and the Clear button: act is clearAction's.
  function clearCtx() {
    return { line: !!line, picks: pickCount(picks), staged: st.staged.length };
  }
  function clearStep(act) {
    if (act === 'unline') {
      gest?.cancel();
      dropLine();
    } else if (act === 'unpick') {
      picks = dropPicks(picks);
      render();
    } else if (act === 'drop') {
      st = dropStaged(st);
      render();
    }
  }

  // Release, or Space in v mode. A line that ends on its start row stages nothing.
  function stageLine() {
    if (!canStage()) return dropLine();
    st = keepLive(leaveUndo(st), world());
    const { pairs, r } = lineDry();
    line = null;
    if (!pairs.length) return render();
    st = r.st;
    afterStage(r, st.error ? '' : skippedText(r.skipped, names));
  }

  const centre = (c) => ({ x: c.j + 0.5, y: c.i + 0.5 });

  // A long-press on a name adds the range from the last pick, as a Shift-click does; a double-tap jumps, as a
  // right-click does.
  function onTouchName(kind, n, how) {
    if (how === 'long') onName(kind, n, { shiftKey: true });
    else onJump(kind, n);
  }

  // A mouse drag along a bar. spanBase is the picks at the press. An edge scroll can pass many names in one frame,
  // so the paint waits for the next frame.
  let spanBase = null;
  let spanFrame = 0;
  function onSpan(kind, from, to, add) {
    if (!ctx.ready()) return;
    spanBase ??= picks;
    picks = dragPick(spanBase, kind, from, to, add, axes[kind]);
    spanFrame ||= requestAnimationFrame(() => {
      spanFrame = 0;
      if (!stopped) render();
    });
  }

  function onName(kind, n, e) {
    if (!ctx.ready()) return;
    // A finger has no modifier keys on any layout, so a touch tap adds or removes too.
    picks = pickName(picks, kind, n, tapMods(e, phone() || e.pointerType === 'touch'), axes[kind]);
    render();
  }

  function setCursor(c) {
    cur = c;
    // The box on a bar is for the keys only; while the pointer leads, the ring and the name outlines are enough.
    g.cursor(c, keyed);
    onAt(pointOf(c, axes));
  }

  // A scroll calls this again with the last pointer place, so the cursor marks the crosspoint a click would hit.
  function pointAt(x, y) {
    const c = g.cellAt(x, y);
    if (c && (c.i !== cur.i || c.j !== cur.j)) setCursor(c);
  }

  function onPoint(x, y) {
    ptr = { x, y };
    keyed = false;
    pointAt(x, y);
  }

  const dims = () => ({ rows: axes.dsts.length, cols: axes.srcs.length, page: g.page() });
  // A jump moves the cursor as a key does. r is from gridpaneljump.js.
  function jumped(r) {
    if (!r || !g) return;
    if (r.c) {
      ptr = null;
      setCursor(r.c);
      g.jump(r.c);
    } else {
      note = noteOf(jumpText(r, names, tally?.levels || []));
      render();
    }
  }
  const jumpFrom = (c) => sourceJump(c, axes, env, pressLevels(), full.srcs);

  // Right-clicks on one source step on from the last one. The pointer on the name moves the cursor off that row.
  let lastStep = null; // { src, dst } of the last right-click step
  function onJump(kind, n) {
    // A right-click on a staged crosspoint unstages it, as Alt-click does; anywhere else it jumps.
    if (kind === 'xp' && view.staged(n.src, n.dst).length) return canStage() && unstageAt(n);
    if (kind === 'xp') return jumped(jumpFrom({ i: axes.dsts.indexOf(n.dst), j: axes.srcs.indexOf(n.src) }));
    if (kind === 'dsts') return jumped(jumpFrom({ i: axes.dsts.indexOf(n), j: -1 }));
    const j = axes.srcs.indexOf(n);
    const r = holderJump({ i: lastStep?.src === n ? axes.dsts.indexOf(lastStep.dst) : cur.j === j ? cur.i : -1, j }, axes, env);
    lastStep = r?.c ? { src: n, dst: axes.dsts[r.c.i] } : null;
    jumped(r);
  }

  function moveTo(move) {
    ptr = null;
    let next = moveCursor(cur, move, dims());
    // A v line stays on the crosspoints.
    if (line?.keys) next = { i: Math.max(0, next.i), j: Math.max(0, next.j) };
    else next = enterGrid(cur, next, picks, axes);
    setCursor(next);
    if (move === 'corner') g.scroll.scrollTo(0, 0);
    else g.reveal(cur);
    if (line?.keys) lineMove(centre(cur));
  }

  // Space, x and Shift moves on a bar: the same picks as a click, a Cmd-click and a Shift-click.
  function pickAt(act) {
    const ranged = act.startsWith('range:');
    const kind = ranged ? rangeKind(cur, act.slice('range:'.length)) : barOf(cur);
    const name = (c) => cursorAt(c, axes)[kind === 'srcs' ? 'src' : 'dst'];
    if (!ctx.ready() || !g || (kind !== 'srcs' && kind !== 'dsts')) return;
    if (act === 'pick' || act === 'add') picks = pickName(picks, kind, name(cur), { toggle: act === 'add' }, axes[kind]);
    else {
      const from = name(cur);
      ptr = null;
      setCursor(rangeCursor(cur, act.slice('range:'.length), dims()));
      g.reveal(cur);
      picks = extendPick(picks, kind, from, name(cur), axes[kind]);
    }
    render();
  }

  function onAt(xp) {
    if (at?.src === xp?.src && at?.dst === xp?.dst) return;
    at = xp;
    // Only the preview: a full paint on each pointer move is too slow on a large grid.
    if (g && (pickCount(picks) || g.previewing())) preview();
  }

  // The key list (shortcuts.js). An embedded run has none.
  const keyList = document.getElementById('keys')
    ? shortcutList({ keyMode: () => false, fromHistory: () => false, enterKeys() {}, focusFirst: () => document.activeElement?.blur() })
    : null;
  // As on the main page (desktopkeys.js): Esc and ? close it, and Tab stays on Close.
  function listKey(e) {
    e.stopPropagation();
    if (e.key === 'Escape' || e.key === '?' || gridKeyAction(e, {}) === 'keys') {
      e.preventDefault();
      keyList.close(true);
    } else if (e.key === 'Tab') {
      e.preventDefault();
      document.getElementById('keys-close').focus();
    }
  }

  const onKey = (e) => {
    // The editor's list drawer beside a live test (editorlive.js) keeps every key of its own.
    if (ui.aside?.contains(e.target)) return;
    if (confirm?.isOpen()) return confirm.key(e);
    if (keyList?.shown()) return listKey(e);
    // A dialog over the run (the editor's Icon library) keeps its keys; dialogkeys.js stops them only at its root.
    if (e.target.closest?.('.pb-modal-backdrop:not([hidden])')) return;
    // The page lock's sign-in and dialogs take their keys first.
    if (e.defaultPrevented) return;
    // A search box's own keys come before the grid's, so Esc there clears the box and does not drop picks.
    const box = boxes?.kindOf(e.target);
    if (box) {
      const boxAct = findBoxAction(e);
      if (!boxAct) return;
      e.preventDefault();
      e.stopPropagation();
      return findBox(box, boxAct);
    }
    const finding = stepBox(find) !== null;
    // Only the Levels box: other fields (the instance name, the editor's) keep their own Esc.
    const inField = !!document.activeElement && document.activeElement.id === 'panel-lvl';
    const act = gridKeyAction(e, {
      staged: st.staged.length,
      picks: pickCount(picks),
      inControl: inControl(ui.grid),
      inField,
      bar: barOf(cur),
      finding,
      line: !!line,
    });
    // A phone has no key list.
    if (!act || (act === 'keys' && (!keyList || phone()))) return;
    const moves =
      MOVES.has(act) ||
      act === 'pick' ||
      act === 'add' ||
      act.startsWith('range:') ||
      ['press', 'line', 'jump', 'step', 'undo', 'unstage'].includes(act) ||
      FIND_KEYS.has(act);
    // In the Live Editor the editor keeps its keys; the grid's moves work only with focus in the grid.
    if (moves && ctx.embedded && !ui.grid.contains(document.activeElement)) return;
    // Also stops a focused crosspoint button's own click on Space, which would press it a second time.
    e.preventDefault();
    e.stopPropagation();
    keyed = true;
    if (MOVES.has(act)) {
      if (g) moveTo(act);
    } else if (act === 'pick' || act === 'add' || act.startsWith('range:')) pickAt(act);
    else if (act === 'keys') keyList.open();
    else if (act === 'press') {
      if (line) return stageLine();
      const xp = pointOf(cur, axes);
      if (xp && g) onPress(xp);
    } else if (act === 'line') {
      if (pointOf(cur, axes) && canStage()) {
        ctx.touched();
        line = { a: centre(cur), b: centre(cur), keys: true };
        render();
      }
    } else if (act === 'leave') {
      document.activeElement.blur();
      g?.scroll.focus({ preventScroll: true });
    } else if (act === 'unline' || act === 'unpick' || act === 'drop') clearStep(act);
    else if (act === 'unstage') {
      const xp = pointOf(cur, axes);
      if (xp && canStage()) unstageAt(xp);
    } else if (act === 'jump') jumped(jumpFrom(cur));
    else if (act === 'step') jumped(holderJump(cur, axes, env));
    else if (act === 'undo') pressUndo();
    else if (act === 'confirm') openConfirm();
    else if (act === 'levels') {
      const i = document.getElementById('panel-lvl');
      i?.focus();
      i?.select();
    } else if (act === 'findSrcs' || act === 'findDsts') {
      const i = boxes?.input(act === 'findSrcs' ? 'srcs' : 'dsts');
      i?.focus();
      i?.select();
    } else if (act === 'nextMatch') findStep(stepBox(find), 1);
    else if (act === 'prevMatch') findStep(stepBox(find), -1);
  };
  // Capture, so the grid's Esc is handled before Kiosk mode's own (panelfull.js).
  document.addEventListener('keydown', onKey, true);
  // A turned phone or a changed Layout setting shows or hides the rest-point targets.
  const offLayout = onLayout(render);

  return {
    render: tick,
    data({ tally: t, prot, nm }) {
      if (!nm && dataOf?.[0] === t && dataOf[1] === prot) return;
      dataOf = [t, prot];
      const sig = JSON.stringify([t, prot]);
      // A 304 on names gives the same object (etagfetch.js), so the 90 KB stringify runs only on a change.
      const nmSig = nm && nm !== namesReply ? JSON.stringify(nm) : namesSig;
      if (sig === dataSig && nmSig === namesSig) return;
      dirty = true;
      dataSig = sig;
      tally = t;
      env = envFrom(t, prot, {}, panel.rights);
      const renamed = nmSig !== namesSig;
      if (renamed) {
        namesSig = nmSig;
        namesReply = nm;
        names = namesFrom(nm);
      }
      syncBox();
      // The axes come from the panel and the names, not the tally.
      if (renamed) rebuild();
    },
    pending: () => ({ staged: st.staged.length > 0, busy: st.busy }),
    apply(p) {
      // The staged routes it shows are dropped below.
      if (confirm?.isOpen()) confirm.close();
      panel = p;
      head?.access(p, !ctx.keyed?.());
      st = restage(st, p);
      drawn = null;
      // env reads the panel's Rights, so the next poll makes it again.
      dataSig = '';
      dataOf = null;
      syncBox();
      rebuild();
      render();
    },
    renamed: render,
    // From each panel read (panelrun.js), so staged routes that leave the Rights are left out at once.
    read(p) {
      const { rights: r, confirm: c } = p;
      head?.access(p, !ctx.keyed?.());
      if (c) panel = { ...panel, confirm: c };
      if (JSON.stringify(r ?? null) === JSON.stringify(panel.rights ?? null)) return;
      panel = { ...panel, rights: r };
      dataSig = '';
      dataOf = null;
      syncBox();
      rebuild();
      render();
    },
    stop() {
      stopped = true;
      document.removeEventListener('keydown', onKey, true);
      gest?.stop();
      pos?.stop();
      g?.stop();
      count.remove();
      head?.stop();
      take?.remove();
      clearer?.remove();
      offLayout();
      if (boxNode) boxNode.hidden = true;
      if (confirm?.isOpen()) confirm.close();
      if (keyList?.shown()) keyList.close();
    },
  };
}
