// Imports nothing, so tests can load it alone. chord(e) names a keystroke the same way everywhere; one set of listeners tracks html[data-shift] and html[data-alt].

export const mac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);

// Letters and digits by e.code (Option+P is 'p', not 'π').
function keyName(e) {
  const m = /^(?:Digit|Key)(.)$/.exec(e.code || '');
  return m ? m[1].toLowerCase() : /^Arrow(.+)$/.exec(e.key || '')?.[1].toLowerCase() || (e.key || '').toLowerCase();
}
export function chord(e) {
  return `${e.ctrlKey ? 'ctrl+' : ''}${e.altKey ? 'alt+' : ''}${e.shiftKey ? 'shift+' : ''}${e.metaKey ? 'meta+' : ''}${keyName(e)}`;
}

// The primary modifier: Ctrl, or Cmd on a Mac, never both. The Windows key is not one.
export const primary = (e, isMac = mac) => (e.ctrlKey && !e.metaKey) || (isMac && e.metaKey && !e.ctrlKey);

// chord(e) with the primary modifier named mod (mod+z, mod+shift+z), '' without one. docs/agents/web-layout.md lists the mod commands.
export function modChord(e, isMac = mac) {
  if (!primary(e, isMac)) return '';
  return `mod+${e.altKey ? 'alt+' : ''}${e.shiftKey ? 'shift+' : ''}${keyName(e)}`;
}

// Undo: Ctrl+Z, or Cmd+Z on a Mac, by the typed letter so a QWERTZ keyboard keeps its Z. own is true for the OS's
// own text-undo chord, which a box with text keeps. null for any other key.
export function undoKey(e, isMac = mac) {
  if (!primary(e, isMac) || e.shiftKey || e.altKey || (e.key || '').toLowerCase() !== 'z') return null;
  return { own: !!e.metaKey === isMac };
}

// Keep in step with the combobox and Settings search: both get their Ctrl/Cmd+J/K from here.
export function listStep(e, isMac = mac) {
  return (primary(e, isMac) && !e.altKey && !e.shiftKey && { KeyJ: 1, KeyK: -1 }[e.code]) || 0;
}

// Opens a closed patch box list (Show all): plain arrows, or Ctrl/Cmd+J/K as in an open list.
export function browseStep(e) {
  if (e.ctrlKey || e.metaKey || e.altKey) return listStep(e);
  return e.shiftKey ? 0 : { ArrowDown: 1, ArrowUp: -1 }[e.key] || 0;
}

// Step for a search box's result list: arrows or Ctrl/Cmd+J/K. 0 when there are no results, so the key falls through to
// other handlers (Settings steps sections then).
export function resultStep(e, hasResults) {
  if (!hasResults) return 0;
  if (e.ctrlKey || e.metaKey || e.altKey) return listStep(e);
  return { ArrowDown: 1, ArrowUp: -1 }[e.key] || 0;
}

// Ctrl-click (also Cmd-click on a Mac) on a destination adds it as + does. Same modifier rule as listStep.
export const addClick = (e) => !e.altKey && !e.shiftKey && primary(e);

const MODS = ['ctrl', 'alt', 'shift', 'meta', 'mod'];
const GLYPH = { ctrl: '⌃', alt: '⌥', shift: '⇧', meta: '⌘', mod: '⌘' };
const WORD = { ctrl: 'Ctrl', alt: 'Alt', shift: 'Shift', meta: 'Meta', mod: 'Ctrl' };
const SHOWN = { up: '↑', down: '↓', left: '←', right: '→', enter: 'Enter', escape: 'Esc', esc: 'Esc', home: 'Home', end: 'End' };
Object.assign(SHOWN, { pageup: 'PgUp', pagedown: 'PgDn', space: 'Space', tab: 'Tab', delete: 'Delete', backspace: 'Backspace', click: 'click' });
const ARIA = { up: 'ArrowUp', down: 'ArrowDown', left: 'ArrowLeft', right: 'ArrowRight', esc: 'Escape', pageup: 'PageUp', pagedown: 'PageDown' };

// A spec is chord syntax plus mod: 'ctrl+shift+s', 'mod+enter', 'alt' (a bare modifier), 'ctrl+?'. The key can be '+'.
function parseSpec(spec) {
  const mods = [];
  let rest = spec;
  for (;;) {
    const m = MODS.find((x) => rest === x || rest.startsWith(`${x}+`));
    if (!m) break;
    mods.push(m);
    rest = rest.slice(m.length + 1);
  }
  return { mods, key: rest };
}

// Every shown key label comes from here (test/keylabels.test.mjs). Shift alone is ⇧ on every OS.
export function keyLabel(spec, isMac = mac) {
  const { mods, key } = parseSpec(spec);
  const shown = SHOWN[key] ?? key;
  if (isMac || mods.every((m) => m === 'shift')) return mods.map((m) => GLYPH[m]).join('') + shown;
  return [...mods.map((m) => WORD[m]), ...(key ? [shown] : [])].join('+');
}

// The aria-keyshortcuts form: Control+E, Alt+Shift+H.
export function ariaKeys(spec, isMac = mac) {
  const { mods, key } = parseSpec(spec);
  const name = { ctrl: 'Control', alt: 'Alt', shift: 'Shift', meta: 'Meta', mod: isMac ? 'Meta' : 'Control' };
  const k = ARIA[key] ?? (key.length === 1 ? key.toUpperCase() : key[0].toUpperCase() + key.slice(1));
  return [...mods.map((m) => name[m]), k].join('+');
}

// A Mac's Ctrl-click fires contextmenu and never click. add is optional: without it every click picks.
export function pickOrAdd(b, pick, add) {
  b.addEventListener('click', (e) => (add && addClick(e) ? add() : pick()));
  if (!add || !mac) return;
  b.addEventListener('contextmenu', (e) => {
    if (!addClick(e)) return;
    e.preventDefault();
    add();
  });
}

// Window blur clears both: the key-up of an Alt-Tab never arrives.
const HELD = { shift: 'shiftKey', alt: 'altKey' };
function held(e) {
  const root = document.documentElement;
  for (const [name, prop] of Object.entries(HELD)) root.toggleAttribute(`data-${name}`, !!e?.[prop]);
}
if (typeof document !== 'undefined' && document.addEventListener) {
  document.addEventListener('keydown', held, true);
  document.addEventListener('keyup', held, true);
  globalThis.addEventListener?.('blur', () => held(null));
}
