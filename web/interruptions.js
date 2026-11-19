import { onMsg, wsSend } from './core.js';

const WAIT_MS = 5000;
let seq = 0;
let waiting = null; // { id, done }

// fields: what the save would send ({key: {value, version}}). Resolves the list, or null when Sode does not answer.
export function askInterruptions(fields) {
  waiting?.done(null);
  if (!Object.keys(fields).length) return Promise.resolve([]);
  const id = ++seq;
  return new Promise((resolve) => {
    const timer = setTimeout(() => waiting?.done(null), WAIT_MS);
    waiting = {
      id,
      done: (list) => {
        clearTimeout(timer);
        waiting = null;
        resolve(list);
      },
    };
    if (!wsSend({ t: 'settingsReview', id, fields })) waiting.done(null);
  });
}

export function initInterruptions() {
  onMsg('settingsReviewResult', (m) => {
    if (waiting && m.id === waiting.id) waiting.done(m.ok ? m.interruptions || [] : null);
  });
}
