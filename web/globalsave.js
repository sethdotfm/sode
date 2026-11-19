import { on, onMsg, wsSend, sentence, LINK_DOWN } from './core.js';
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

// how.send returns false if the request could not go out, else calls settle(result).
function makeSaver({ how, setNote, isOpen, failTitle, render = () => {}, okNote = '' }) {
  let pending = null; // { done, sent, id }

  function change(payload, done) {
    const big = how.check?.(payload);
    if (big) return setNote(big, true);
    pending = { done, sent: false };
    send(payload);
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
      setNote(LINK_DOWN, true);
      return;
    }
    render();
    setNote('Saving…');
  }

  function answer(p, r) {
    const open = isOpen();
    pending = null;
    if (r.ok) p.done();
    if (open) {
      render();
      if (!r.ok) setNote(sentence(r.err) || 'Not saved.', true);
      else if (okNote) setNote(okNote);
      return;
    }
    // The dialog closed before the answer: say if the save failed.
    if (!r.ok) showResult('error', failTitle, sentence(r.err) || 'Not saved.');
  }

  how.listen?.((m) => {
    if (pending?.sent && m.id === pending.id) answer(pending, m);
  });

  // The answer to a save on a dead socket never comes: let it be tried again.
  on('wsclose', () => {
    if (!pending?.sent) return;
    pending = null;
    render();
    if (isOpen()) setNote(LINK_DOWN, true);
  });

  function cancel() {
    pending = null;
    setNote('');
    render();
  }

  return {
    change,
    cancel,
    busy: () => pending != null,
    // Closing: true when a sent save is still to be answered.
    close() {
      if (pending?.sent) return true;
      pending = null;
      return false;
    },
  };
}

// t: message sent (a payload's own t wins); resultT: the answer. isOpen(): after close a failure becomes a result banner.
// okNote: shown after a save; failTitle: banner title if a save fails after close.
export function globalSaver({ t, resultT, ...rest }) {
  return makeSaver({
    ...rest,
    how: {
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
      check: (parts) => profileProblem(tgt()?.profile || profile(), parts),
      send(p, parts, settle) {
        saveProfile(parts, tgt()?.user || undefined).then(settle);
      },
    },
  });
}

// Save or delete a named item in a scope's list. Sets and presets share the flow, and so does editing another
// user's profile: with target() set, every scope goes to that profile's saver.
//   key: payload key for the whole list (profile saver, and the global one unless globalKey differs).
//   listOf(scope): the scope's current list as it is sent.
//   same(a, b): do two names mean one item?   dropLocal(name): delete from localStorage.
export function scopeWriter({ global, user, target, key, globalKey = key, listOf, same, dropLocal }) {
  // The whole next list into a remote scope; done() runs after it lands. The caller writes local scopes.
  function saveIn(scope, list, done) {
    const t = target?.();
    const [saver, k] = t ? [t, key] : scope === 'user' ? [user, key] : [global, globalKey];
    saver.change({ [k]: list }, done);
  }
  // Without the item; local deletes immediately. done() runs after it is gone.
  function removeIn(scope, name, done) {
    if (scope === 'local') {
      dropLocal(name);
      return done();
    }
    saveIn(
      scope,
      listOf(scope).filter((x) => !same(x.name, name)),
      done,
    );
  }
  return { saveIn, removeIn };
}
