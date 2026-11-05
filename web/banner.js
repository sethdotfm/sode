// The result banner: what a result puts up, how each layout draws it, and how
// it goes away. A success closes itself after cfg.resultDismissSeconds (with a
// bar running down); a failure stays until closed by hand. plans.js
// re-exports showResult/resultNode.
//
// Each layout draws its own copy of a result (one is hidden). A copy is tagged
// with the result's id (data-rid); the result has one timer (setResult), and
// dismissing collapses every copy still on the page, so nothing needs a
// registry and a replaced copy is simply dropped.

import { el, S, icon, emit } from './core.js';

let seq = 0;

// Puts r ({ kind, title, text, track }) up as the current result and tells
// the layouts. text: a string, or a function building nodes.
export function setResult(r) {
  const res = (S.result = { ...r, id: ++seq });
  const ms = dismissMs(res.kind, S.cfg?.resultDismissSeconds || 0);
  if (ms) setTimeout(() => dismiss(res), ms); // one timer per result, whichever layouts draw it
  emit('result', res);
}

export const showResult = (kind, title, text, track = null) => setResult({ kind, title, text, track });

// How long a result stays up by itself: a success for `secs` (0: until
// closed), anything else until closed.
export const dismissMs = (kind, secs) => (kind === 'ok' && secs > 0 ? secs * 1000 : 0);

function dismiss(r) {
  if (S.result === r) S.result = null;
  for (const wrap of document.querySelectorAll(`.sode__banner[data-rid="${r.id}"]`)) {
    // Collapse (see .sode__banner--out) so what is below slides up, then drop it.
    const done = () => wrap.remove();
    wrap.classList.add('sode__banner--out');
    wrap.addEventListener('transitionend', (e) => { if (e.target === wrap) done(); }, { once: true });
    setTimeout(done, 400); // a hidden copy never transitions
  }
}

export function resultNode(r) {
  const wrap = el('div', 'sode__banner');
  wrap.dataset.rid = String(r.id);
  const close = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode__banner-x', icon('x'));
  close.type = 'button';
  close.setAttribute('aria-label', 'Dismiss');
  close.dataset.pbTip = 'Dismiss';
  close.addEventListener('click', () => dismiss(r));
  const inner = el('div', 'sode__banner-in', el('div', `pb-alert pb-alert--${r.kind}`,
    el('div', 'sode__banner-body', el('div', 'pb-alert__title', r.title), typeof r.text === 'function' ? r.text() : r.text), close));
  wrap.append(inner);
  const ms = dismissMs(r.kind, S.cfg?.resultDismissSeconds || 0);
  if (ms) {
    const bar = el('div', 'pb-progress__bar pb-progress__bar--ok sode__banner-bar');
    bar.style.animationDuration = `${ms}ms`;
    inner.append(el('div', 'pb-progress sode__banner-time', bar));
  }
  return wrap;
}
