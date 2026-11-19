// Reload on the panels page's Open now list (POST /api/v1/panels/{id}/instances/reload): per row and for every page
// of the panel. A Trusted user only, as for an Assign; the server says so again. Kept out of panelsmon.js.
import { el, icon } from './core.js';
import { reloadState, reloadAllState } from './panelbackrules.js';

// The text an Open now row shows while its stream is gone: nothing would hear a Reload or a new name.
export const offlineNote = () => el('span', 'sode-ov__muted', 'Offline');

// target is {instance}, {keyless} or {all: true}; reload(id, target) sends it.
function button(label, tip, state, onClick) {
  const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm', icon('rotate-cw'), ` ${label}`);
  b.type = 'button';
  b.disabled = state !== 'ready';
  b.dataset.pbTip = state === 'ready' ? tip : 'Offline: no page holds a stream now';
  b.classList.add('pb-tip--end');
  b.addEventListener('click', onClick);
  return b;
}

// The Reload button of one row, or null where it has none.
export function reloadButton(p, row, reload) {
  const state = reload ? reloadState(row) : '';
  if (!state) return null;
  const b = button('Reload', 'Reload this page. It keeps its key and name.', state, () =>
    reload(p.id, row.keyless ? { keyless: row.keyless } : { instance: row.device }),
  );
  b.dataset.reloadInstance = row.device;
  return b;
}

// Reload all: every page of the panel that holds a stream now.
export function reloadAllButton(p, reload) {
  const state = reload ? reloadAllState(p) : '';
  if (!state) return null;
  const b = button('Reload all', 'Reload every open page of this panel', state, () => reload(p.id, { all: true }));
  b.dataset.reloadAll = '';
  return b;
}
