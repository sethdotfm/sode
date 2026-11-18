// Auto on narrow touch screens, per-browser override, else the global default. index.html applies the same rule before first paint.

import { on } from './core.js';
import { pref } from './device.js';

const root = document.documentElement;
const narrowTouch = matchMedia('(max-width: 700px) and (pointer: coarse)');
const listeners = [];

const layoutPref = () => pref('layout');

export const layout = () => root.dataset.layout;
// Returns a function that removes fn.
export const onLayout = (fn) => {
  listeners.push(fn);
  return () => {
    const i = listeners.indexOf(fn);
    if (i >= 0) listeners.splice(i, 1);
  };
};

function applyLayout() {
  const p = layoutPref();
  const next = p === 'auto' ? (narrowTouch.matches ? 'phone' : 'desktop') : p;
  if (root.dataset.layout === next) return;
  root.dataset.layout = next;
  if (next === 'phone') root.dataset.pbLayout = 'phone';
  else delete root.dataset.pbLayout;
  for (const fn of listeners) fn(next);
}

narrowTouch.addEventListener('change', applyLayout);
on('prefs', applyLayout);
applyLayout();
