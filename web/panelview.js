// Draws a panel's grid from the state model. Holds no state of its own: paintGrid is called after
// every change, with the model state, the router env and the names.
import { el, icon } from './core.js';
import { sourceState, destinationState, currentOfDestination, armed, resultOf } from './panelmodel.js';

const RESULT_ICON = { connected: 'check', already: 'equal', failed: 'x' };
const RESULT_WORD = { connected: 'Connected', already: 'Already', failed: 'Failed' };

export function buildGrid(host, panel, cells, onPress) {
  host.replaceChildren();
  host.style.gridTemplate = `repeat(${panel.rows}, 1fr) / repeat(${panel.cols}, 1fr)`;
  host.style.aspectRatio = `${panel.cols} / ${panel.rows}`;
  const nodes = cells.map((cell) => {
    const cls = cell.kind === 'take' ? 'pb-btn pb-btn--commit sode-panel__take' : 'pb-btn pb-xpt';
    const b = el('button', `sode-panel__cell sode-panel__cell--${cell.kind} ${cls}`);
    b.type = 'button';
    b.style.gridArea = `${cell.r} / ${cell.c} / span ${cell.rs} / span ${cell.cs}`;
    b.addEventListener('click', () => onPress(cell));
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
  const result = resultOf(st, cell.n, now);
  if (result) {
    const mark = icon(RESULT_ICON[result]);
    mark.setAttribute('role', 'img');
    mark.setAttribute('aria-label', RESULT_WORD[result]);
    name.append(' ', mark);
  }
  b.dataset.result = result || '';
  const cur = currentOfDestination(st, env, cell);
  const under = env.protectedDsts.has(cell.n) ? 'Protected' : cur >= 0 ? names.source(cur) : '';
  const kids = [name, el('small', '', under)];
  const staged = st.staged.find((x) => x.dst === cell.n);
  if (staged) kids.push(el('span', 'sode-panel__staged', `→ ${names.source(staged.src)}`));
  b.replaceChildren(...kids);
}

export function paintGrid(grid, ctx) {
  const on = armed(ctx.st, ctx.env);
  grid.cells.forEach((cell, i) => {
    const b = grid.nodes[i];
    if (cell.kind === 'source') paintSource(b, cell, ctx);
    else if (cell.kind === 'destination') paintDestination(b, cell, ctx);
    else {
      b.textContent = cell.caption || (ctx.st.mode === 'instant' ? 'Instant' : 'Take');
      b.disabled = !on;
      b.classList.toggle('sode-panel__take--armed', on);
    }
  });
}
