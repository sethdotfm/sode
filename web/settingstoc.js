// Settings sidebar: a tree of topics and sections, pinned beside the column (wide), a strip (narrow), or the phone's
// list page. Routers > Routers has an entry for each router with two or more (setRouters). Only the topic in view is
// open; Ctrl+J/K steps sections. initToc runs one time, after every section is in the page.

import { el } from './core.js';
import { layout } from './layout.js';
import { listStep } from './keys.js';
import { topicTree, topicOf, routerEntries } from './settingstree.js';

const sectionTitle = (s) => s.dataset.toc || s.querySelector('legend').firstChild.textContent.trim();
// A section's topic band (index.html: data-topic).
const bandOf = (form, s) => form.querySelector(`.sode__topic[data-topic="${s.dataset.topic}"]`);

// pickRouter(uid): an entry of a router selects it in the router list.
export function initToc({ form, sections, toc, isOpen, pickRouter }) {
  const tree = topicTree(
    sections.map((s) => ({
      title: sectionTitle(s),
      topic: s.dataset.topic,
      topicTitle: bandOf(form, s)?.textContent.trim(),
    })),
  );
  // A clicked entry stays marked until the scroll ends or the user scrolls, (near the bottom a scroll can stop short).
  let picked = -1;
  // Set while Ctrl+J/K steps. Those scroll instantly and keep `picked`, so a held or spammed key walks one section per
  // press. It does not re-read the stale scroll position.
  let stepping = false;
  let marked = -1;
  // Sections the Role or the phone leaves out (settingsroles.js): no entry, no anchor, no step.
  let off = new Set();
  const shown = (i) => !off.has(i);

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
  const focusField = (node) => {
    reveal(node);
    node.focus();
  };
  const smooth = (instant) => (instant || matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth');
  // i: the section to mark; node: what to scroll to (a band or the section).
  function go(i, instant, node = sections[i]) {
    if (phone()) return showPage(i);
    picked = i;
    stepping = instant;
    mark(i);
    node.scrollIntoView({ behavior: smooth(instant), block: 'start' });
  }

  const entry = (cls, text, onClick) => {
    const b = el('button', 'pb-tab ' + cls, text);
    b.type = 'button';
    b.addEventListener('click', onClick);
    return b;
  };
  // What starts each section on the page: a topic's first section starts at its band.
  const anchors = sections.map((s, i) => (tree.some((x) => x.first === i) ? bandOf(form, s) : s));
  const secBtns = sections.map((s, i) => entry('sode__toc-sec', sectionTitle(s), () => go(i, false)));
  const branches = tree.map((t) => {
    const head = entry('sode__toc-topic', t.title, () => go(t.first, false, bandOf(form, sections[t.first])));
    head.dataset.topic = t.id;
    return { head, kids: t.sections.map((i) => secBtns[i]) };
  });
  toc.replaceChildren(...branches.flatMap((b) => [b.head, ...b.kids]));
  // The entries of the routers, after the Routers section's entry. The phone has no Routers topic.
  const routersAt = sections.findIndex((s) => s.dataset.section === 'routers');
  let routerBtns = [];

  function mark(cur) {
    marked = cur;
    const at = topicOf(tree, cur);
    branches.forEach((b, k) => {
      const open = k === at;
      const t = tree[k];
      b.head.hidden = !t.sections.some(shown);
      b.head.setAttribute('aria-expanded', String(open));
      b.head.classList.toggle('sode__toc-open', open);
      for (const kid of b.kids) kid.hidden = !shown(secBtns.indexOf(kid)) || (!open && !phone());
    });
    const routersOpen = routersAt >= 0 && shown(routersAt) && !phone() && topicOf(tree, routersAt) === at;
    for (const b of routerBtns) b.hidden = !routersOpen;
    secBtns.forEach((b, i) => {
      b.classList.toggle('active', i === cur);
      if (i === cur) b.setAttribute('aria-current', 'location');
      else b.removeAttribute('aria-current');
    });
  }
  const firstShown = () =>
    Math.max(
      sections.findIndex((_, i) => shown(i)),
      0,
    );
  mark(0);
  let tocTick = false;
  function markSection() {
    tocTick = false;
    if (!isOpen() || picked >= 0) return;
    // Last section whose top reached the window top (its scroll margin down). At the very bottom, the last section: it can be too short to reach it.
    const atEnd = window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2;
    const line = (s) => parseFloat(getComputedStyle(s).scrollMarginTop) + 8;
    const last = sections.findLastIndex((_, i) => shown(i));
    const at = anchors.findLastIndex((s, i) => shown(i) && s.getBoundingClientRect().top <= line(s));
    mark(atEnd ? last : at >= 0 ? at : firstShown());
  }
  window.addEventListener(
    'scroll',
    () => {
      if (tocTick) return;
      tocTick = true;
      requestAnimationFrame(markSection);
    },
    { passive: true },
  );
  // Ctrl+J/K (Cmd on a Mac) step sections, also from the search box; not while a dialog is over Settings.
  for (const ev of ['scrollend', 'wheel', 'touchstart', 'pointerdown', 'keydown']) {
    window.addEventListener(
      ev,
      (e) => {
        if (picked < 0 || (stepping && (ev === 'scrollend' || (ev === 'keydown' && listStep(e))))) return;
        picked = -1;
        stepping = false;
        if (ev !== 'scrollend') markSection();
      },
      { passive: true },
    );
  }
  document.addEventListener('keydown', (e) => {
    if (!isOpen() || e.defaultPrevented) return;
    const step = listStep(e);
    if (!step || e.target.closest?.('dialog, [role="dialog"], .pb-modal')) return;
    e.preventDefault();
    let to = picked >= 0 ? picked : marked;
    do to += step;
    while (to >= 0 && to < sections.length && !shown(to));
    if (to >= 0 && to < sections.length) go(to, true);
  });

  return {
    phone,
    showPage,
    reveal,
    focusField,
    markSection,
    pick(i) {
      picked = i;
      mark(i);
    },
    back() {
      if (!phone() || form.dataset.page == null) return false;
      showPage(-1);
      return true;
    },
    // hidden: section indexes to leave out.
    setOff(hidden) {
      off = new Set(hidden);
      if (phone() && form.dataset.page != null && off.has(Number(form.dataset.page))) showPage(-1);
      mark(shown(marked) ? marked : firstShown());
    },
    goSection(slug) {
      const i = sections.findIndex((s) => s.dataset.section === slug);
      if (i >= 0) go(i, false);
    },
    // setRouters: one entry for each router under Routers > Routers (routerEntries). routers: [{uid, name}].
    setRouters(routers, selected) {
      if (routersAt < 0) return;
      const want = routerEntries(routers, selected);
      const same = want.length === routerBtns.length && want.every((r, k) => routerBtns[k].dataset.router === r.uid && routerBtns[k].textContent === r.title);
      if (!same) {
        for (const b of routerBtns) b.remove();
        routerBtns = want.map((r) => {
          const b = entry('sode__toc-router', r.title, () => {
            pickRouter?.(r.uid);
            go(routersAt, false);
          });
          b.dataset.router = r.uid;
          return b;
        });
        secBtns[routersAt].after(...routerBtns);
      }
      routerBtns.forEach((b, k) => {
        b.classList.toggle('sode__toc-open', want[k].current);
        if (want[k].current) b.setAttribute('aria-current', 'true');
        else b.removeAttribute('aria-current');
      });
      mark(marked);
    },
  };
}
