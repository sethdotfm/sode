// Keys, in one place: `chord(e)` names a keystroke the same way everywhere
// ('alt+p', 'ctrl+alt+j', 'alt+3', 'shift+enter'), and the held-modifier flags
// on <html> (`data-shift`, `data-alt`) are tracked by one set of listeners.
// Imports nothing, so tests can load it alone.

// True on a Mac, iPhone or iPad: ⌘ and ⌥ in labels, ⌘ as the command key.
export const mac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);

// Modifier prefix in a fixed order, then the key: a digit or letter by its
// physical key (e.code, so Option+P is 'p' and not 'π'), the arrows as
// up/down/left/right, anything else by e.key in lowercase.
export function chord(e) {
  const m = /^(?:Digit|Key)(.)$/.exec(e.code || '');
  const key = m ? m[1].toLowerCase() : /^Arrow(.+)$/.exec(e.key || '')?.[1].toLowerCase() || (e.key || '').toLowerCase();
  return `${e.ctrlKey ? 'ctrl+' : ''}${e.altKey ? 'alt+' : ''}${e.shiftKey ? 'shift+' : ''}${e.metaKey ? 'meta+' : ''}${key}`;
}

// html[data-shift] / html[data-alt] while Shift / Option-Alt is held (the Next
// chips open, the preset chips show their keys). Window blur clears both: the
// key-up of an Alt-Tab never arrives.
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
