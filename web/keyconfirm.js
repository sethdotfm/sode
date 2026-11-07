// Desktop confirm dialog (#confirm): churning key, lockout, Esc. The page wires its keys and decides what a close or send does.

import { $, el, S } from './core.js';
import { planSummary, planVerb } from './plans.js';
import { lockout } from './lockout.js';
import { churnKey } from './lockrules.js';

// spaceHint: the undo's follows box says Space ticks it. viaKeys: answered from the keyboard.
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
      const key = churnKey(lastKey);
      lastKey = key;
      const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
      const verb = planVerb(plan);
      $('confirm-title').textContent = `Confirm ${verb}`;
      $('confirm-body').replaceChildren(...planSummary(plan, { prot, spaceHint }), el('div', 'sode__prompt', ...prompt(keyEl)));
      $('confirm-ok').textContent = `${verb[0].toUpperCase()}${verb.slice(1)} (${key.toUpperCase()})`;
      $('confirm-ok').disabled = true;
      $('confirm').hidden = false;
      $('confirm-dialog').focus();
      cur = {
        plan,
        prot,
        key,
        lock: lockout(() => {
          keyEl.classList.remove('sode__key--locked');
          $('confirm-ok').disabled = false;
        }),
      };
    },

    close(viaKeys = false) {
      const c = end();
      onClose(c?.plan, viaKeys);
    },

    take(viaKeys = false) {
      if (!cur?.lock.ready()) return;
      const c = end();
      onTake(c.plan, viaKeys, c.prot);
    },

    key(e) {
      e.preventDefault();
      e.stopPropagation();
      if (!cur) return;
      if (e.key === 'Escape') return api.close(true);
      if (e.repeat || e.ctrlKey || e.metaKey || e.altKey) return;
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
