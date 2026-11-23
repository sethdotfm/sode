// The phone's Confirm sheet: a bottom sheet with a slider to send.
import { $ } from './core.js';
import { planVerb, showResult, planBlocked, planSummary, sendTakes, VERBS } from './plans.js';
import { lockout } from './widgets.js';
import { tieTake } from './tieflow.js';

// reset() clears the composer after a send whose verb resets the boxes.
export function mobileConfirm(reset) {
  let pending = null;

  function openSheet(plan) {
    const { block, prot, tie } = planBlocked(plan);
    if (block) return showResult(block.kind, block.title, block.text);
    if (tie) return tieTake(plan, prot, { onSent: (p) => VERBS[p.verb].resets && reset(), confirm: showSheet });
    showSheet(plan, prot);
  }

  // showSheet opens the sheet on plan. A take with tie routes opens it after its pickers (tieflow.js).
  function showSheet(plan, prot) {
    pending?.lock.cancel(); // one replaced: its lockout must not unlock this one
    $('m-sheet-title').textContent = `Confirm ${planVerb(plan)}`;
    $('m-sheet-body').replaceChildren(...planSummary(plan, { prot }));
    $('m-slide-label').textContent = `Slide to ${planVerb(plan)}`;
    resetSlide(true);
    $('m-sheet').hidden = false;
    pending = { ...plan, lock: lockout(() => $('m-slide').classList.remove('sode__slide--locked')) };
  }

  function closeSheet() {
    pending?.lock.cancel();
    pending = null;
    $('m-sheet').hidden = true;
  }

  function confirmSheet() {
    const p = pending;
    closeSheet();
    if (sendTakes(p) && VERBS[p.verb].resets) reset();
  }

  const handle = $('m-slide-handle');
  const track = $('m-slide');
  let drag = null;

  const maxX = () => track.clientWidth - handle.offsetWidth;

  function setSlide(x) {
    const pct = Math.round((x / maxX()) * 100);
    handle.style.transform = `translateX(${x}px)`;
    track.style.setProperty('--slide', `${x + handle.offsetWidth}px`);
    handle.setAttribute('aria-valuenow', String(pct));
  }

  function resetSlide(locked) {
    track.classList.toggle('sode__slide--locked', locked);
    track.classList.remove('sode__slide--drag');
    setSlide(0);
  }

  handle.addEventListener('pointerdown', (e) => {
    if (!pending || !pending.lock.ready()) return;
    drag = { x0: e.clientX };
    handle.setPointerCapture(e.pointerId);
    track.classList.add('sode__slide--drag');
  });
  handle.addEventListener('pointermove', (e) => {
    if (!drag) return;
    setSlide(Math.max(0, Math.min(maxX(), e.clientX - drag.x0)));
  });
  const release = (e) => {
    if (!drag) return;
    const x = Math.max(0, Math.min(maxX(), e.clientX - drag.x0));
    drag = null;
    track.classList.remove('sode__slide--drag');
    if (x >= maxX() * 0.9) {
      setSlide(maxX());
      confirmSheet();
    } else {
      setSlide(0);
    }
  };
  handle.addEventListener('pointerup', release);
  handle.addEventListener('pointercancel', release);
  handle.addEventListener('keydown', (e) => {
    if (!pending || !pending.lock.ready()) return;
    if (e.key === 'ArrowRight' || e.key === 'End') {
      e.preventDefault();
      setSlide(maxX());
      confirmSheet();
    }
  });
  $('m-sheet-cancel').addEventListener('click', closeSheet);
  $('m-sheet').addEventListener('click', (e) => {
    if (e.target === $('m-sheet')) closeSheet();
  });

  return { open: openSheet, close: closeSheet };
}
