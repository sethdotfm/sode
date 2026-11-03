// The churning-letter confirm of the desktop page and the grid page: a
// random key, a lockout before it works, Esc to cancel. It fills the page's
// #confirm dialog; the page wires its keys (so it decides when a keystroke
// belongs to the confirm) and says what happens when it closes or sends.

import { $, el, S } from './core.js';
import { planSummary, planVerb } from './plans.js';
import { lockout } from './widgets.js';

const KEYS = 'abcdefghjkmnpqrstuvwxyz';

// prompt(keyNode): the nodes of the line under the summary. spaceHint:
// the undo's follows box says Space ticks it. onTake(plan, viaKeys, prot):
// the confirm was accepted and has closed; send the plan. onClose(plan,
// viaKeys): it was cancelled. viaKeys: answered from the keyboard.
// Returns { open(plan, prot), close(viaKeys), take(viaKeys), key(e), isOpen() }.
export function keyConfirm({ prompt, spaceHint = false, onTake, onClose }) {
  let cur = null; // { plan, prot, key, lock }
  let lastKey = '';

  function end() {
    const c = cur;
    c?.lock.cancel();
    cur = null;
    $('confirm').hidden = true;
    return c;
  }

  const api = {
    isOpen: () => cur != null,

    open(plan, prot) {
      cur?.lock.cancel(); // one replaced: its lockout mustn't unlock this one
      let key;
      do key = KEYS[Math.floor(Math.random() * KEYS.length)]; while (key === lastKey);
      lastKey = key;
      const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
      const verb = planVerb(plan);
      $('confirm-title').textContent = `Confirm ${verb}`;
      $('confirm-body').replaceChildren(
        ...planSummary(plan, { prot, spaceHint }),
        el('div', 'sode__prompt', ...prompt(keyEl)),
      );
      $('confirm-ok').textContent = `${verb[0].toUpperCase()}${verb.slice(1)} (${key.toUpperCase()})`;
      $('confirm-ok').disabled = true;
      $('confirm').hidden = false;
      $('confirm-dialog').focus();
      cur = { plan, prot, key, lock: lockout(() => {
        keyEl.classList.remove('sode__key--locked');
        $('confirm-ok').disabled = false;
      }) };
    },

    close(viaKeys = false) {
      const c = end();
      onClose(c?.plan, viaKeys);
    },

    // Accepts it once the lockout has passed.
    take(viaKeys = false) {
      if (!cur?.lock.ready()) return;
      const c = end();
      onTake(c.plan, viaKeys, c.prot);
    },

    // A keydown while the confirm is open: it takes them all.
    key(e) {
      e.preventDefault();
      e.stopPropagation();
      if (e.key === 'Escape') return api.close(true);
      if (e.repeat || e.ctrlKey || e.metaKey || e.altKey) return;
      // Space ticks the box that adds an undo's follows.
      if (e.key === ' ' && cur.plan.extra) {
        $('confirm-body').querySelector('.sode__plan-extra input')?.click();
        return;
      }
      if (e.key.toLowerCase() === cur.key || (S.cfg.confirmEnter && e.key === 'Enter')) api.take(true);
    },
  };

  $('confirm-cancel').addEventListener('click', () => api.close());
  $('confirm-ok').addEventListener('click', () => api.take());
  return api;
}
