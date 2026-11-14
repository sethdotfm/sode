// Draws a panel's grid from the state model. Holds no state of its own: paintGrid is called after
// every change, with the model state, the router env and the names.
import { el, icon } from './core.js';
import { artOf, artBOf, artBBadge, cellScale, textHidden } from './panellook.js';
import { currentLabel, nameCellText, SHOW_KINDS } from './panelrules.js';
import { PAGE_KINDS, pageCellText } from './pagecells.js';
import {
  sourceState,
  destinationState,
  presetState,
  currentOfDestination,
  currentSource,
  armed,
  resultShown,
  guardWaiting,
  undoReady,
  GUARD_HOLD_MS,
  GUARD_DOUBLE_MS,
} from './panelmodel.js';
import { NAV_KINDS, navTarget, navText } from './panelnav.js';

const RESULT_ICON = { connected: 'check', already: 'equal', failed: 'x' };

// A cell's artwork is built once with the grid and never touched by the paint functions (fill keeps it): a <use>
// that is detached and re-attached re-resolves its sprite and blinks (TestChromePanelArtHolds). The row holds
// Icon A (sprite icon or image) and then Icon B (a glyph); null when the cell has neither. Beside a sprite icon the
// two share the row 40 to 60 (panels.css); an image fills the row and Icon B is a corner badge (artBBadge), so any
// Icon A drawn as an image gets the badge here. A two-character glyph is marked so its font fits.
export function artNode(cell, art) {
  const a = artOf(cell);
  const b = artBOf(cell);
  if (!a && !b) return null;
  const badge = artBBadge(cell);
  const row = el('div', `sode-panel__art${a && !a.icon ? ' sode-panel__art--fill' : ''}`);
  row.setAttribute('aria-hidden', 'true');
  if (a?.icon) {
    const svg = icon(a.icon, false);
    svg.setAttribute('class', 'pb-icon sode-panel__art-a');
    row.append(svg);
  } else if (a) {
    row.append(artImage('sode-panel__art-a', art, a.id, a.lib));
  }
  if (b) row.append(el('span', `sode-panel__art-b${b.length > 1 ? ' sode-panel__art-b--wide' : ''}${badge ? ' sode-panel__art-b--badge' : ''}`, b));
  return row;
}

// An uploaded image from createArt, hidden until it loads. An Icon library SVG becomes a mask in the text colour, as a
// sprite icon is drawn, so it changes with the theme and a multi-colour file draws in one; a mask, like an <img>, never runs
// the file's script. Anything else is an <img> in its own colours. The swap needs the node in place by then, which a
// resolved promise's later turn gives.
export function artImage(cls, art, id, lib) {
  const img = el('img', cls);
  img.alt = '';
  img.hidden = true;
  art?.look(id).then((l) => {
    if (!l) return;
    if (lib && l.type.startsWith('image/svg+xml')) {
      const mask = el('span', `${cls} sode-panel__art-mask`);
      mask.style.maskImage = `url("${l.url}")`;
      img.replaceWith(mask);
      return;
    }
    img.src = l.url;
    img.hidden = false;
  });
  return img;
}

// Replaces a cell's children apart from its artwork, and only when they differ from the last paint, so an idle cell
// is not touched by the one-second redraw.
function fill(b, ...kids) {
  const now = [...b.childNodes].filter((n) => n !== b._art);
  if (now.length === kids.length && now.every((n, i) => n.isEqualNode(kids[i]))) return;
  for (const n of now) n.remove();
  b.append(...kids);
}

// hold is { down(cell), cancel() } for a Take cell with a hold guard: pointer or Enter and Space start it, leaving ends it.
// art is createArt's answer, for cells with uploaded artwork.
export function buildGrid(host, panel, cells, onPress, hold, art) {
  host.replaceChildren();
  host.style.gridTemplate = `repeat(${panel.rows}, 1fr) / repeat(${panel.cols}, 1fr)`;
  host.style.aspectRatio = `${panel.cols} / ${panel.rows}`;
  const nodes = cells.map((cell) => {
    const cls = cell.kind === 'take' ? 'pb-btn pb-btn--commit sode-panel__take' : 'pb-btn pb-xpt';
    const b = el('button', `sode-panel__cell sode-panel__cell--${cell.kind} ${cls}`);
    b.type = 'button';
    b.disabled = SHOW_KINDS.includes(cell.kind);
    if (cell.fontSize) b.style.setProperty('--cell-fs', cellScale(cell));
    b._art = artNode(cell, art);
    if (b._art) b.append(b._art);
    if (textHidden(cell)) b.dataset.hideText = '1';
    b.style.gridArea = `${cell.r} / ${cell.c} / span ${cell.rs} / span ${cell.cs}`;
    b.addEventListener('click', () => onPress(cell));
    if (cell.kind === 'take' && hold) {
      b.addEventListener('pointerdown', () => hold.down(cell));
      for (const ev of ['pointerup', 'pointerleave', 'pointercancel', 'blur']) b.addEventListener(ev, hold.cancel);
      b.addEventListener('keydown', (e) => (e.key === 'Enter' || e.key === ' ') && !e.repeat && hold.down(cell));
      b.addEventListener('keyup', hold.cancel);
    }
    host.append(b);
    return b;
  });
  return { cells, nodes };
}

const markIcon = () => {
  const i = icon('check');
  i.classList.add('sode-panel__mark');
  return i;
};

