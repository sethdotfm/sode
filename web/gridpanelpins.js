// The little scroll bars of the pinned bands that hold fewer pins than their edge has. Each bar
// drives a scroller of its own beside the grid's, one pin per item size. Its offset in pins, with the part of a pin, is
// the band's start, which the view draws (gridpanelview.js). The grid's scroller does not move, so a band scroll leaves
// the rest where it is.
import { el } from './core.js';
import { bigScroll } from './bigscroll.js';

// A wheel notch is about 100 px and a trackpad step a few px; Chrome's own wheel glide on the grid is about this long.
const STEP_PX = 40;
const GLIDE_MS = 140;

// frame holds the grid's scroller (scroll). onAt(kind, at) runs when a bar moves a band's start; kind is 'srcs' or
// 'dsts'. at is in pins with the part of a pin, so the band moves with each px, as the grid does. The Destination band's bar is down the right edge of its pinned names, the Source band's across the bottom
// of its pinned names.

export function pinBars(frame, scroll, onAt) {
  const make = (kind) => {
    const y = kind === 'dsts';
    const fill = el('div', 'sode-gridpanel__pins-fill');
    const sc = el('div', `sode-gridpanel__pins sode-gridpanel__pins--${y ? 'y' : 'x'}`, fill);
    sc.setAttribute('aria-hidden', 'true');
    sc.tabIndex = -1;
    sc.hidden = true;
    frame.append(sc);
    const bar = bigScroll(sc, y ? 'y' : 'x', 'little', { watch: fill });
    let size = 1;
    let at = 0;
    const pos = () => (y ? sc.scrollTop : sc.scrollLeft);
    const put = (v) => {
      sc[y ? 'scrollTop' : 'scrollLeft'] = v;
    };
    // A wheel step glides to its goal as the grid's own scroll does; a trackpad's small steps go at once. goal is
    // null with no glide running.
    let goal = null;
    let from = 0;
    let t0 = 0;
    let raf = 0;
    const glide = (now) => {
      const k = Math.min(1, (now - t0) / GLIDE_MS);
      const v = from + (goal - from) * (1 - (1 - k) ** 3);
      put(v);
      if (k < 1 && Math.abs(pos() - goal) >= 0.5) raf = requestAnimationFrame(glide);
      else {
        put(goal);
        raf = 0;
        goal = null;
      }
    };
    const halt = () => {
      cancelAnimationFrame(raf);
      raf = 0;
      goal = null;
    };
    sc.addEventListener(
      'scroll',
      () => {
        // Under half a px from a whole pin is that pin, since the browser rounds the offset to a device px.
        const v = pos() / size;
        const k = Math.abs(v - Math.round(v)) * size < 0.5 ? Math.round(v) : v;
        if (k === at) return;
        at = k;
        onAt(kind, k);
      },
      { passive: true },
    );
    return {
      // b is the band ({ pins, slots, at }), px the view's sizes ({ xp, w, h }). Set on new axes or a resize only.
      set(b, px) {
        const on = b.pins > b.slots;
        if (sc.hidden === on) sc.hidden = !on;
        if (!on) return;
        size = px.xp;
        const s = sc.style;
        const left = scroll.offsetLeft + px.w;
        const top = scroll.offsetTop + px.h;
        if (y) {
          s.left = `calc(${left}px - var(--sode-littlescroll-w))`;
          s.top = `${top}px`;
          s.height = `${b.slots * size}px`;
          fill.style.height = `${b.pins * size}px`;
        } else {
          s.left = `${left}px`;
          s.top = `calc(${top}px - var(--sode-littlescroll-w))`;
          s.width = `${b.slots * size}px`;
          fill.style.width = `${b.pins * size}px`;
        }
        this.to(b.at + b.f);
        bar?.now();
      },
      // Puts the bar at band start k (pins, with the part of a pin), with no onAt: the view set it.
      to(k) {
        halt();
        at = k;
        const v = k * size;
        if (Math.abs(pos() - v) >= 1) sc[y ? 'scrollTop' : 'scrollLeft'] = v;
      },
      // d in px. A step of a wheel notch's size glides; a run of small ones (a trackpad) goes at once.
      by(d) {
        const max = (y ? sc.scrollHeight - sc.clientHeight : sc.scrollWidth - sc.clientWidth) || 0;
        const base = goal ?? pos();
        const next = Math.max(0, Math.min(max, base + d));
        if (Math.abs(d) < STEP_PX) {
          halt();
          return put(next);
        }
        goal = next;
        from = pos();
        t0 = performance.now();
        raf ||= requestAnimationFrame(glide);
      },
      stop() {
        halt();
        bar?.stop();
        sc.remove();
      },
    };
  };
  return { srcs: make('srcs'), dsts: make('dsts') };
}
