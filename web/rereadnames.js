// Re-read router names, from the Palette and from Settings. The banner says when the router answered and what changed,
// or that it did not answer. The server ends a read when the names stop or after its 10 s timeout. Then it sends
// names for a change, else namesChecked. With namesChecked, namesAt stays the same when the router did not answer.
import { S, on, wsSend } from './core.js';
import { showResult } from './banner.js';
import { namesChange, changeText } from './rereadrules.js';

// Longer than the server's timeout, so the server's answer normally comes first.
const WAIT_MS = 15_000;

let pending = null;

function finish(kind, title, text) {
  if (!pending) return;
  clearTimeout(pending.timer);
  pending = null;
  showResult(kind, title, text);
}

const noAnswer = () => finish('warn', 'The router did not answer', 'The names did not change. Check the router connection, then try again.');

export function rereadNames() {
  if (!wsSend({ t: 'rereadNames' })) return false;
  if (pending) clearTimeout(pending.timer);
  pending = { at: S.namesAt, before: { srcs: S.srcs, dsts: S.dsts }, timer: setTimeout(noAnswer, WAIT_MS) };
  showResult('info', 'Re-reading router names…', '');
  return true;
}

on('names', () => {
  if (!pending) return;
  const total = { srcs: S.srcs.length, dsts: S.dsts.length };
  finish('done', 'Router names updated', changeText(namesChange(pending.before, { srcs: S.srcs, dsts: S.dsts }), total));
});

on('namesChecked', () => {
  if (!pending) return;
  if (S.namesAt === pending.at) return noAnswer();
  finish('done', 'Router names re-read', changeText({ srcs: 0, dsts: 0 }, { srcs: S.srcs.length, dsts: S.dsts.length }));
});
