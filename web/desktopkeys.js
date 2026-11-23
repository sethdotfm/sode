import { $, emit } from './core.js';
import { chord, modChord, undoKey, redoKey, ctrlChar, browseStep } from './keys.js';
import { layout } from './layout.js';
import { flipped, toggleCompact } from './device.js';
import { levelsShown } from './levelview.js';

// A box with text keeps the OS text-undo and text-redo chords for its typing.
const typedUndo = (e, undo) => undo.own && e.target instanceof HTMLInputElement && !!e.target.value;
// [box, Next]. Levels has no Next.
const LAST = { 'ctrl+s': ['src', false], 'ctrl+d': ['dst', false], 'ctrl+f': ['lvl', false], 'ctrl+shift+s': ['src', true], 'ctrl+shift+d': ['dst', true] };

// dsts is an accessor: desktop.js replaces the array. historyView is what desktopHistory returned. grid is the Grid
// popup's { shown, toggle } (gridpopup.js). clearEverything is the one Clear: the boxes and the staged routes.
export function desktopKeys({
  boxes,
  dsts,
  confirm,
  keys,
  historyView,
  active,
  select,
  addDst,
  useLast,
  wantsMatch,
  patchNow,
  clearEverything,
  nothingToClear,
  grid,
}) {
  const { histList, histArea, onListKey, enterKeys, keyMode, undoLast, redoLast, historyShown } = historyView;
  // Ctrl+Z steps back, Ctrl+Shift+Z forward (stepback.js): { own, run } or null.
  const stepOf = (e) => {
    const undo = undoKey(e);
    if (undo) return { ...undo, run: undoLast };
    const redo = redoKey(e);
    return redo && { ...redo, run: redoLast };
  };
  // Modifier rule: docs/agents/web-layout.md (Keys). Each check excludes the other two modifiers.
  document.addEventListener('keydown', (e) => {
    if (confirm.isOpen()) return confirm.key(e);
    if (layout() !== 'desktop' || document.documentElement.hasAttribute('data-sode-setup')) return;
    // Ctrl+? toggles the shortcut list anywhere, even mid-typing. Matched by character (any
    // layout) and by the / key, for browsers reporting e.key without Shift under Ctrl.
    if (ctrlChar(e, '?') || (ctrlChar(e, e.key) && e.shiftKey && e.code === 'Slash')) {
      e.preventDefault();
      e.stopImmediatePropagation(); // not Settings' own keys
      if (keys.shown()) keys.close(true);
      else keys.open();
      return;
    }
    if (!$('keys').hidden) {
      e.stopImmediatePropagation(); // Esc here closes the list, not Settings
      if (e.key === 'Escape' || e.key === '?') {
        e.preventDefault();
        keys.close(true);
      }
      // Keep focus in the dialog: Close is its only control.
      if (e.key === 'Tab') {
        e.preventDefault();
        $('keys-close').focus();
      }
      return;
    }
    if (!active()) return;
    // Ctrl+e on every OS. Chrome and Firefox give it to the page, as Google Docs uses it.
    if (chord(e) === 'ctrl+e' && !document.documentElement.dataset.sodeLock) {
      e.preventDefault();
      if (!e.repeat) grid.toggle();
      return;
    }
    // The open Grid popup has its own keys (gridpanelrun.js). Of the page's, only undo, redo and the key list reach it.
    if (grid.shown()) {
      const step = stepOf(e);
      if (step && !typedUndo(e, step)) {
        e.preventDefault();
        step.run();
      } else if (e.key === '?' && !e.ctrlKey && !e.metaKey && !e.altKey && !(e.target instanceof HTMLInputElement)) {
        e.preventDefault();
        keys.open();
      }
      return;
    }
    // Compact view, also from a box: preventDefault keeps macOS Option+Shift+C from typing Ç.
    if (chord(e) === 'alt+shift+c') {
      e.preventDefault();
      if (!e.repeat) toggleCompact();
      return;
    }
    // Keyboard mode's own key works also from a box. Esc with nothing to clear starts it too (below). A second press goes
    // back to Source. Compact view has no keyboard mode: History is hidden. preventDefault keeps macOS Option+Shift+K
    // from typing a character.
    if (chord(e) === 'alt+shift+k') {
      e.preventDefault();
      if (e.repeat || !historyShown()) return;
      if (keyMode()) boxes.src.input.focus();
      else enterKeys();
      return;
    }
    if (ctrlChar(e, ',')) {
      e.preventDefault();
      emit('settingsOpen');
      return;
    }
    const last = LAST[chord(e)];
    if (last) {
      e.preventDefault();
      useLast(...last);
      return;
    }
    const id = e.target.id;
    const k = id === 'src' || id === 'dst' ? id : null;
    const b = k && boxes[k];
    const inBox = k || id === 'lvl';

    const step = stepOf(e);
    if (step) {
      if (typedUndo(e, step)) return;
      e.preventDefault();
      step.run();
      return;
    }

    if (b && !b.cb.open && browseStep(e)) {
      e.preventDefault();
      b.all.open();
      return;
    }
    if (b && b.cb.nav(e)) return;
    if (k === 'dst' && modChord(e) === 'mod+enter') {
      e.preventDefault();
      addDst();
      return;
    }
    // + adds another destination; Ctrl/Cmd + is left for zoom.
    if (k === 'dst' && e.key === '+' && !e.ctrlKey && !e.metaKey) {
      e.preventDefault();
      addDst();
      return;
    }
    if (k === 'dst' && e.key === 'Backspace' && !b.input.value && dsts().length) {
      e.preventDefault();
      select('dst', dsts().pop());
      return;
    }
    if (e.key === 'Tab') {
      if (b && wantsMatch(k) && !b.cb.accept()) {
        e.preventDefault();
        b.input.classList.add('pb-input--invalid');
        return;
      }
      if (inBox) {
        const ring = (flipped() ? ['dst', 'src', 'lvl'] : ['src', 'dst', 'lvl']).filter((x) => x !== 'lvl' || levelsShown());
        const i = ring.indexOf(id);
        if (i >= 0) {
          e.preventDefault();
          $(ring[(i + (e.shiftKey ? ring.length - 1 : 1)) % ring.length]).focus();
        }
      }
      return;
    }
    // Compact view has no keyboard mode: History is hidden.
    if ((e.target === histList || e.target === document.body) && historyShown()) {
      if (onListKey(e)) e.preventDefault();
      return;
    }
    // Esc on a History button starts Keyboard mode at that row.
    if (e.key === 'Escape' && e.target !== histList && histArea(e.target)) {
      e.preventDefault();
      enterKeys(e.target.closest('[data-key]')?.dataset.key);
      return;
    }
    if (!inBox) return; // leave buttons to the browser
    if (e.key === 'Enter') {
      e.preventDefault();
      patchNow();
      return;
    }
    // One Esc: an open match list closes first. Then everything clears, as Clear does. Only then Keyboard mode.
    if (e.key === 'Escape') {
      if (b && b.cb.open) {
        b.cb.close();
        return;
      }
      if (!nothingToClear()) clearEverything();
      else if (historyShown()) enterKeys();
    }
  });
}
