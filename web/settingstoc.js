// The Settings table of contents: one button per section, scrolling to it,
// with the section in view marked. Wide screens pin it beside the column;
// narrow ones show it as a strip across the top; the phone shows one section
// at a time (the TOC is the list page). Also Ctrl+J/K between sections.
// initToc is called once, after every section has been added to the form.

import { el } from './core.js';
import { layout } from './layout.js';
import { mac, chord } from './keys.js';

export function initToc({ form, sections, toc, isOpen }) {
  // A clicked section stays marked while the page scrolls to it, which may
  // stop short of it near the bottom, until the scroll ends or the user
  // scrolls themselves.
  let picked = -1;

  // The phone shows one section at a time: the page is the list of sections
  // (with the search box), a tap opens one, Back returns to the list. Only
  // CSS keyed on data-page does the hiding (base.css), so it ignores desktop.
  const phone = () => layout() === 'phone';
  function showPage(i) {
    sections.forEach((s, k) => s.classList.toggle('sode__page-on', k === i));
    if (i < 0) delete form.dataset.page;
    else form.dataset.page = String(i);
    if (isOpen()) window.scrollTo(0, 0);
  }
  // Opens the section holding node (phone), so a found or failing field is on screen.
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
    // The last section whose top has reached the top of the window (where a
    // click scrolls it to, its scroll margin down); at the very bottom the
    // last one, which may be too short to ever reach it.
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

  // Ctrl+J/K (⌘J/K on a Mac) step through the sections, like the match lists
  // (from the search box too; not while a dialog is over Settings).
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
    // Phone: Back leaves a section for the list; false when already on the list.
    back() {
      if (!phone() || form.dataset.page == null) return false;
      showPage(-1);
      return true;
    },
  };
}
