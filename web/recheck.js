// PIN recheck (GLOSSARY.md, docs/agents/users-locks.md): per connection, for one minute.

import { S, on, onMsg, wsSend, LINK_DOWN } from './core.js';
import { pinPrompt } from './pinprompt.js';

let done = null; // resolves the askRecheck in flight
let seq = 0;
let waiting = 0; // id of the pinRecheck in flight

function settle(ok) {
  const d = done;
  done = null;
  d?.(ok);
}

const prompt = pinPrompt({
  id: 'recheck',
  title: 'PIN recheck',
  go: 'Continue',
  pin: { autocomplete: 'current-password', spellcheck: false, placeholder: 'Your PIN', 'aria-label': 'Your PIN' },
  send(pin) {
    const id = ++seq;
    if (!wsSend({ t: 'pinRecheck', pin, id })) return false;
    waiting = id;
    return true;
  },
  onClose(ok) {
    waiting = 0;
    settle(ok);
  },
  dropCloses: true,
});

const inline = new Map(); // id -> resolve, for recheckPin

onMsg('pinRecheckResult', (m) => {
  const own = inline.get(m.id);
  if (own) {
    inline.delete(m.id);
    return own({ ok: !!m.ok, err: m.err || (m.ok ? '' : 'Wrong PIN.') });
  }
  if (m.id !== waiting) return;
  waiting = 0;
  if (m.ok) prompt.close(true);
  else prompt.fail(m.err || 'Wrong PIN.');
});

// what names the action, for example 'Clearing History'. go: the button's words, the action that the PIN lets go on.
export function askRecheck(what, go = 'Continue') {
  settle(false); // a second ask replaces the first
  prompt.open(`${what} needs your PIN again. Enter your PIN.`, go);
  return new Promise((resolve) => {
    done = resolve;
  });
}

// For a page with its own PIN field, such as the import review: sends the PIN and resolves {ok, err}.
export function recheckPin(pin) {
  const id = ++seq;
  if (!wsSend({ t: 'pinRecheck', pin, id })) return Promise.resolve({ ok: false, err: LINK_DOWN });
  return new Promise((resolve) => inline.set(id, resolve));
}

// A socket drop ends the recheck on the server.
on('conn', () => {
  if (S.wsState === 'up') return;
  for (const done of inline.values()) done({ ok: false, err: LINK_DOWN });
  inline.clear();
});
