// The uptime strip's bars, shared by the router graph on /log and each instance on /panels. CSS is in base.css.
import { el } from './core.js';
import { slots, slotText } from './uptime.js';

// data is what slots() takes. With pick the bars are buttons (the /log strip opens a slot); without, plain marks,
// since the panels page rebuilds its list every poll and a focused button would hold that rebuild off.
export function uptimeBars(data, { fmt, words, pick, sel } = {}) {
  return slots(data).map((s) => {
    const bar = el(pick ? 'button' : 'span', `sode-up__bar sode-up__bar--${s.colour}`);
    bar.title = slotText(s, fmt, words);
    if (!pick) return bar;
    bar.type = 'button';
    bar.setAttribute('aria-label', bar.title);
    bar.disabled = s.colour === 'future';
    const on = sel != null && s.from === sel;
    if (on) bar.classList.add('sode-up__bar--sel');
    bar.setAttribute('aria-pressed', String(on));
    bar.addEventListener('click', () => pick(s));
    return bar;
  });
}
