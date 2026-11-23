// The default router of each patch box ('src', 'dst'): a bare number in the box means a number on it. Kept for this
// browser by router UID, since an administrator can change a prefix. A new browser, or a router that is gone, gives
// the first router. With one router the button hides.
import { S, on, routerData } from './core.js';
import { readJSON, writeJSON } from './store.js';

const KEY = 'sode-box-routers';

// boxRouter is the default router of box k, as a routerData UID: blank for the first router.
export function boxRouter(k) {
  const uid = readJSON(KEY, {})?.[k];
  return uid && S.routers.slice(1).some((r) => r.uid === uid) ? uid : '';
}

// nextBoxRouter makes the router after the current default of box k its default, the first router after the last.
export function nextBoxRouter(k) {
  const now = routerData(boxRouter(k)).uid;
  const i = S.routers.findIndex((r) => r.uid === now);
  const next = S.routers[(i + 1) % S.routers.length];
  const all = readJSON(KEY, {}) || {};
  writeJSON(KEY, { ...all, [k]: next && next !== S.routers[0] ? next.uid : undefined });
}

// paintBoxRouter shows box k's default router on button b: "Router B", with the router title in the tip.
export function paintBoxRouter(k, b) {
  const rt = routerData(boxRouter(k));
  b.hidden = S.routers.length < 2;
  b.textContent = `Router ${rt.prefix}`;
  const what = k === 'src' ? 'source' : 'destination';
  b.dataset.pbTip = `A ${what} number with no prefix is on router ${rt.prefix}${rt.title ? ` (${rt.title})` : ''}. Select to change the router.`;
}

// wireBoxRouter keeps button b of box k in step with the routers; a press moves to the next router, then calls changed.
export function wireBoxRouter(k, b, changed) {
  const paint = () => paintBoxRouter(k, b);
  b.addEventListener('mousedown', (e) => e.preventDefault());
  b.addEventListener('click', () => {
    nextBoxRouter(k);
    paint();
    changed();
  });
  on('hello', paint);
  paint();
}
