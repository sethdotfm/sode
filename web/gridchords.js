// The editor grid's Cmd/Ctrl commands. Pure apart from keys.js. Both modifiers on every OS, as saveChord does; an
// exact chord(e) match leaves out any extra modifier. Keys are spelled as chord() names them (Cmd+Shift+Z is
// 'shift+meta+z'); paneleditor.js maps each command name to what it does.
import { chord } from './keys.js';

export const GRID_CHORDS = {
  'ctrl+a': 'selectAll',
  'meta+a': 'selectAll',
  'ctrl+c': 'copy',
  'meta+c': 'copy',
  'ctrl+x': 'cut',
  'meta+x': 'cut',
  'ctrl+v': 'paste',
  'meta+v': 'paste',
};

// Inputs that take typing; a checkbox or button does not, so the grid command still runs from one.
const TEXT_INPUTS = new Set(['text', 'search', 'number', 'email', 'url', 'tel', 'password', '']);

export const typing = (t) =>
  !!t && (t.isContentEditable || t.tagName === 'TEXTAREA' || (t.tagName === 'INPUT' && TEXT_INPUTS.has((t.type || '').toLowerCase())));

// The command name for a keydown, or null. A focused text field keeps its own cut, copy, paste, select all and undo.
export const gridCommand = (e) => (typing(e.target) ? null : (GRID_CHORDS[chord(e)] ?? null));
