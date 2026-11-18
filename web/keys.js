// Imports nothing, so tests can load it alone. chord(e) names a keystroke the same way everywhere; one set of listeners tracks html[data-shift] and html[data-alt].

export const mac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);

// Letters and digits by e.code (Option+P is 'p', not 'π').
export function chord(e) {
  const m = /^(?:Digit|Key)(.)$/.exec(e.code || '');
  const key = m ? m[1].toLowerCase() : /^Arrow(.+)$/.exec(e.key || '')?.[1].toLowerCase() || (e.key || '').toLowerCase();
  return `${e.ctrlKey ? 'ctrl+' : ''}${e.altKey ? 'alt+' : ''}${e.shiftKey ? 'shift+' : ''}${e.metaKey ? 'meta+' : ''}${key}`;
}

// Keep in step with the combobox and Settings search: both take their Ctrl/Cmd+J/K from here.
export function listStep(e) {
  const mod = (e.ctrlKey && !e.metaKey) || (mac && e.metaKey && !e.ctrlKey);
  return (mod && !e.altKey && !e.shiftKey && { KeyJ: 1, KeyK: -1 }[e.code]) || 0;
}

// Step for a search box's result list: arrows or Ctrl/Cmd+J/K. 0 when there are no results, so the key falls through to
// other handlers (Settings steps sections then).
export function resultStep(e, hasResults) {
  if (!hasResults) return 0;
  if (e.ctrlKey || e.metaKey || e.altKey) return listStep(e);
  return { ArrowDown: 1, ArrowUp: -1 }[e.key] || 0;
}

// Ctrl-click (also Cmd-click on a Mac) on a destination adds it as + does. Same modifier rule as listStep.
export const addClick = (e) => !e.altKey && !e.shiftKey && ((e.ctrlKey && !e.metaKey) || (mac && e.metaKey && !e.ctrlKey));

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
