// The Home key, Alt+Shift+h, on every page. homekey.js reads the page and gives the facts here, and homeview.js does
// Home's part. Pure, so the Node tests run it.

// As chord() (keys.js) names it: the physical H through e.code, so macOS Option+Shift+H (which types Ó) still matches.
// Browsers keep Ctrl+H, Ctrl+Shift+H and Alt+Home, so the key follows Alt+Shift+k and Alt+Shift+c.
export const HOME_CHORD = 'alt+shift+h';

// What the key does. refusal is the Palette's own (paletteRefusal in palette.js, without unsaved edits): under the Lock,
// on a Kiosk panel and during setup it does nothing. On Home it goes back to the plain Home view; another page opens
// Home in this tab.
export function homeKeyAct({ page, refusal }) {
  if (refusal) return 'none';
  return page === 'main' ? 'here' : 'go';
}

// Home's next step, after the Palette, the take Confirm, the key list and the Grid popup closed. state: dialog (a
// dialog shows), settings (the Settings view shows), unsaved (a bus unsavedAsk listener has edits) and tried (the steps
// done in this press). With no unsaved edits, every dialog closes at once, as before the Palette opens. With unsaved
// edits each dialog gets its own Esc, and Settings its Exit, so each warns first. It stops at the first one that still
// shows: the next press is the second try, which discards. The patch boxes and the staged routes stay.
export function homeStep({ dialog, settings, unsaved, tried = new Set() }) {
  if (dialog) return tried.has('dialogs') ? 'stop' : unsaved ? 'escDialogs' : 'closeDialogs';
  if (settings) return tried.has('settings') ? 'stop' : 'exitSettings';
  return 'focus';
}
