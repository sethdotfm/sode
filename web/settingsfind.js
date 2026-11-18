// Settings search. A pick of a hit scrolls to it, focuses a control and outlines it with a pulse. Esc clears the box
// first; a second Esc leaves Settings.

import { $, el } from './core.js';
import { showPinFields } from './settingspin.js';
import { buildIndex, find, hitWhere } from './settingsindex.js';
import { resultStep } from './keys.js';

// Runs one time, after the table of contents and #open-log (about.js) exist. Returns reindex, for a control added later.
export function initFind({ sections, toc }) {
  const findBox = $('settings-find');
  const foundList = $('settings-found');
  const secIds = sections.map((sec) => sec.dataset.section);
  // The band and router sub-heading text, read when the index is built (a router's title changes with its settings).
  const headText = (sel) => (sel ? document.querySelector(sel)?.textContent.trim() : undefined);
  const targets = new Map();

  function settingsOf(sec, secId) {
    const out = new Map();
    const add = (id, label, node) => {
      label = (label || '').trim();
      if (id && label && !out.has(id)) {
        out.set(id, { id, label });
        targets.set(`${secId}/${id}`, node);
      }
    };
    for (const lab of sec.querySelectorAll('label[for]')) {
      if (!lab.closest('.pb-btn-group')) add(lab.htmlFor, lab.textContent, lab.closest('.sode__field') || lab);
    }
    for (const lab of sec.querySelectorAll('.sode__field-label[id]')) add(lab.id.replace(/-label$/, ''), lab.textContent, lab.closest('.sode__field') || lab);
    for (const chk of sec.querySelectorAll('.pb-check')) add(chk.querySelector('input')?.id, chk.querySelector('.pb-check__label')?.textContent, chk);
    for (const f of sec.querySelectorAll('[data-find]')) add(f.dataset.find, f.dataset.label || f.textContent, f);
    return [...out.values()];
  }
  const OUTSIDE = [{ secId: 'history', id: 'open-log', label: 'View Logs', node: $('open-log') }];
  const makeIndex = () =>
    buildIndex(
      sections.map((sec, i) => {
        const settings = settingsOf(sec, secIds[i]);
        for (const x of OUTSIDE.filter((o) => o.secId === secIds[i])) {
          settings.push({ id: x.id, label: x.label });
          targets.set(`${x.secId}/${x.id}`, x.node);
        }
        return {
          id: secIds[i],
          title: sec.querySelector('legend').textContent,
          topic: headText(`.sode__topic[data-topic="${sec.dataset.topic}"]`),
          router: headText(sec.dataset.router && `.sode__router[data-router="${sec.dataset.router}"]`),
          settings,
        };
      }),
    );
  let settingsIndex = makeIndex();

  let hits = [];
  let cur = 0;
  let flashTimer = 0;
  let flashStart = null;
  let flashed = null;

  function unflash() {
    clearTimeout(flashTimer);
    if (flashStart) window.removeEventListener('scrollend', flashStart);
    flashStart = null;
    if (!flashed) return;
    flashed.node.classList.remove('sode__is-found');
    flashed = null;
  }
  // Starts after the scroll ends, so a far target pulses on screen; the timeout covers a scroll that never fires scrollend.
  function flash(node, smooth) {
    unflash();
    const start = () => {
      unflash();
      node.classList.add('sode__is-found');
      flashed = { node };
      flashTimer = setTimeout(unflash, 2500);
    };
    if (!smooth) return start();
    flashStart = start;
    window.addEventListener('scrollend', start, { once: true });
    flashTimer = setTimeout(start, 1000);
  }

  // A [data-find] row or button is hidden while it does not apply (Sign in, My API keys), so search skips it. So does
  // a section the Role or the phone leaves out (settingsroles.js).
  const offered = (h) => {
    if (sections.find((s) => s.dataset.section === h.sectionId)?.hidden) return false;
    const node = h.kind === 'setting' ? targets.get(`${h.sectionId}/${h.id}`) : null;
    return !(node?.matches('[data-find]') && node.closest('[hidden]'));
  };

  function showHits() {
    const q = findBox.value.trim();
    hits = q ? find(settingsIndex, q).filter(offered) : [];
    cur = 0;
    foundList.replaceChildren(
      ...hits.map((h, i) => {
        const li = el('li', null, el('span', null, h.label), el('span', 'sode__hint', (h.kind === 'section' ? 'Section in ' : 'in ') + hitWhere(h)));
        li.id = `settings-found-${i}`;
        li.setAttribute('role', 'option');
        li.addEventListener('mousedown', (e) => e.preventDefault());
        li.addEventListener('click', () => pickHit(h));
        return li;
      }),
    );
    foundList.hidden = !hits.length;
    $('settings-find-none').hidden = !q || hits.length > 0;
    findBox.setAttribute('aria-expanded', String(hits.length > 0));
    markHit();
  }
  function markHit() {
    [...foundList.children].forEach((li, i) => li.setAttribute('aria-selected', String(i === cur)));
    if (hits.length) findBox.setAttribute('aria-activedescendant', `settings-found-${cur}`);
    else findBox.removeAttribute('aria-activedescendant');
    foundList.children[cur]?.scrollIntoView({ block: 'nearest' });
  }
  function pickHit(h) {
    const i = secIds.indexOf(h.sectionId);
    const node = h.kind === 'section' ? sections[i] : targets.get(`${h.sectionId}/${h.id}`);
    if (!node) return;
    if (h.kind === 'setting' && /^cfg-pin(2|-clear)?$/.test(h.id)) showPinFields(true);
    if (toc.phone()) toc.showPage(i);
    findBox.value = '';
    showHits();
    toc.pick(i);
    const smooth = !matchMedia('(prefers-reduced-motion: reduce)').matches;
    node.scrollIntoView({ behavior: smooth ? 'smooth' : 'auto', block: h.kind === 'section' ? 'start' : 'center' });
    flash(node, smooth);
    if (h.kind === 'setting') document.getElementById(h.id)?.focus({ preventScroll: true });
    else findBox.blur();
  }

  // Typed text here is not a settings edit: the form must not see it (it would dismiss the
  // unsaved-changes warning).
  for (const ev of ['input', 'change'])
    findBox.addEventListener(ev, (e) => {
      e.stopPropagation();
      if (ev === 'input') showHits();
    });
  findBox.addEventListener('blur', () => {
    foundList.hidden = true;
    findBox.setAttribute('aria-expanded', 'false');
  });
  findBox.addEventListener('focus', () => {
    if (hits.length) {
      foundList.hidden = false;
      findBox.setAttribute('aria-expanded', 'true');
    }
  });
  findBox.addEventListener('keydown', (e) => {
    const step = resultStep(e, hits.length > 0);
    if (step) {
      // Consumed here, so the section stepper (settingstoc.js) sees defaultPrevented.
      e.preventDefault();
      cur = (cur + (step > 0 ? 1 : hits.length - 1)) % hits.length;
      markHit();
    } else if (e.ctrlKey || e.metaKey || e.altKey) {
      return;
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (hits[cur]) pickHit(hits[cur]);
    } else if (e.key === 'Escape' && findBox.value) {
      e.preventDefault();
      e.stopPropagation();
      findBox.value = '';
      showHits();
    }
  });

  return () => {
    settingsIndex = makeIndex();
  };
}
