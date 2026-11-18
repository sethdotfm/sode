// The toolbar's Rename: an icon beside the panel picker, which already shows the name. It opens the sidebar's Panel
// tab with the Name field focused. #ed-name stays hidden and is the one source of the name for the rest of the editor.
import { $, el, icon } from './core.js';

// rename() is the sidebar's (editorside.js renameField).
export function initNameEdit(rename) {
  $('ed-name').closest('label').hidden = true;
  const btn = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm', icon('text-cursor-input'));
  btn.type = 'button';
  btn.id = 'ed-name-btn';
  btn.setAttribute('aria-label', 'Rename');
  btn.dataset.pbTip = 'Rename this panel';
  btn.addEventListener('click', rename);
  $('ed-pick').closest('label').after(btn);
}
