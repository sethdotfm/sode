// Where the view is in a Grid panel, on the phone layout only. A phone draws its scroll bars only while a scroll
// runs. So the corner gives the shown ranges, and the body's bottom and right edges get thin strips, display only.
// g is the drawn grid (gridpanelview.js), frame its scroller's parent. A scroll writes only thumb transforms and, when
// a range changes, text node data (TestChromeGridPanelScale).
import { el, icon } from './core.js';
import { thumbOf, rangeText } from './gridpanelposrules.js';

export function gridPosition(g, frame, isPhone) {
  const bar = (axis) => {
    const thumb = el('div', 'sode-gridpanel__pos-thumb');
    const track = el('div', `sode-gridpanel__pos sode-gridpanel__pos--${axis}`, thumb);
    track.setAttribute('aria-hidden', 'true');
    return { track, thumb, axis, len: 0, key: '', trackKey: '' };
  };
  const bars = { srcs: bar('x'), dsts: bar('y') };
  frame.append(bars.srcs.track, bars.dsts.track);

  const head = g.field.querySelector('.sode-gridpanel__head');
  const line = (k, name) => {
    const text = document.createTextNode('');
    const n = el('span', `sode-gridpanel__range sode-gridpanel__range--${k}`, icon(name), text);
    n.setAttribute('aria-hidden', 'true');
    // Before the words: the Destinations word stays the corner's last child.
    head?.querySelector('.sode-gridpanel__label--src')?.before(n);
    return { n, text };
  };
  const ranges = { srcs: line('src', 'arrow-right'), dsts: line('dst', 'arrow-down') };

  let shown = null;
  const place = () => {
    if (!isPhone()) return;
    shown = g.shown();
    const box = g.bodyBox();
    const f = frame.getBoundingClientRect();
    for (const k of ['srcs', 'dsts']) {
      const b = bars[k];
      const s = shown[k];
      const x = b.axis === 'x';
      b.len = x ? box.right - box.left : box.bottom - box.top;
      // The track: along the body's bottom or right edge, in frame px. CSS moves it inside that edge by its width.
      const left = (x ? box.left : box.right) - f.left;
      const top = (x ? box.bottom : box.top) - f.top;
      const tk = `${left},${top},${b.len}`;
      if (tk !== b.trackKey) {
        b.trackKey = tk;
        b.track.style.left = `${left}px`;
        b.track.style.top = `${top}px`;
        b.track.style[x ? 'width' : 'height'] = `${b.len}px`;
      }
      b.track.hidden = !(s.max > 0);
      const t = thumbOf(s, b.len);
      const key = `${Math.round(t.size)},${Math.round(t.at)}`;
      if (key !== b.key) {
        b.key = key;
        b.thumb.style[x ? 'width' : 'height'] = `${t.size}px`;
        b.thumb.style.transform = x ? `translateX(${t.at}px)` : `translateY(${t.at}px)`;
      }
      const r = ranges[k];
      const words = rangeText(s);
      if (r.text.data !== words) r.text.data = words;
    }
  };
  const off = g.onDraw(place);

  place();

  return {
    stop() {
      off();
      for (const b of Object.values(bars)) b.track.remove();
      for (const r of Object.values(ranges)) r.n.remove();
    },
  };
}
