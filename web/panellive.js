// Keeps an open panel current: polls the panel's rev, shows the banner while an update waits and calls
// apply(panel) when panelupdate.js says so. Polled because a browser cannot send Bearer on an event stream.
import { el } from './core.js';
import { createUpdate, noticeRev, snooze, canSnooze, applyNow, updateStep, applied, bannerLine } from './panelupdate.js';

const CHECK_MS = 5000;

// get(path) reads JSON with the panel's key; state() is { staged, busy }; ui is { update, updateText, snooze, applyNow, bar }.
// Returns a function that stops the watch.
export function watchPanel({ id, rev, get, state, apply, ui }) {
  let u = createUpdate(rev);
  let checking = false;
  let barAt = 0; // the deadline the bar on screen counts to

  // The bar is the main page's result countdown (sode__banner-bar in base.css): one compositor animation per
  // deadline, joined part way when the first tick comes late. No bar while staged routes or a take hold the copy.
  const showBar = (s) => {
    const counting = s.step === 'banner' && !s.wait;
    ui.bar.hidden = !counting;
    if (!counting) {
      barAt = 0;
      ui.bar.replaceChildren();
      return;
    }
    if (barAt === s.deadline) return;
    barAt = s.deadline;
    const bar = el('div', 'pb-progress__bar pb-progress__bar--warn sode__banner-bar');
    bar.style.animationDuration = `${s.totalMs}ms`;
    bar.style.animationDelay = `-${s.totalMs - s.leftMs}ms`;
    ui.bar.replaceChildren(bar);
  };

  const show = (s) => {
    const text = s.step === 'banner' ? bannerLine(s) : '';
    ui.update.hidden = !text;
    ui.updateText.textContent = text;
    ui.snooze.hidden = !canSnooze(u);
    showBar(s);
  };

  let latest = null; // the changed copy fetched, applied when the rules allow

  function tick() {
    if (!latest) return;
    const s = updateStep(u, state(), Date.now());
    if (s.step === 'apply') {
      const fresh = latest;
      latest = null;
      u = applied(u, fresh.rev);
      show(s);
      apply(fresh);
    } else show(s);
  }

  async function check() {
    if (checking) return;
    checking = true;
    try {
      const p = await get(`api/v1/panels/${id}`);
      u = noticeRev(u, p.rev, Date.now());
      latest = u.since ? p : null;
    } catch {
      // A failed read leaves the panel as it is; the next check tries again.
    }
    checking = false;
    tick();
  }

  ui.snooze.addEventListener('click', () => {
    u = snooze(u, Date.now());
    tick();
  });
  ui.applyNow.addEventListener('click', () => {
    u = applyNow(u, Date.now());
    tick();
  });
  const timers = [setInterval(check, CHECK_MS), setInterval(tick, 1000)];
  return () => timers.forEach(clearInterval);
}
