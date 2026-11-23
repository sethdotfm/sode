// The toolbar's Rename opens the Panel tab with the Name field focused.
import { $, el, icon } from './core.js';

// rename() is the sidebar's (editorside.js renameField).
export function initNameEdit(rename) {
  $('ed-name').closest('label').hidden = true;
  const btn = el('button', 'pb-btn pb-btn--outline pb-btn--sm sode-ed__rename', icon('text-cursor-input'), el('span', null, 'Rename'));
  btn.type = 'button';
  btn.id = 'ed-name-btn';
  btn.setAttribute('aria-label', 'Rename');
  btn.dataset.pbTip = 'Rename';
  btn.addEventListener('click', rename);
  $('ed-pick').closest('label').after(btn);
}
