// The editor grid's commands: Ctrl, or Cmd on a Mac, as saveChord. Pure apart from keys.js. Keys are spelled as
// modChord() names them, so an extra modifier is no match. paneleditor.js maps each command name to what it does.
import { modChord } from './keys.js';

export const GRID_CHORDS = {
  'mod+a': 'selectAll',
  'mod+c': 'copy',
  'mod+x': 'cut',
  'mod+v': 'paste',
  'mod+z': 'undo',
  'mod+shift+z': 'redo',
};

// Inputs that take typing; a checkbox or button does not, so the grid command still runs from one.
const TEXT_INPUTS = new Set(['text', 'search', 'number', 'email', 'url', 'tel', 'password', '']);

export const typing = (t) =>
  !!t && (t.isContentEditable || t.tagName === 'TEXTAREA' || (t.tagName === 'INPUT' && TEXT_INPUTS.has((t.type || '').toLowerCase())));

// The command name for a keydown, or null. A focused text field keeps its own cut, copy, paste, select all and undo.
export const gridCommand = (e, isMac) => (typing(e.target) ? null : (GRID_CHORDS[modChord(e, isMac)] ?? null));
