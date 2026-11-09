// Draws a panel's grid from the state model. Holds no state of its own: paintGrid is called after
// every change, with the model state, the router env and the names.
import { el, icon } from './core.js';
import { artOf, cellScale } from './panellook.js';
import { currentLabel } from './panelrules.js';
import {
  sourceState,
  destinationState,
  presetState,
  currentOfDestination,
  currentSource,
  armed,
  resultShown,
  guardWaiting,
  GUARD_HOLD_MS,
  GUARD_DOUBLE_MS,
} from './panelmodel.js';

const RESULT_ICON = { connected: 'check', already: 'equal', failed: 'x' };

// A cell's artwork lives outside the paint functions, which replace a cell's children; paintGrid puts it back first.
export function artNode(cell, art) {
  const a = artOf(cell);
  if (!a) return null;
  if (a.icon) {
    const svg = icon(a.icon, false);
    svg.setAttribute('class', 'pb-icon sode-panel__art sode-panel__art--icon');
    return svg;
  }
  const img = el('img', 'sode-panel__art');
  img.alt = '';
  img.hidden = true;
  art?.url(a.id).then((url) => {
    if (!url) return;
    img.src = url;
    img.hidden = false;
  });
  return img;
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
    b.disabled = cell.kind === 'current';
    if (cell.fontSize) b.style.setProperty('--cell-fs', cellScale(cell));
    b._art = artNode(cell, art);
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

function paintSource(b, cell, { st, env, names }) {
  const s = sourceState(st, cell.n, env);
  b.dataset.s = s;
  b.replaceChildren(el('span', '', names.of(cell)));
  if (s === 'current') b.append(icon('check'));
}

function paintDestination(b, cell, { st, env, names, now }) {
  b.dataset.s = destinationState(st, cell.n);
  b.dataset.v = st.view?.dst === cell.n ? '1' : '';
  b.classList.toggle('pb-xpt--protected', env.protectedDsts.has(cell.n));
  const name = el('span', '', names.of(cell));
  const shown = resultShown(st, cell.n, now);
  if (shown) {
    const mark = icon(RESULT_ICON[shown.result]);
    mark.setAttribute('role', 'img');
    mark.setAttribute('aria-label', shown.word);
    name.append(' ', mark);
    if (shown.text) name.append(' ', el('small', 'sode-panel__result', shown.text));
  }
  b.dataset.result = shown?.result || '';
  const cur = currentOfDestination(st, env, cell);
  const under = env.protectedDsts.has(cell.n) ? 'Protected' : cur >= 0 ? names.source(cur) : '';
  const kids = [name, el('small', '', under)];
  const staged = st.staged.find((x) => x.dst === cell.n);
  if (staged) kids.push(el('span', 'sode-panel__staged', `→ ${names.source(staged.src)}`));
  b.replaceChildren(...kids);
}

function paintPreset(b, cell, { st, env, names }) {
  b.dataset.s = presetState(st, cell, env);
  b.replaceChildren(el('span', '', names.of(cell)));
  b.disabled = !env.presets?.has(`${cell.scope}:${cell.name}`);
}

// Not pressable: it only reads the viewed destination.
function paintCurrent(b, cell, { st, env, names }) {
  const src = currentSource(env, st);
  const label = currentLabel(cell, st.view ? names.destination(st.view.dst) : '');
  b.replaceChildren(el('small', '', label), el('span', '', src >= 0 ? names.source(src) : ''));
}

function takeLabel(st, cell, now) {
  if (cell.caption) return cell.caption;
  if (st.mode === 'instant') return 'Instant';
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
    else if (cell.kind === 'clear') b.textContent = ctx.names.of(cell);
    else {
      b.textContent = takeLabel(ctx.st, cell, ctx.now);
      b.dataset.guard = ctx.st.holdAt ? 'holding' : guardWaiting(ctx.st, ctx.now) ? 'waiting' : '';
      b.style.setProperty('--wait-ms', `${ctx.st.holdAt ? GUARD_HOLD_MS : GUARD_DOUBLE_MS}ms`);
      b.disabled = !on;
      b.classList.toggle('sode-panel__take--armed', on);
    }
    if (b._art) b.prepend(b._art);
  });
}
