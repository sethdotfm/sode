// Ctrl+Z and Ctrl+Shift+Z on Home (docs/agents/history.md, Step back): each Ctrl+Z undoes one take further back, and
// Ctrl+Shift+Z redoes what Ctrl+Z undid. Only this browser's takes count: their keys are in `sessionStorage`, so a reload
// keeps them and a new tab starts empty. backStep and forwardStep are pure, so Node tests them.
import { undoIndex, undoRoot, takeMark } from './undone.js';
import { isHop } from './tiehistoryrules.js';
import { readTab, writeTab } from './store.js';

const OWN_KEY = 'sode-own-takes'; // keys of the entries this tab sent, oldest first
const REDO_KEY = 'sode-redo-steps'; // the takes Ctrl+Z undid, each a list of keys, newest last
const OWN_MAX = 1000;
const REDO_MAX = 100;

// History ids start again at 0 after a Clear, and after a restart with no history file. The entry's time with the id
// keeps the take of another tab from matching an old id of this tab.
export const entryKey = (x) => `${x.id}@${x.time}`;
const keyId = (key) => Number.parseInt(key, 10);

// The same rule as the panel Undo cell (panelundo.go). An undo or a Protect set is never a step. A Retry of a failed undo
// is part of that undo (undoRoot), so it is no step too. A take that a later undo put back is passed over (goneOf). A
// Free Protect counts, since its Undo sets the Protect again. A hop entry of a tie take is part of the route asked
// for. The step is the destination entry. Its Undo never switches a tie line back (tieundo.js).
const stepKind = (x, byId) => !x.ext && !isHop(x) && !undoRoot(x, byId) && x.protect !== 'set';

const triedIds = (hist) => new Set(hist.filter((x) => x.undo != null).map((x) => x.undo));

// redo: lists of keys. A key there whose undo has no loaded entry yet is still on its way back, so it is passed over too.
function goneOf(hist, redo) {
  const index = undoIndex(hist);
  const tried = triedIds(hist);
  const waiting = new Set(redo.flat().filter((key) => !tried.has(keyId(key))));
  return (x) => waiting.has(entryKey(x)) || takeMark(x, index)?.state === 'undone';
}

// The take that Ctrl+Z undoes: the newest one of own (keys) that is not gone. Its Group's other entries that are not
// gone come with it. empty(entries) is true when their undo has nothing to send. Such a take is passed over, as the
// panel Undo cell passes over a take with no undo plan. hist is newest first. Answers { e, entries } (entries newest
// first) or null.
export function backStep(hist, own, redo = [], empty = () => false) {
  const mine = new Set(own);
  const byId = new Map(hist.map((x) => [x.id, x]));
  const gone = goneOf(hist, redo);
  const can = (x) => mine.has(entryKey(x)) && stepKind(x, byId) && !gone(x);
  const judged = new Set();
  for (const e of hist) {
    if (!can(e) || judged.has(e.group)) continue;
    const many = e.group && e.groupN > 1;
    if (many) judged.add(e.group);
    const entries = many ? hist.filter((x) => x.group === e.group && can(x)) : [e];
    if (!empty(entries)) return { e, entries };
  }
  return null;
}

// The take that Ctrl+Shift+Z redoes: the newest list in redo that still has a loaded entry and whose undo did not fail on
// every level. keep is redo without the lists passed over, so they do not come back. ids is the list of keys.
export function forwardStep(hist, redo) {
  const byKey = new Map(hist.map((x) => [entryKey(x), x]));
  const index = undoIndex(hist);
  const tried = triedIds(hist);
  const keep = [...redo];
  while (keep.length) {
    const ids = keep[keep.length - 1];
    const entries = ids.map((key) => byKey.get(key)).filter(Boolean);
    const failed = entries.length && entries.every((x) => tried.has(x.id) && !takeMark(x, index));
    if (entries.length && !failed) return { entries, ids, keep };
    keep.pop();
  }
  return { entries: null, ids: null, keep };
}

// A send from this tab. A plan from Ctrl+Z has step 'back' and the keys it undoes (stepIds); one from Ctrl+Shift+Z has
// step 'forward'. Any other take clears the redo list, as in an editor; an undo, a redo, a retry or a Protect does not.
export function nextRedo(redo, plan) {
  // A Protect has no Redo, so its Undo is not kept.
  if (plan.step === 'back') return plan.protect ? redo : [...redo, plan.stepIds].slice(-REDO_MAX);
  if (plan.step === 'forward') return redo.slice(0, -1);
  if (plan.protect || ['undo', 'redo', 'retry'].includes(plan.verb)) return redo;
  return [];
}

// A memory copy answers only while this tab's last write of the key did not land (storage blocked or full).
const mem = new Map();
function read(k) {
  if (mem.has(k)) return mem.get(k);
  try {
    const v = JSON.parse(readTab(k));
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}
function write(k, v) {
  if (writeTab(k, JSON.stringify(v))) mem.delete(k);
  else mem.set(k, v);
}

export const ownTakes = () => read(OWN_KEY).filter((key) => typeof key === 'string');
export const redoSteps = () => read(REDO_KEY).filter(Array.isArray);
export const keepRedo = (redo) => write(REDO_KEY, redo);

// ids: the History ids of entries that this tab's sends wrote (takeResult, the Protect results). The server sends each
// entry before its result, so hist (S.hist) holds it. An id with no loaded entry is not kept.
export function noteOwn(ids, hist) {
  const byId = new Map(hist.map((x) => [x.id, x]));
  const add = ids.filter((id) => byId.has(id)).map((id) => entryKey(byId.get(id)));
  if (add.length) write(OWN_KEY, [...ownTakes(), ...add].slice(-OWN_MAX));
}

export const noteSent = (plan) => keepRedo(nextRedo(redoSteps(), plan));

// History Clear: the ids start again, so neither list names a step any more.
export function forgetSteps() {
  write(OWN_KEY, []);
  write(REDO_KEY, []);
}