function paintSource(b, cell, { st, env, names }) {
  const s = sourceState(st, cell.n, env);
  b.dataset.s = s;
  fill(b, el('span', 'sode-panel__text', names.of(cell)), ...(s === 'current' ? [markIcon()] : []));
}

function paintDestination(b, cell, { st, env, names, now }) {
  b.dataset.s = destinationState(st, cell.n);
  b.dataset.v = st.view?.dst === cell.n ? '1' : '';
  b.classList.toggle('pb-xpt--protected', env.protectedDsts.has(cell.n));
  const name = el('span', '', el('span', 'sode-panel__text', names.of(cell)));
  const shown = resultShown(st, cell.n, now);
  if (shown) {
    const mark = icon(RESULT_ICON[shown.result]);
    mark.setAttribute('role', 'img');
    mark.setAttribute('aria-label', shown.word);
    name.append(' ', mark);
    if (shown.text) name.append(' ', el('small', 'sode-panel__result sode-panel__text', shown.text));
  }
  b.dataset.result = shown?.result || '';
  const cur = currentOfDestination(st, env, cell);
  const under = env.protectedDsts.has(cell.n) ? 'Protected' : cur >= 0 ? names.source(cur) : '';
  const kids = [name, el('small', 'sode-panel__text', under)];
  const staged = st.staged.find((x) => x.dst === cell.n);
  if (staged) kids.push(el('span', 'sode-panel__staged sode-panel__text', `→ ${names.source(staged.src)}`));
  fill(b, ...kids);
}

function paintPreset(b, cell, { st, env, names }) {
  b.dataset.s = presetState(st, cell, env);
  fill(b, el('span', 'sode-panel__text', names.of(cell)));
  b.disabled = !env.presets?.has(`${cell.scope}:${cell.name}`);
}

// Not pressable: it only reads the viewed destination.
function paintCurrent(b, cell, { st, env, names }) {
  const src = currentSource(env, st);
  const label = currentLabel(cell, st.view ? names.destination(st.view.dst) : '');
  fill(b, el('small', 'sode-panel__text', label), el('span', 'sode-panel__text', src >= 0 ? names.source(src) : ''));
}

// Not pressable: the panel's own name, or the name of the instance showing it.
function paintName(b, cell, ctx) {
  const t = nameCellText(cell, ctx);
  fill(b, ...(t.caption ? [el('small', 'sode-panel__text', t.caption)] : []), el('span', 'sode-panel__text', t.value));
}

// Not pressable: the Page this grid shows (st.page), so a copy on another Page names that Page.
function paintPageCell(b, cell, ctx) {
  const t = pageCellText(cell, { ...ctx, page: ctx.st.page });
  const cap = t.caption ? [el('small', 'sode-panel__text', t.caption)] : [];
  fill(b, ...cap, ...t.lines.map((l) => el('span', 'sode-panel__text', l)));
}

// Greyed and unpressable when the press would go nowhere: a disabled or deleted target, or an end without wrap.
function paintNav(b, cell, { st, pages = [], wrap = true }) {
  b.disabled = navTarget(pages, st.page, cell, wrap) === null;
  fill(b, el('span', 'sode-panel__text', navText(cell, pages)));
}

// Greyed with nothing of this instance's left to undo, a route to restore outside the Rights, or no take scope (ctx.undo).
function paintUndo(b, cell, { st, names, undo }) {
  b.disabled = !undoReady(st, undo);
  fill(b, el('span', 'sode-panel__text', names.of(cell)));
}

function takeLabel(st, cell, now) {
  if (cell.caption) return cell.caption;
  if (st.mode === 'instant') return 'Take';
  if (st.holdAt) return 'Keep holding';
  if (guardWaiting(st, now)) return 'Press again';
  return st.guard === 'hold' ? 'Hold to take' : 'Take';
}

export function paintGrid(grid, ctx) {
  const on = armed(ctx.st, ctx.env);
  grid.cells.forEach((cell, i) => {
    const b = grid.nodes[i];
    if (cell.kind === 'source') paintSource(b, cell, ctx);
    else if (cell.kind === 'destination') paintDestination(b, cell, ctx);
    else if (cell.kind === 'preset') paintPreset(b, cell, ctx);
    else if (cell.kind === 'current') paintCurrent(b, cell, ctx);
    else if (cell.kind === 'panelName' || cell.kind === 'instanceName') paintName(b, cell, ctx);
    else if (PAGE_KINDS.includes(cell.kind)) paintPageCell(b, cell, ctx);
    else if (cell.kind === 'clear') fill(b, el('span', 'sode-panel__text', ctx.names.of(cell)));
    else if (cell.kind === 'undo') paintUndo(b, cell, ctx);
    else if (NAV_KINDS.includes(cell.kind)) paintNav(b, cell, ctx);
    else {
      fill(b, el('span', 'sode-panel__text', takeLabel(ctx.st, cell, ctx.now)));
      b.dataset.guard = ctx.st.holdAt ? 'holding' : guardWaiting(ctx.st, ctx.now) ? 'waiting' : '';
      b.style.setProperty('--wait-ms', `${ctx.st.holdAt ? GUARD_HOLD_MS : GUARD_DOUBLE_MS}ms`);
      b.disabled = !on;
      b.classList.toggle('sode-panel__take--armed', on);
    }
  });
}
