// Imports nothing, so tests can load it alone. chord(e) names a keystroke the same way everywhere; one set of listeners tracks html[data-shift] and html[data-alt].

export const mac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);

// Letters and digits by e.code (Option+P is 'p', not 'π').
export function chord(e) {
  const m = /^(?:Digit|Key)(.)$/.exec(e.code || '');
  const key = m ? m[1].toLowerCase() : /^Arrow(.+)$/.exec(e.key || '')?.[1].toLowerCase() || (e.key || '').toLowerCase();
  return `${e.ctrlKey ? 'ctrl+' : ''}${e.altKey ? 'alt+' : ''}${e.shiftKey ? 'shift+' : ''}${e.metaKey ? 'meta+' : ''}${key}`;
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
