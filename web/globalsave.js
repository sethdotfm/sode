// One saver flow for filter sets and presets. globalSaver: server-side setting, so with a PIN it asks first (PIN row, relock after).
// profileSaver: the user's own profile, no PIN.

import { on, onMsg, wsSend, sentence } from './core.js';
import { unlock, relock, needsPin } from './pin.js';
import { showResult } from './plans.js';
import { profile, saveProfile } from './profile.js';

export { PROFILE_MAX_BYTES, profileProblem } from './profilesrules.js';
import { profileProblem } from './profilesrules.js';

let seq = 0; // shared: ids stay unique across savers of one result type

// One onMsg per result type, shared by its savers; each ignores ids not its own.
const resultHandlers = new Map();
function onResult(resultT, fn) {
  let list = resultHandlers.get(resultT);
  if (!list) {
    list = [];
    resultHandlers.set(resultT, list);
    onMsg(resultT, (m) => {
      for (const f of list) f(m);
    });
  }
  list.push(fn);
}

// how.send returns false if the request couldn't go out, else calls settle(result).
function makeSaver({ how, setNote, isOpen, failTitle, render = () => {}, okNote = '' }) {
  const { pin } = how;
  let pending = null; // { run, done, sent }

  function change(payload, done) {
    const big = how.check?.(payload);
    if (big) return setNote(big, true);
    pending = { run: () => send(payload), done, sent: false };
    if (pin && needsPin()) return askPin();
    pending.run();
  }

  function send(payload) {
    const p = pending;
    p.sent = true;
    const settle = (r) => {
      if (pending === p) answer(p, r);
    };
    if (how.send(p, payload, settle) === false) {
      pending = null;
      render();
      setNote('Not connected to Sode.', true);
      return;
    }
    render();
    setNote('Saving…');
  }

  function answer(p, r) {
    const open = isOpen();
    if (r.locked && pin) {
      relock();
      if (open) {
        pending = { run: p.run, done: p.done, sent: false };
        return askPin('Enter the PIN again.');
      }
    }
    pending = null;
    if (r.ok) p.done();
    if (open) {
      render();
      if (!r.ok) setNote(sentence(r.err) || (pin ? '' : 'Not saved.'), true);
      else if (okNote) setNote(okNote);
      return;
    }
    // Closed while saving: say if it failed (and relock).
    if (!r.ok) showResult('error', failTitle, sentence(r.err) || (pin ? 'Settings are locked.' : 'Not saved.'));
    if (pin) relock();
  }

  how.listen?.((m) => {
    if (pending?.sent && m.id === pending.id) answer(pending, m);
  });

  // The answer to a save on a dead socket never comes: let it be tried again.
  on('wsclose', () => {
    if (!pending?.sent) return;
    pending = null;
    render();
    if (isOpen()) setNote('Connection lost; try again.', true);
  });

  function askPin(err = '') {
    pin.row.hidden = false;
    pin.input.value = '';
    setNote(err || 'Enter the settings PIN.', !!err);
    pin.input.focus();
  }

  function cancel() {
    if (pin) pin.row.hidden = true;
    pending = null;
    setNote('');
    render();
  }

  if (pin) {
    const sendPin = async () => {
      pin.ok.disabled = true;
      const r = await unlock(pin.input.value);
      if (r.superseded) return;
      pin.ok.disabled = false;
      if (r.stale) return;
      if (!r.ok) {
        setNote(r.err, true);
        pin.input.select();
        return;
      }
      pin.row.hidden = true;
      pending?.run();
    };
    pin.ok.addEventListener('click', sendPin);
    pin.input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        sendPin();
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        cancel();
      }
    });
  }

  return {
    change,
    cancel,
    busy: () => pending != null,
    // Closing: true when a sent save is still to be answered (a PIN saver relocks after it); else drop anything waiting for the PIN and relock.
    close() {
      if (pin) pin.row.hidden = true;
      if (pending?.sent) return true;
      pending = null;
      if (pin) relock();
      return false;
    },
  };
}

// t: message sent (a payload's own t wins); resultT: the answer. isOpen(): after close a failure becomes a result banner.
// okNote: shown after a save; failTitle: banner title if a save fails after close.
export function globalSaver({ t, resultT, pinIn, pinRow, pinOk, ...rest }) {
  return makeSaver({
    ...rest,
    how: {
      pin: { input: pinIn, row: pinRow, ok: pinOk },
      listen: (fn) => onResult(resultT, fn),
      send(p, payload) {
        p.id = ++seq;
        return wsSend({ t, ...payload, id: p.id });
      },
    },
  });
}

// 'Me': change(parts) with { filterSets } or { presets }, the whole list. target() -> { user, profile } | null aims at another
// profile (trusted user), size-checked against it as last read; null or absent saves the own profile.
export function profileSaver({ target, ...opts }) {
  const tgt = () => target?.() || null;
  return makeSaver({
    ...opts,
    how: {
      pin: null,
      check: (parts) => profileProblem(tgt()?.profile || profile(), parts),
      send(p, parts, settle) {
        saveProfile(parts, tgt()?.user || undefined).then(settle);
      },
    },
  });
}
