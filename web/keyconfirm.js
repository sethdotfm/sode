// Desktop confirm dialog (#confirm): churning key, lockout, Esc. The page wires its keys and decides what a close or send does.
// The caller supplies the plan with its wording and summary, so a page without the plan module can use it.

import { $, el } from './core.js';
import { lockout } from './lockout.js';
import { churnKey } from './lockrules.js';

// verb(plan) is the word for the title and button; summary(plan, { prot, spaceHint }) the body nodes. spaceHint: the
// undo's follows box says Space ticks it. enter(): Enter also confirms. cfg(): settings with confirmLockoutMs, else
// S.cfg. viaKeys: answered from the keyboard. refocus: a close puts focus back where it was.
export function keyConfirm({ prompt, verb: verbOf, summary, enter = () => false, cfg = () => undefined, spaceHint = false, refocus = false, onTake, onClose }) {
  let cur = null; // { plan, prot, key, lock }
  let lastKey = '';
  let back = null; // what had focus before the dialog opened

  function end() {
    const c = cur;
    c?.lock.cancel();
    cur = null;
    const a = document.activeElement;
    const inside = !a || a === document.body || $('confirm').contains(a);
    $('confirm').hidden = true;
    // A hidden dialog drops focus to body; the caller's onTake or onClose can still move it.
    if (refocus && inside && back?.isConnected) back.focus({ preventScroll: true });
    back = null;
    return c;
  }

  const api = {
    isOpen: () => cur != null,

    open(plan, prot) {
      cur?.lock.cancel(); // one replaced: its lockout must not unlock this one
      const a = document.activeElement;
      if (!cur && a && a !== document.body && !$('confirm').contains(a)) back = a;
      const key = churnKey(lastKey);
      lastKey = key;
      const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
      const verb = verbOf(plan);
      $('confirm-title').textContent = `Confirm ${verb}`;
      $('confirm-body').replaceChildren(...summary(plan, { prot, spaceHint }), el('div', 'sode__prompt', ...prompt(keyEl)));
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
        }, cfg()),
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
      if (e.key.toLowerCase() === cur.key || (e.key === 'Enter' && enter())) api.take(true);
    },
  };

  $('confirm-cancel').addEventListener('click', () => api.close());
  $('confirm-ok').addEventListener('click', () => api.take());
  return api;
}
