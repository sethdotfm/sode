// Phone or desktop layout: automatic on narrow touch screens, with a
// per-device override remembered in this browser. index.html applies the
// same rule before first paint.

const KEY = 'sode-layout';
const root = document.documentElement;
const narrowTouch = matchMedia('(max-width: 700px) and (pointer: coarse)');
const listeners = [];

export function layoutPref() {
  try { return localStorage.getItem(KEY) || 'auto'; } catch (e) { return 'auto'; }
}

export function setLayoutPref(v) {
  try { localStorage.setItem(KEY, v); } catch (e) {}
  applyLayout();
}

export const layout = () => root.dataset.layout;
export const onLayout = (fn) => { listeners.push(fn); };

export function applyLayout() {
  const pref = layoutPref();
  const next = pref === 'auto' ? (narrowTouch.matches ? 'phone' : 'desktop') : pref;
  if (root.dataset.layout === next) return;
  root.dataset.layout = next;
  for (const fn of listeners) fn(next);
}

narrowTouch.addEventListener('change', applyLayout);
applyLayout();
