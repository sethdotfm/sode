// The uptime strip's bars, shared by the router graph on /log and each instance on /panels. CSS is in base.css.
import { el } from './core.js';
import { slots, slotText } from './uptime.js';

// data is what slots() reads. With pick, the bars are buttons (the /log strip opens a slot). Without it they are plain
// marks, since the panels page rebuilds its list every poll and a focused button would hold that rebuild off.
// A strip of buttons is one Tab stop: the picked bar, else the newest bar that can be pressed (roveBars moves it).
// A tip opens over the strip, so it does not cover the line under it, and lines up with the strip's nearer end.
export function uptimeBars(data, { fmt, words, pick, sel } = {}) {
  const ss = slots(data);
  const bars = ss.map((s, i) => {
    const bar = el(pick ? 'button' : 'span', `sode-up__bar sode-up__bar--${s.color} pb-tip--top`);
    if (i < ss.length / 3) bar.classList.add('pb-tip--start');
    else if (i >= ss.length - ss.length / 3) bar.classList.add('pb-tip--end');
    bar.dataset.pbTip = slotText(s, fmt, words);
    if (!pick) return bar;
    bar.type = 'button';
    bar.tabIndex = -1;
    bar.setAttribute('aria-label', bar.dataset.pbTip);
    bar.disabled = s.color === 'future';
    const on = sel != null && s.from === sel;
    if (on) bar.classList.add('sode-up__bar--sel');
    bar.setAttribute('aria-pressed', String(on));
    bar.addEventListener('click', () => pick(s));
    return bar;
  });
  if (pick) {
    const stop = bars.find((b) => b.classList.contains('sode-up__bar--sel')) || bars.findLast((b) => !b.disabled);
    if (stop) stop.tabIndex = 0;
  }
  return bars;
}

// Arrow keys move the focus and the Tab stop along a strip of bar buttons; Home and End go to its ends. Call once for
// each strip: the listener stays when the bars are replaced.
export function roveBars(strip) {
  strip.addEventListener('keydown', (e) => {
    const bars = [...strip.querySelectorAll('button.sode-up__bar:not(:disabled)')];
    const i = bars.indexOf(e.target);
    if (i < 0 || e.altKey || e.ctrlKey || e.metaKey) return;
    const j = { ArrowLeft: i - 1, ArrowRight: i + 1, Home: 0, End: bars.length - 1 }[e.key];
    if (j == null) return;
    e.preventDefault();
    const to = bars[Math.max(0, Math.min(j, bars.length - 1))];
    for (const b of bars) b.tabIndex = b === to ? 0 : -1;
    to.focus();
  });
}
