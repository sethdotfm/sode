// Ctrl+Z and Ctrl+Shift+Z on Home (docs/agents/history.md, Step back): each Ctrl+Z undoes one take further back, and
// Ctrl+Shift+Z redoes what Ctrl+Z undid. Only this browser's takes count: their History ids are in `sessionStorage`, so
// a reload keeps them and a new tab starts empty. backStep and forwardStep are pure, so Node tests them.
import { undoIndex, markFor } from './undone.js';
import { readTab, writeTab } from './store.js';

const OWN_KEY = 'sode-own-takes'; // History ids of the entries this tab sent, oldest first
const REDO_KEY = 'sode-redo-steps'; // the takes Ctrl+Z undid, each a list of History ids, newest last
const OWN_MAX = 1000;
const REDO_MAX = 100;

// The same rule as the panel Undo cell (panelundo.go). An undo or a Protect set is never a step. A take that a later undo
// put back on every level is passed over (goneOf). A Free Protect counts, since its Undo sets the Protect again.
const stepKind = (x) => !x.ext && x.undo == null && x.protect !== 'set';

// redo: lists of ids. An id there whose undo has no loaded entry yet is still on its way back, so it is passed over too.
function goneOf(hist, redo) {
  const index = undoIndex(hist);
  const tried = new Set(hist.filter((x) => x.undo != null).map((x) => x.undo));
  const waiting = new Set(redo.flat().filter((id) => !tried.has(id)));
  return (x) => waiting.has(x.id) || markFor([x.id], x.levels, index)?.state === 'undone';
}

// The take that Ctrl+Z undoes: the newest one of own (ids) that is not gone. Its Group's other entries that are not gone
// come with it. hist is newest first. Answers { e, entries } (entries newest first) or null.
export function backStep(hist, own, redo = []) {
  const mine = new Set(own);
  const gone = goneOf(hist, redo);
  const can = (x) => mine.has(x.id) && stepKind(x) && !gone(x);
  const e = hist.find(can);
  if (!e) return null;
  const entries = e.group && e.groupN > 1 ? hist.filter((x) => x.group === e.group && can(x)) : [e];
  return { e, entries };
}

// The take that Ctrl+Shift+Z redoes: the newest list in redo that still has a loaded entry and whose undo did not fail on
// every level. keep is redo without the lists passed over, so they do not come back.
export function forwardStep(hist, redo) {
  const byId = new Map(hist.map((x) => [x.id, x]));
  const index = undoIndex(hist);
  const tried = new Set(hist.filter((x) => x.undo != null).map((x) => x.undo));
  const keep = [...redo];
  while (keep.length) {
    const ids = keep[keep.length - 1];
    const entries = ids.map((id) => byId.get(id)).filter(Boolean);
    const failed = entries.length && entries.every((x) => tried.has(x.id) && !markFor([x.id], x.levels, index));
    if (entries.length && !failed) return { entries, ids, keep };
    keep.pop();
  }
  return { entries: null, ids: null, keep };
}

// A send from this tab. A plan from Ctrl+Z has step 'back' and the ids it undoes; one from Ctrl+Shift+Z has step
// 'forward'. Any other take clears the redo list, as in an editor; an undo, a redo, a retry or a Protect does not.
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

export const ownTakes = () => read(OWN_KEY);
export const redoSteps = () => read(REDO_KEY);
export const keepRedo = (redo) => write(REDO_KEY, redo);

// The History ids of entries that this tab's sends wrote (takeResult, the Protect results).
export function noteOwn(ids) {
  const add = ids.filter((id) => Number.isInteger(id));
  if (add.length) write(OWN_KEY, [...ownTakes(), ...add].slice(-OWN_MAX));
}

export const noteSent = (plan) => keepRedo(nextRedo(redoSteps(), plan));
