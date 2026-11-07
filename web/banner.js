// Result banner. Each layout draws its own copy (data-rid); one timer per result, and dismissing collapses every copy.
// plans.js re-exports showResult/resultNode.

import { el, S, icon, emit } from './core.js';

let seq = 0;

// r: { kind, title, text, track }; text is a string or a function building nodes.
export function setResult(r) {
  const res = (S.result = { ...r, id: ++seq });
  const ms = dismissMs(res.kind, S.cfg?.resultDismissSeconds || 0);
  if (ms) setTimeout(() => dismiss(res), ms);
  emit('result', res);
}

export const showResult = (kind, title, text, track = null) => setResult({ kind, title, text, track });

export const dismissMs = (kind, secs) => (kind === 'ok' && secs > 0 ? secs * 1000 : 0);

function dismiss(r) {
  if (S.result === r) S.result = null;
  for (const wrap of document.querySelectorAll(`.sode__banner[data-rid="${r.id}"]`)) {
    // .sode__banner--out collapses it so what is below slides up.
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
