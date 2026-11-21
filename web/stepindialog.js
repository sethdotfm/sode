// The Step in and Step out controls in Settings > My user, the Step in popup and the "Step out?" question on
// links that leave the page. stepin.js holds the state.
import { $, el, S, on } from './core.js';
import { btn } from './formbits.js';
import { modal } from './modal.js';
import { pinPrompt } from './pinprompt.js';
import { layout } from './layout.js';
import { sendStepIn, stepOut, steppedIn } from './stepin.js';
import { stepInOffered, stepsOut } from './stepinrules.js';

let seq = 0;
let waiting = 0;

const prompt = pinPrompt({
  id: 'stepin',
  title: 'Step in: this tab only',
  go: 'Step in',
  name: { placeholder: 'Your name', label: 'Your name', missing: 'Enter your name.' },
  pin: { placeholder: 'Your PIN', 'aria-label': 'Your PIN' },
  send(pin, name) {
    const id = ++seq;
    if (!sendStepIn(name, pin, id)) return false;
    waiting = id;
    return true;
  },
  onClose: () => (waiting = 0),
});

on('stepInResult', (m) => {
  if (m.id !== waiting) return;
  waiting = 0;
  if (m.ok) prompt.close();
  else prompt.fail(m.err || 'Wrong PIN.');
});

let leave = null;
let leaveFrom = null;

function askLeave(a) {
  if (!leave) {
    const cancel = () => {
      leave.hide();
      if (leaveFrom?.isConnected) leaveFrom.focus();
    };
    const go = () => {
      leave.hide();
      location.href = leaveFrom.href;
    };
    leave = modal({
      titleId: 'stepout-ask-title',
      title: 'Step out?',
      cls: 'sode__filters sode__pinprompt',
      foot: [btn('pb-btn--outline', 'Cancel', cancel, 'stepout-ask-cancel'), btn('', 'Step out', go, 'stepout-ask-go')],
      onEsc: cancel,
    });
  }
  leaveFrom = a;
  leave.show();
  $('stepout-ask-go').focus();
}

export function initStepIn() {
  const note = $('account-step-note');
  let was = false;
  on('access', (L) => {
    const stepped = steppedIn();
    const offered = stepInOffered({ locked: L.locked, phone: layout() === 'phone', stepped, user: L.user, users: S.users });
    $('account-stepin').hidden = !offered;
    $('account-stepout').hidden = !stepped;
    if (stepped) {
      $('account-name').replaceChildren('Stepped in as ', el('strong', null, L.user));
      for (const id of ['account-login', 'account-logout']) $(id).hidden = true;
    }
    if (stepped) note.hidden = true;
    // Every Settings save sends a lockState, which must not drop a half-typed Step in.
    if (!stepped && (was || !offered)) prompt.close();
    was = stepped;
  });
  on('stepEnded', (m) => {
    note.textContent = m.note || '';
    note.hidden = !m.note;
  });
  $('account-stepin').addEventListener('click', () => prompt.open());
  $('account-stepout').addEventListener('click', () => stepOut());
  document.addEventListener(
    'click',
    (e) => {
      const a = e.target.closest?.('a[href]');
      if (!a || !steppedIn() || e.defaultPrevented || e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
      if (!stepsOut(a.href, location.href, a.target)) return;
      e.preventDefault();
      askLeave(a);
    },
    true,
  );
}
