// Saving something for everyone (filter sets, presets): a change to a
// server-side setting, so with a settings PIN it asks for it first. One
// saver per dialog does the whole flow: the PIN row, the request and its
// answer, locking up again afterwards. The dialog supplies its PIN controls
// and how it shows a note.

import { S, on, onMsg, wsSend, sentence } from './core.js';
import { unlock, relock, needsPin, demandPin } from './pin.js';
import { showResult } from './plans.js';

// t: the message sent ({ t, ...payload, id }; a payload's own t wins) and
// resultT: the answer ({ id, ok, err, locked }). pinIn/pinRow/pinOk: the
// PIN row's input, row and Unlock button. setNote(text, isError). isOpen():
// the dialog is showing (after it closed, a failure is a result banner).
// render(): redraw what depends on busy(). okNote: shown after a save.
// failTitle: the banner if a save fails after the dialog closed.
// Returns { change(payload, done), cancel(), close(), busy() }.
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

export function globalSaver({ t, resultT, pinIn, pinRow, pinOk, setNote, isOpen, failTitle, render = () => {}, okNote = '' }) {
  let pending = null; // { run, done, id once sent }

  // Sends payload now (the PIN, if needed, is already in); done() runs when
  // the server says ok.
  function change(payload, done) {
    pending = { run: () => send(payload), done };
    if (needsPin()) return askPin();
    pending.run();
  }

  function send(payload) {
    pending.id = ++seq;
    if (!wsSend({ t, ...payload, id: pending.id })) {
      pending = null;
      render();
      setNote('Not connected to Sode.', true);
      return;
    }
    render();
    setNote('Saving…');
  }

  onResult(resultT, (m) => {
    if (!pending || m.id !== pending.id) return;
    const p = pending;
    const open = isOpen();
    if (m.locked) {
      relock();
      demandPin();
      if (open) {
        pending = { run: p.run, done: p.done };
        return askPin('Enter the PIN again.');
      }
    }
    pending = null;
    if (m.ok) p.done();
    if (open) {
      render();
      if (!m.ok) setNote(sentence(m.err), true);
      else if (okNote) setNote(okNote);
      return;
    }
    // Closed while it was saving: say if it didn't, then lock up.
    if (!m.ok) showResult('error', failTitle, sentence(m.err) || 'Settings are locked.');
    relock();
  });

  // The answer to a save on a dead socket never comes: let it be tried again.
  on('wsclose', () => {
    if (!pending?.id) return;
    pending = null;
    render();
    if (isOpen()) setNote('Connection lost; try again.', true);
  });

  function askPin(err = '') {
    pinRow.hidden = false;
    pinIn.value = '';
    setNote(err || 'Enter the settings PIN.', !!err);
    pinIn.focus();
  }

  function cancel() {
    pinRow.hidden = true;
    pending = null;
    setNote('');
    render();
  }

  async function sendPin() {
    pinOk.disabled = true;
    const r = await unlock(pinIn.value);
    if (r.superseded) return;
    pinOk.disabled = false;
    if (r.stale) return;
    if (!r.ok) { setNote(r.err, true); pinIn.select(); return; }
    pinRow.hidden = true;
    pending?.run();
  }

  pinOk.addEventListener('click', sendPin);
  pinIn.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); sendPin(); }
    if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); cancel(); }
  });

  return {
    change,
    cancel,
    busy: () => pending != null,
    // The dialog closes. True when a save already sent is still to be
    // answered (its answer locks up after it); otherwise anything waiting
    // for the PIN is dropped and settings lock again.
    close() {
      pinRow.hidden = true;
      if (pending?.id) return true;
      pending = null;
      relock();
      return false;
    },
  };
}
