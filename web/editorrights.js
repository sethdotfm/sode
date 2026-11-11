// The signed-in user's own Rights for the panel editor (`myRights`, never in the hello). They can arrive before the
// editor has loaded, so they are caught at import time; Find and the Fixed destination picker read them here.
import { onMsg, on } from './core.js';

let rights = null;
const watchers = [];
const tell = () => watchers.forEach((fn) => fn(rights));
onMsg('myRights', (m) => {
  rights = m.rights || null;
  tell();
});
on('wsclose', () => {
  rights = null;
  tell();
});

export const myRights = () => rights;
export const onMyRights = (fn) => watchers.push(fn);
