// The Confirm for Ctrl+Shift+L. Sign out ends the sign-in in every tab of the session (users-locks.md). The key is one
// away from Ctrl+L, so it asks with the churning letter and lockout. Its own modal, not #confirm: that one is the
// take Confirm's (desktop.js), and lock.js also runs on pages without it.
import { el, S } from './core.js';
import { btn } from './formbits.js';
import { modal } from './modal.js';
import { lockout } from './lockout.js';
import { churnKey } from './lockrules.js';

let dlg = null;
let cur = null; // { key, lock, go }
let lastKey = '';
let back = null;

function close() {
  cur?.lock.cancel();
  cur = null;
  dlg.hide();
  if (back?.isConnected) back.focus({ preventScroll: true });
  back = null;
}

function build() {
  const text = el('p', { id: 'signout-ask-text' });
  const prompt = el('div', 'sode__prompt');
  const ok = btn('', '', () => cur?.lock.ready() && cur.go(), 'signout-ask-ok');
  dlg = modal({
    titleId: 'signout-ask-title',
    title: '',
    cls: 'sode__filters sode__pinprompt',
    body: [text, prompt],
    foot: [btn('pb-btn--outline', 'Cancel', close, 'signout-ask-cancel'), ok],
    onEsc: close,
    describedBy: 'signout-ask-text',
  });
  dlg.root.addEventListener('keydown', (e) => {
    if (!cur || e.repeat || e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.key.toLowerCase() !== cur.key && !(e.key === 'Enter' && S.cfg?.confirmEnter && !e.target.closest?.('button'))) return;
    e.preventDefault();
    if (cur.lock.ready()) cur.go();
  });
  return { text, prompt, ok };
}

let parts = null;

// stepped: the tab is stepped in, and the same key steps out (the server's userSignOut does the same, users-locks.md).
export function askSignOut({ user, stepped }, run) {
  parts ??= build();
  const verb = stepped ? 'Step out' : 'Sign out';
  const key = churnKey(lastKey);
  lastKey = key;
  const keyEl = el('kbd', 'sode__key sode__key--locked', key.toUpperCase());
  dlg.title.textContent = `Confirm ${verb.toLowerCase()}`;
  parts.text.textContent = stepped
    ? `This tab stops acting as ${user} and goes back to the sign-in from before the Step in.`
    : `This signs out ${user} in every tab that shares this sign-in.`;
  parts.prompt.replaceChildren('Press ', keyEl, S.cfg?.confirmEnter ? ' or Enter' : '', ' to confirm, Esc to cancel.');
  parts.ok.textContent = `${verb} (${key.toUpperCase()})`;
  parts.ok.disabled = true;
  cur?.lock.cancel();
  const a = document.activeElement;
  if (!dlg.shown() && a && a !== document.body) back = a;
  cur = {
    key,
    go: () => {
      close();
      run();
    },
    lock: lockout(() => {
      keyEl.classList.remove('sode__key--locked');
      parts.ok.disabled = false;
    }),
  };
  dlg.show();
  dlg.dialog.focus();
}
