// The editor's sign-in note. The editor never sits blank: each way it cannot reach the panel puts a note and, where it
// helps, a button.
import { $, on } from './core.js';
import { signInAgain, onSessionKey } from './sessionkey.js';
import { signInNote, openFailNote } from './editorrules.js';

// say(id, text) is the editor's. Returns failed(err) for a boot that did not load the panel list or panel.
export function editorSignIn(say) {
  let signInFix = signInAgain;
  let bootFail = ''; // a later sign-in event must not wipe it
  let lastStep = '';
  const note = (step, err) => {
    lastStep = step;
    const { text, button } = signInNote(step, err);
    say('panels-note', text || bootFail);
    const b = $('panels-signin');
    b.hidden = !button;
    b.textContent = button || 'Sign in';
    signInFix = step === 'ended' ? () => location.reload() : signInAgain;
  };
  $('panels-signin').addEventListener('click', () => signInFix());
  onSessionKey((k, step, err) => note(k ? 'ready' : step, err));
  on('wsclose', () => note('offline'));
  return (err) => {
    // An ended or refused key already has its note from onSessionKey.
    if (lastStep === 'ended' || lastStep === 'refused') return;
    bootFail = openFailNote(err.status === undefined ? { status: 0 } : err);
    say('panels-note', bootFail);
  };
}
