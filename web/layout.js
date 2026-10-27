// Phone or desktop layout: automatic on narrow touch screens, with a
// per-device override remembered in this browser. Until it chooses, the
// global default (Settings) applies. index.html applies the same rule before
// first paint.

import { on, pref, setPref } from './core.js';

const root = document.documentElement;
const narrowTouch = matchMedia('(max-width: 700px) and (pointer: coarse)');
const listeners = [];

export const layoutPref = () => pref('layout');
export const setLayoutPref = (v) => setPref('layout', v);

export const layout = () => root.dataset.layout;
export const onLayout = (fn) => { listeners.push(fn); };

export function applyLayout() {
  const p = layoutPref();
  const next = p === 'auto' ? (narrowTouch.matches ? 'phone' : 'desktop') : p;
  if (root.dataset.layout === next) return;
  root.dataset.layout = next;
  for (const fn of listeners) fn(next);
}

narrowTouch.addEventListener('change', applyLayout);
on('prefs', applyLayout);
applyLayout();
