// Settings table of contents: pinned beside the column (wide), a strip (narrow), or the phone's list page; Ctrl+J/K steps sections.
// initToc runs once, after every section is added.

import { el } from './core.js';
import { layout } from './layout.js';
import { mac, chord } from './keys.js';

export function initToc({ form, sections, toc, isOpen }) {
  // A clicked section stays marked until the scroll ends or the user scrolls, since near the bottom it may stop short.
  let picked = -1;

  // Phone: one section at a time; the page is the list, Back returns. Only CSS keyed on data-page hides (base.css).
  const phone = () => layout() === 'phone';
  function showPage(i) {
    sections.forEach((s, k) => s.classList.toggle('sode__page-on', k === i));
    if (i < 0) delete form.dataset.page;
    else form.dataset.page = String(i);
    if (isOpen()) window.scrollTo(0, 0);
  }
  function reveal(node) {
    if (!phone()) return;
    const i = sections.findIndex((s) => s.contains(node));
    if (i >= 0 && String(i) !== form.dataset.page) showPage(i);
  }
  const focusField = (node) => { reveal(node); node.focus(); };
  toc.replaceChildren(...sections.map((sec, i) => {
    const b = el('button', 'pb-tab', sec.querySelector('legend').textContent);
    b.type = 'button';
    b.addEventListener('click', () => {
      if (phone()) return showPage(i);
      picked = i;
      mark(i);
      sec.scrollIntoView({ behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'start' });
    });
    return b;
  }));
  function mark(cur) {
    [...toc.children].forEach((b, i) => {
      b.classList.toggle('active', i === cur);
      if (i === cur) b.setAttribute('aria-current', 'location');
      else b.removeAttribute('aria-current');
    });
  }
  let tocTick = false;
  function markSection() {
    tocTick = false;
    if (!isOpen() || picked >= 0) return;
    // Last section whose top reached the window top (its scroll margin down); at the very bottom the last, which may be too short to reach it.
    const atEnd = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2;
    const line = (s) => parseFloat(getComputedStyle(s).scrollMarginTop) + 8;
    mark(Math.max(atEnd ? sections.length - 1 : sections.findLastIndex((s) => s.getBoundingClientRect().top <= line(s)), 0));
  }
  window.addEventListener('scroll', () => {
    if (tocTick) return;
    tocTick = true;
    requestAnimationFrame(markSection);
  }, { passive: true });
  for (const ev of ['scrollend', 'wheel', 'touchstart', 'keydown']) {
    window.addEventListener(ev, () => {
      if (picked < 0) return;
      picked = -1;
      if (ev !== 'scrollend') markSection();
    }, { passive: true });
  }

  // Ctrl+J/K (Cmd on a Mac) step sections, also from the search box; not while a dialog is over Settings.
  const STEP = { 'ctrl+j': 1, 'ctrl+k': -1, ...(mac ? { 'meta+j': 1, 'meta+k': -1 } : {}) };
  document.addEventListener('keydown', (e) => {
    if (!isOpen() || e.defaultPrevented || form.hasAttribute('data-locked')) return;
    const step = STEP[chord(e)];
    if (!step || e.target.closest?.('dialog, [role="dialog"], .pb-modal')) return;
    e.preventDefault();
    const at = [...toc.children].findIndex((b) => b.classList.contains('active'));
    toc.children[Math.min(sections.length - 1, Math.max(0, at + step))].click();
  });

  return {
    phone, showPage, reveal, focusField, markSection,
    pick(i) { picked = i; mark(i); },
    back() {
      if (!phone() || form.dataset.page == null) return false;
      showPage(-1);
      return true;
    },
  };
}
