// The Home key on Home (homekey.js): back to the plain Home view with the focus in the Source box. It closes the
// Palette, the take Confirm, the key list, the Grid popup, the dialogs and the Settings view. Unsaved edits get their
// own warning first (homeStep in homekeyrules.js). It never clears the patch boxes or the staged routes: Esc does that.
import { $, on, emit } from './core.js';
import { closePalette, unsavedEdits } from './palette.js';
import { closeOpenConfirm } from './keyconfirm.js';
import { closeShown, escShown, dialogShown } from './dialogkeys.js';
import { gridShown, hideGrid } from './gridpopup.js';
import { isOpen } from './settingsform.js';
import { layout } from './layout.js';
import { homeStep } from './homekeyrules.js';

const RUN = {
  closeDialogs: closeShown,
  escDialogs: escShown,
  // Settings' Exit: with unsaved changes the first try warns and the second discards (settings.js).
  exitSettings: () => emit('settingsClose'),
};
const TRIED = { closeDialogs: 'dialogs', escDialogs: 'dialogs', exitSettings: 'settings' };

on('homeKey', () => {
  closePalette();
  closeOpenConfirm();
  if ($('keys') && !$('keys').hidden) $('keys-close').click();
  if (gridShown()) hideGrid(false);
  const tried = new Set();
  for (;;) {
    const step = homeStep({ dialog: dialogShown(), settings: isOpen(), unsaved: unsavedEdits(), tried });
    if (step === 'stop') return;
    if (step === 'focus') break;
    tried.add(TRIED[step]);
    RUN[step]();
  }
  $(layout() === 'phone' ? 'm-src' : 'src')?.focus({ preventScroll: true });
});
