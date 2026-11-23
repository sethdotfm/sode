// Result banner. Each layout draws its own copy (data-rid); one timer per result, and dismissing collapses every copy.

import { el, S, icon, emit } from './core.js';

let seq = 0;

// r: { kind, title, text, track }; text is a string or a function building nodes. Kind 'ok' is green, so it is for a Take
// that succeeded only (docs/color-roles.md). 'done' is any other success: neutral, and it goes away like 'ok'. 'info' is neutral.
export function setResult(r) {
  const res = (S.result = { ...r, id: ++seq });
  const ms = dismissMs(res.kind, S.cfg?.resultDismissSeconds || 0);
  if (ms) setTimeout(() => dismissResult(res), ms);
  emit('result', res);
}

export const showResult = (kind, title, text, track = null) => setResult({ kind, title, text, track });

export const dismissMs = (kind, secs) => ((kind === 'ok' || kind === 'done') && secs > 0 ? secs * 1000 : 0);

const ALERT = { ok: 'pb-alert pb-alert--ok', warn: 'pb-alert pb-alert--warn', error: 'pb-alert pb-alert--error' };

// Also the Grid panel page's banner (gridpanelheadview.js), for a result whose cause has cleared.
export function dismissResult(r) {
  if (S.result === r) S.result = null;
  for (const wrap of document.querySelectorAll(`.sode__banner[data-rid="${r.id}"]`)) {
    const done = () => wrap.remove();
    wrap.classList.add('sode__banner--out');
    wrap.addEventListener(
      'transitionend',
      (e) => {
        if (e.target === wrap) done();
      },
      { once: true },
    );
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
  close.addEventListener('click', () => dismissResult(r));
  const inner = el(
    'div',
    'sode__banner-in',
    el(
      'div',
      ALERT[r.kind] || 'pb-alert',
      el('div', 'sode__banner-body', el('div', 'pb-alert__title', r.title), typeof r.text === 'function' ? r.text() : r.text),
      close,
    ),
  );
  wrap.append(inner);
  const ms = dismissMs(r.kind, S.cfg?.resultDismissSeconds || 0);
  if (ms) {
    const bar = el('div', 'pb-progress__bar sode__banner-bar');
    bar.style.animationDuration = `${ms}ms`;
    inner.append(el('div', 'pb-progress sode__banner-time', bar));
  }
  return wrap;
}
