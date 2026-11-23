// The inline Confirm for changes to a panel's keys: the churning letter behind the confirm lockout, answered by the
// letter or its button. Rotate and Revoke (panelkeytab.js) and the Authentication mode switch (actsguard.js) share it.
import { el, S } from './core.js';
import { btn } from './formbits.js';
import { churnKey, lockoutMs } from './lockrules.js';

// The letter answers while focus is anywhere inside root. ask resolves true on the Confirm and false on Cancel or when a
// newer ask or cancel() replaces it.
export function confirmBox(root) {
  const node = el('div', 'sode-ov__keyconfirm');
  node.hidden = true;
  node.tabIndex = -1;
  let cur = null;

  function close(c, ok) {
    if (cur !== c) return;
    cur = null;
    clearTimeout(c.lock);
    node.hidden = true;
    node.replaceChildren();
    c.done(ok);
  }

  function ask(text, verb) {
    if (cur) close(cur, false);
    const key = churnKey();
    const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
    const c = { key, ready: false, lock: 0, done: null };
    const answer = new Promise((res) => (c.done = res));
    const ok = btn('sode-btn--primary sode__confirm-ok', `${verb} (${key.toUpperCase()})`, () => c.ready && close(c, true));
    ok.disabled = true;
    const cancel = btn('pb-btn--outline', 'Cancel', () => close(c, false));
    c.go = () => c.ready && close(c, true);
    c.lock = setTimeout(() => {
      c.ready = true;
      keyEl.classList.remove('sode__key--locked');
      ok.disabled = false;
    }, lockoutMs(S.cfg));
    cur = c;
    node.replaceChildren(el('p', null, text), el('div', 'sode__prompt', 'Press', keyEl, 'to confirm', ok, cancel));
    node.hidden = false;
    node.focus();
    return answer;
  }

  root.addEventListener('keydown', (e) => {
    if (!cur || e.ctrlKey || e.metaKey || e.altKey || e.key.toLowerCase() !== cur.key) return;
    e.preventDefault();
    e.stopPropagation();
    cur.go();
  });

  return { node, ask, cancel: () => cur && close(cur, false) };
}
