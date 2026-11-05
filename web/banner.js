// The result banner: what showResult puts up, how each layout draws it, and
// how it goes away (a success closes itself after cfg.resultDismissSeconds,
// a failure stays until closed). plans.js re-exports showResult/resultNode.

import { el, S, icon, emit } from './core.js';

// text: a string, or a function building nodes (each layout renders its own).
export function showResult(kind, title, text, track = null) {
  S.result = { kind, title, text, track };
  clearTimeout(dismissTimer);
  const secs = S.cfg?.resultDismissSeconds || 0;
  if (kind === 'ok' && secs > 0) { const r = S.result; dismissTimer = setTimeout(() => dismissResult(r), secs * 1000); }
  emit('result', S.result);
}

// A success closes itself after S.cfg.resultDismissSeconds (0: stays), with a
// bar running down; a failure stays until closed by hand. The timer belongs to
// the result, not to a node: each layout draws its own copy (one is hidden),
// and every copy still on the page closes together.
let dismissTimer = 0;
const shown = new Set();
function dismissResult(r) {
  clearTimeout(dismissTimer);
  if (S.result === r) S.result = null;
  for (const wrap of shown) {
    shown.delete(wrap);
    if (!wrap.isConnected) continue;
    // Collapse (see .sode__banner--out) so what is below slides up, then drop it.
    const done = () => wrap.remove();
    wrap.classList.add('sode__banner--out');
    wrap.addEventListener('transitionend', (e) => { if (e.target === wrap) done(); }, { once: true });
    setTimeout(done, 400); // a hidden copy never transitions
  }
}

export function resultNode(r) {
  const wrap = el('div', 'sode__banner');
  const close = el('button', 'pb-btn pb-btn--ghost pb-btn--icon pb-btn--sm sode__banner-x', icon('x'));
  close.type = 'button';
  close.setAttribute('aria-label', 'Dismiss');
  close.dataset.pbTip = 'Dismiss';
  close.addEventListener('click', () => dismissResult(r));
  const inner = el('div', 'sode__banner-in', el('div', `pb-alert pb-alert--${r.kind}`,
    el('div', 'sode__banner-body', el('div', 'pb-alert__title', r.title), typeof r.text === 'function' ? r.text() : r.text), close));
  wrap.append(inner);
  if (r.kind === 'ok' && (S.cfg?.resultDismissSeconds || 0) > 0) {
    const bar = el('div', 'pb-progress__bar pb-progress__bar--ok sode__banner-bar');
    bar.style.animationDuration = `${S.cfg.resultDismissSeconds}s`;
    inner.append(el('div', 'pb-progress sode__banner-time', bar));
  }
  shown.add(wrap);
  return wrap;
}
