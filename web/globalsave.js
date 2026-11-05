// Saving filter sets and presets from a dialog: ONE saver that does the
// whole flow (the request and its answer, notes, a lost socket), in two
// flavours that differ only in how a change is sent:
//   globalSaver  for everyone: a change to a server-side setting, so with a
//                settings PIN it asks for it first (PIN row, relock after).
//   profileSaver the logged-in user's own profile: no PIN, since a user may
//                always change their own.
// The dialog supplies how it shows a note (and, for the PIN, its controls).

import { on, onMsg, wsSend, sentence } from './core.js';
import { unlock, relock, needsPin, demandPin } from './pin.js';
import { showResult } from './plans.js';
import { profile, saveProfile } from './profile.js';

// The server caps a whole profile at 48 KiB (the socket read limit is 64).
export const PROFILE_MAX_BYTES = 48 * 1024;

// Why `parts` ({filterSets}, {presets}) would make the profile `p` too big
// to save, or ''. Parts replace the same-named part of p.
export function profileProblem(p, parts) {
  const next = { prefs: p.prefs, filterSets: p.filterSets, presets: p.presets, pinOrder: p.pinOrder, ...parts };
  // UTF-8 bytes as the server counts them: Go's json.Marshal also writes
  // < > & as <, >, & (5 more bytes each).
  const json = JSON.stringify(next);
  const bytes = new TextEncoder().encode(json).length + 5 * (json.match(/[<>&]/g) || []).length;
  return bytes > PROFILE_MAX_BYTES
    ? `Your profile is too big to save (${Math.ceil(bytes / 1024)} KiB, at most ${PROFILE_MAX_BYTES / 1024} KiB). Delete something first.`
    : '';
}

let seq = 0; // shared: ids stay unique across savers of one result type

// One onMsg per result type, shared by every saver of that type (the preset
// editor and the Ctrl+G popup both save presets): each ignores ids not its own.
const resultHandlers = new Map();
function onResult(resultT, fn) {
  let list = resultHandlers.get(resultT);
  if (!list) {
    list = [];
    resultHandlers.set(resultT, list);
    onMsg(resultT, (m) => { for (const f of list) f(m); });
  }
  list.push(fn);
}

// The shared flow. `how` is { pin, check(payload), send(pending, payload,
// settle) }: pin is { input, row, ok } or null; check returns why a payload
// can't be saved or ''; send starts the request and returns false if it
// couldn't go out (otherwise settle(result) is called with { ok, err,
// locked }). A result is { ok, err, locked }.
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
    const settle = (r) => { if (pending === p) answer(p, r); };
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
      demandPin();
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
    // Closed while it was saving: say if it didn't (and lock up again).
    if (!r.ok) showResult('error', failTitle, sentence(r.err) || (pin ? 'Settings are locked.' : 'Not saved.'));
    if (pin) relock();
  }

  how.listen?.((m) => { if (pending?.sent && m.id === pending.id) answer(pending, m); });

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
      if (!r.ok) { setNote(r.err, true); pin.input.select(); return; }
      pin.row.hidden = true;
      pending?.run();
    };
    pin.ok.addEventListener('click', sendPin);
    pin.input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') { e.preventDefault(); sendPin(); }
      if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); cancel(); }
    });
  }

  return {
    change,
    cancel,
    busy: () => pending != null,
    // The dialog closes. True when a save already sent is still to be
    // answered (a PIN saver locks up after it); otherwise anything waiting
    // for the PIN is dropped and settings lock again.
    close() {
      if (pin) pin.row.hidden = true;
      if (pending?.sent) return true;
      pending = null;
      if (pin) relock();
      return false;
    },
  };
}

// t: the message sent ({ t, ...payload, id }; a payload's own t wins) and
// resultT: the answer ({ id, ok, err, locked }). pinIn/pinRow/pinOk: the
// PIN row's input, row and Unlock button. setNote(text, isError). isOpen():
// the dialog is showing (after it closed, a failure is a result banner).
// render(): redraw what depends on busy(). okNote: shown after a save.
// failTitle: the banner if a save fails after the dialog closed.
// Returns { change(payload, done), cancel(), close(), busy() }.
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

// For 'Me': change(parts, done) with parts { filterSets } or { presets },
// the whole list. Same options as globalSaver without t/resultT/pin*.
// target (optional): () => { user, profile } | null, for an editor aimed at
// one profile (the logged-in user's, or another's as a trusted user): the
// save goes to that user and the size check is made against that profile as
// last read. Without it (or while it gives null) the own profile is saved.
export function profileSaver({ target, ...opts }) {
  const tgt = () => target?.() || null;
  return makeSaver({
    ...opts,
    how: {
      pin: null,
      check: (parts) => profileProblem(tgt()?.profile || profile(), parts),
      send(p, parts, settle) { saveProfile(parts, tgt()?.user || undefined).then(settle); },
    },
  });
}
