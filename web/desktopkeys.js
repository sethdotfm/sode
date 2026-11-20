import { $ } from './core.js';
import { mac, chord, browseStep } from './keys.js';
import { layout } from './layout.js';
import { flipped, toggleCompact } from './device.js';
import { levelsShown } from './levelview.js';

// Ctrl+Z, or Cmd+Z on a Mac.
const undoChord = (e) => ((mac && e.metaKey && !e.ctrlKey) || (e.ctrlKey && !e.metaKey)) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'z';

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
  const { histList, histArea, onListKey, enterKeys, undoLast, historyShown } = historyView;
  // Modifier rule: docs/agents/web-layout.md (Keys). Each check excludes the other two modifiers.
  document.addEventListener('keydown', (e) => {
    if (confirm.isOpen()) return confirm.key(e);
    if (layout() !== 'desktop' || document.documentElement.hasAttribute('data-sode-setup')) return;
    // Ctrl+? toggles the shortcut list anywhere, even mid-typing. Matched by character (any
    // layout) and by the / key, for browsers reporting e.key without Shift under Ctrl.
    if (e.ctrlKey && !e.metaKey && !e.altKey && (e.key === '?' || (e.shiftKey && e.code === 'Slash'))) {
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
    // The open Grid popup has its own keys (gridpanelrun.js). Of the page's, only undo and the key list reach it.
    if (grid.shown()) {
      // As below: the OS text-undo chord leaves a search box with text to undo its typing.
      if (undoChord(e) && !((mac && e.metaKey) === mac && e.target instanceof HTMLInputElement && e.target.value)) {
        e.preventDefault();
        undoLast();
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
    if (e.ctrlKey && !e.metaKey && !e.altKey && e.key === ',') {
      e.preventDefault();
      $('settings-open').click();
      return;
    }
    const last = e.ctrlKey && !e.metaKey && !e.altKey && { KeyS: 'src', KeyD: 'dst', KeyF: 'lvl' }[e.code];
    if (last && (!e.shiftKey || last !== 'lvl')) {
      e.preventDefault();
      useLast(last, e.shiftKey);
      return;
    }
    const id = e.target.id;
    const k = id === 'src' || id === 'dst' ? id : null;
    const b = k && boxes[k];
    const inBox = k || id === 'lvl';

    // Ctrl+Z / ⌘Z undoes the last Sode take; the OS text-undo chord leaves a box with text to
    // undo its typing.
    const cmdZ = mac && e.metaKey && !e.ctrlKey;
    if ((cmdZ || (e.ctrlKey && !e.metaKey)) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'z') {
      if (cmdZ === mac && e.target instanceof HTMLInputElement && e.target.value) return;
      e.preventDefault();
      undoLast();
      return;
    }

    if (b && !b.cb.open && browseStep(e)) {
      e.preventDefault();
      b.all.open();
      return;
    }
    if (b && b.cb.nav(e)) return;
    if (k === 'dst' && e.key === 'Enter' && e.ctrlKey !== e.metaKey && !e.shiftKey && !e.altKey) {
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
