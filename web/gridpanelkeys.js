// Grid panel keys to actions. Pure apart from keys.js. The run does the action and marks the key handled when this
// returns one. Keys are exact chord(e) names, so h/j/k/l are physical keys and a chord with an extra modifier is not
// the grid's. ctx is { staged, picks, inControl, inField, bar, finding, line }. staged counts staged routes and picks
// counts picked names. inControl is true when focus is on a header button or a field, inField only for the Levels box. bar
// is barOf(cursor) in gridpanelcursor.js. finding is true while a box has text. line is true while a Drawn line
// shows. A move action is a name in gridpanelcursor.js MOVES. 'range:<move>' is a Shift move along a bar or across
// the crosspoints, which extends the pick.
import { chord } from './keys.js';
import { MOVES } from './gridpanelcursor.js';

const free = (act) => (c) => (c.inControl ? null : act);
const onBar = (c) => c.bar === 'srcs' || c.bar === 'dsts';
const named = (act) => (c) => (!c.inControl && onBar(c) ? act : null);
// On a crosspoint (bar null) a Shift move extends the pick of the names it crosses: sources across, destinations down.
const range = (kind, move) => (c) => (!c.inControl && (c.bar === kind || c.bar === null) ? `range:${move}` : null);
const keys = () => 'keys';

// The line goes first, then picks, then staged routes. The Clear button beside Take uses it too.
export const clearAction = (c) => (c.line ? 'unline' : c.picks ? 'unpick' : c.staged ? 'drop' : null);

const ACTIONS = {
  enter: (c) => (c.staged && !c.inControl ? 'confirm' : null),
  f: free('levels'),
  // With nothing to clear, Kiosk mode's own Esc works. In a field (the Levels box) Esc only leaves it, so typing
  // there never drops picks or staged routes.
  escape: (c) => (c.inField ? 'leave' : clearAction(c)),
  v: free('line'),
  ' ': (c) => (c.inControl || c.bar === 'corner' ? null : onBar(c) ? 'pick' : 'press'),
  x: named('add'),
  'shift+left': range('srcs', 'left'),
  'shift+right': range('srcs', 'right'),
  'shift+h': range('srcs', 'left'),
  'shift+l': range('srcs', 'right'),
  'alt+shift+left': range('srcs', 'pageLeft'),
  'alt+shift+right': range('srcs', 'pageRight'),
  'shift+up': range('dsts', 'up'),
  'shift+down': range('dsts', 'down'),
  'shift+k': range('dsts', 'up'),
  'shift+j': range('dsts', 'down'),
  'shift+pageup': range('dsts', 'pageUp'),
  'shift+pagedown': range('dsts', 'pageDown'),
  'alt+shift+up': range('dsts', 'pageUp'),
  'alt+shift+down': range('dsts', 'pageDown'),
  // As on the main page: Ctrl+? from anywhere, also as the / key for browsers that drop Shift's character under Ctrl.
  'shift+?': free('keys'),
  '?': free('keys'),
  'ctrl+shift+?': keys,
  'ctrl+shift+/': keys,
  g: free('jump'),
  'shift+g': free('step'),
  u: free('undo'),
  // One staged route at the cursor, as an Alt-click does.
  delete: (c) => (c.staged && !c.inControl && !c.bar ? 'unstage' : null),
  backspace: (c) => (c.staged && !c.inControl && !c.bar ? 'unstage' : null),
  up: free('up'),
  down: free('down'),
  left: free('left'),
  right: free('right'),
  k: free('up'),
  j: free('down'),
  h: free('left'),
  l: free('right'),
  pageup: free('pageUp'),
  pagedown: free('pageDown'),
  'alt+up': free('pageUp'),
  'alt+down': free('pageDown'),
  'alt+left': free('pageLeft'),
  'alt+right': free('pageRight'),
  home: free('corner'),
  end: free('colEnd'),
  'ctrl+home': free('rowStart'),
  'meta+home': free('rowStart'),
  'ctrl+end': free('rowEnd'),
  'meta+end': free('rowEnd'),
  // Find and Filter (gridpanelfind.js). n and N stay free while no box has text.
  s: free('findSrcs'),
  d: free('findDsts'),
  n: (c) => (c.finding ? free('nextMatch')(c) : null),
  'shift+n': (c) => (c.finding ? free('prevMatch')(c) : null),
};

// While a line is drawn, Space stages it and only moves and Esc work besides.
const LINE_KEEPS = new Set(['press', 'unline', 'leave']);

export function gridKeyAction(e, ctx) {
  const act = ACTIONS[chord(e)]?.(ctx) ?? null;
  return ctx.line && act && !MOVES.has(act) && !LINE_KEEPS.has(act) ? null : act;
}

// Keys in a Source bar or Destination bar search box. Any other key is typing. Ctrl+Tab cannot be used: browsers
// keep it, so Ctrl+/ switches the mode. Chrome on Windows and Linux keeps Ctrl+N for a new window, so there only
// Ctrl+P arrives; on a Mac both do.
const BOX = {
  enter: 'nextMatch',
  'shift+enter': 'prevMatch',
  'ctrl+n': 'nextMatch',
  'ctrl+p': 'prevMatch',
  escape: 'clear',
  tab: 'swap',
  'shift+tab': 'swap',
  'ctrl+/': 'mode',
};

export const findBoxAction = (e) => BOX[chord(e)] ?? null;
