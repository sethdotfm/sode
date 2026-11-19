// Reload on the panels page's Open now list (POST /api/v1/panels/{id}/instances/reload): per row and for every page
// of the panel. A Trusted user only, as for an Assign; the server says so again. Kept out of panelsmon.js.
import { el, icon } from './core.js';
import { reloadAllState } from './panelbackrules.js';
import { streamState } from './panelassignrules.js';

// The text an Open now row shows while its stream is gone: nothing would hear a Reload or a new name.
export const offlineNote = () => el('span', 'sode-ov__muted', 'Offline');

function button(label, tip, state, onClick) {
  const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm', icon('rotate-cw'), ` ${label}`);
  b.type = 'button';
  b.disabled = state !== 'ready';
  b.dataset.pbTip = state === 'ready' ? tip : 'Offline: no open page';
  b.classList.add('pb-tip--end');
  b.addEventListener('click', onClick);
  return b;
}

export function reloadButton(p, row, reload) {
  const state = reload ? streamState(row) : '';
  if (!state) return null;
  const b = button('Reload', 'Keeps key and name', state, () => reload(p.id, row.keyless ? { keyless: row.keyless } : { instance: row.device }));
  b.dataset.reloadInstance = row.device;
  return b;
}

export function reloadAllButton(p, reload) {
  const state = reload ? reloadAllState(p) : '';
  if (!state) return null;
  const b = button('Reload all', 'All open pages', state, () => reload(p.id, { all: true }));
  b.dataset.reloadAll = '';
  return b;
}
