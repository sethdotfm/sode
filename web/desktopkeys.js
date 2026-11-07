// Desktop document key handling: Ctrl+?, Ctrl+comma, Ctrl+S/D/F fills, Ctrl+Z, box keys, Tab ring, Esc.

import { $ } from './core.js';
import { mac, chord } from './keys.js';
import { layout } from './layout.js';
import { flipped } from './device.js';

// Modifier rule: see AGENTS.md. Each check excludes the other two modifiers.
// dsts is an accessor: desktop.js replaces the array. historyView is what desktopHistory returned.
export function desktopKeys({
  boxes,
  sel,
  dsts,
  lvl,
  confirm,
  keys,
  historyView,
  active,
  select,
  addDst,
  useLast,
  wantsMatch,
  patchNow,
  clearAll,
  renderTally,
}) {
  const { histList, histArea, onListKey, enterKeys, undoLast } = historyView;
  // Modifier rule: see AGENTS.md. Each check excludes the other two modifiers.
  document.addEventListener('keydown', (e) => {
    if (confirm.isOpen()) return confirm.key(e);
    if (layout() !== 'desktop') return;
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

    if (b && !b.cb.open && (chord(e) === 'down' || chord(e) === 'up')) {
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
        const ring = flipped() ? ['dst', 'src', 'lvl'] : ['src', 'dst', 'lvl'];
        const i = ring.indexOf(id);
        if (i >= 0) {
          e.preventDefault();
          $(ring[(i + (e.shiftKey ? ring.length - 1 : 1)) % ring.length]).focus();
        }
      }
      return;
    }
    if (e.target === histList || e.target === document.body) {
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
    if (e.key === 'Escape') {
      if (b && b.cb.open) {
        b.cb.close();
        return;
      }
      if (e.target.value) {
        if (k) select(k, null);
        else lvl.clear();
        renderTally();
      } else if (!dsts().length && !boxes.src.input.value && !boxes.dst.input.value && lvl.empty()) {
        enterKeys();
      } else {
        clearAll();
      }
    }
  });
}
