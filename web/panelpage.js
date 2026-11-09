// Entry for the three panel pages (/panels, /panels/editor, /panels/<id>): a placeholder until
// the panels land. Sign-in and the page lock come from lock.js, as on /log.
import { $, on, connect } from './core.js';
import './brand.js';
import './lock.js';
import { startSession, signInAgain, onSessionKey } from './sessionkey.js';

// 4x8 maps one to one onto a hardware key deck (docs/panels-design.md).
export const DEFAULT_GRID = { panelRows: 4, panelCols: 8 };

on('brand', ({ text }) => {
  const title = document.querySelector('.sode-panels__title');
  title.textContent = title.textContent.replace(/^Sode/, text);
  document.title = title.textContent;
});

$('panels-note').textContent = `Panels are not built yet. The default grid will be ${DEFAULT_GRID.panelRows} by ${DEFAULT_GRID.panelCols}.`;

// A panel itself (/panels/<id>) is not yet told its mode, so every one runs as the signed-in user.
if (/\/panels\/(?!editor\/?$)[^/]+$/.test(location.pathname)) {
  const note = $('panels-note');
  const signIn = $('panels-signin');
  signIn.addEventListener('click', signInAgain);
  onSessionKey((key, step) => {
    signIn.hidden = step !== 'sign-in';
    if (key) note.textContent = 'Signed in. This panel acts as you until you sign out.';
    else if (step === 'sign-in') note.textContent = 'Sign in with your PIN to open this panel.';
    else if (step === 'no-users') note.textContent = 'This panel needs a user, and Sode has none.';
    else if (step === 'request') note.textContent = 'Opening the panel as you.';
  });
  startSession();
}

connect();
