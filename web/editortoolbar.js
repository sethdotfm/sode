// The editor's toolbar (the panel picker, Save, the save state and the panel's links) and its read-only banner.
import { $, el } from './core.js';
import { groupPanels } from './editorrules.js';

// list is GET /api/v1/panels; loaded is the open panel, or null for a new one.
export function drawPicker(list, loaded) {
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

// saveState is { busy, failed }; dirty() is read only when the paint needs it.
export function paintToolbar(ro, saveState, dirty, loaded) {
  $('ed-name').disabled = ro;
  const saveBtn = $('ed-save');
  const failed = !ro && !!saveState.failed;
  saveBtn.disabled = ro || saveState.busy || (!dirty() && !failed);
  saveBtn.textContent = failed ? 'Retry save' : 'Save';
  saveBtn.classList.toggle('pb-btn--ok', !failed);
  saveBtn.classList.toggle('pb-btn--danger', failed);
  $('ed-banner').hidden = !ro;
  // Not say(): the slot keeps its size whatever it holds, so the toolbar never re-wraps (panels.css).
  $('ed-state').textContent = ro ? '' : saveState.busy ? 'Saving' : failed ? 'Not saved' : dirty() ? 'Unsaved changes' : loaded ? 'Saved' : '';
  const link = $('ed-open');
  link.hidden = !loaded;
  if (loaded) link.href = `panels/${loaded.id}`;
  // A panel that only acts as its API client has no link without its key (Share, Link with key). A Both panel's
  // plain link opens in Session mode, and a Public panel's opens for anyone.
  $('ed-copylink').hidden = !loaded || loaded.acts === 'api';
}
