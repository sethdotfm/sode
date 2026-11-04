// Saving something to the logged-in user's own profile (filter sets,
// presets): the same flow as globalSaver but with no PIN row, since a user
// may always change their own profile. One saver per dialog; the dialog
// supplies how it shows a note.

import { on, sentence } from './core.js';
import { profile, saveProfile } from './profile.js';
import { showResult } from './plans.js';

// The server caps a whole profile at 48 KiB (the socket read limit is 64).
export const PROFILE_MAX_BYTES = 48 * 1024;

// Why `parts` ({filterSets}, {presets}) would make the profile `p` too big
// to save, or ''. Parts replace the same-named part of p.
export function profileProblem(p, parts) {
  const next = { prefs: p.prefs, filterSets: p.filterSets, presets: p.presets, ...parts };
  const bytes = JSON.stringify(next).length;
  return bytes > PROFILE_MAX_BYTES
    ? `Your profile is too big to save (${Math.ceil(bytes / 1024)} KiB, at most ${PROFILE_MAX_BYTES / 1024} KiB). Delete something first.`
    : '';
}

// setNote(text, isError), isOpen(): the dialog is showing (after it closed,
// a failure is a result banner), render(): redraw what depends on busy().
// Returns { change(parts, done), cancel(), close(), busy() }.
export function profileSaver({ setNote, isOpen, failTitle, render = () => {}, okNote = '' }) {
  let gen = 0, pending = false;

  function change(parts, done) {
    const big = profileProblem(profile(), parts);
    if (big) return setNote(big, true);
    const mine = ++gen;
    pending = true;
    render();
    setNote('Saving…');
    saveProfile(parts).then((r) => {
      if (mine !== gen) return;
      pending = false;
      const open = isOpen();
      if (r.ok) done();
      if (open) {
        render();
        if (!r.ok) setNote(sentence(r.err) || 'Not saved.', true);
        else if (okNote) setNote(okNote);
      } else if (!r.ok) showResult('error', failTitle, sentence(r.err) || 'Not saved.');
    });
  }

  // A save on a dead socket is never answered: let it be tried again.
  on('wsclose', () => {
    if (!pending) return;
    gen++;
    pending = false;
    render();
    if (isOpen()) setNote('Connection lost; try again.', true);
  });

  return {
    change,
    cancel() { gen++; pending = false; setNote(''); render(); },
    close() { return pending; },
    busy: () => pending,
  };
}
