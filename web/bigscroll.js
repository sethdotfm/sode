// Big scroll bars that Sode draws itself (design rule in AGENTS.md, docs/agents/web-layout.md).
// Firefox cannot size a native bar, and macOS overlay bars hide until a scroll. So the bar is a
// track and thumb next to the scroller. The native bar is hidden by [data-bigscroll] in base.css.
import { thumbFor, thumbAt, dragTo, pageDir, wheelPx } from './bigscrollrules.js';

// Every scroller a user needs on the pages that run bigScrollAll. A selector that matches no
// scroller costs nothing; keep in step with the list in docs/agents/web-layout.md.
// [selector, axis, size]: size is 'big' (default) or 'little'. The first match wins, so little
// entries go before a broader big one.
export const SCROLLERS = [
  ['.sode__search .sode__menu', 'y', 'little'],
  ['.sode__menu', 'y'],
  ['.sode__found', 'y'],
  ['.sode__scroll', 'y'],
  ['#settings-toc', 'y'],
  ['.pb-modal__body', 'y'],
  ['.sode__preset-list', 'y'],
  ['.sode__key-list', 'y', 'little'],
  ['[data-sets] > .sode__sets-run', 'x', 'little'],
  ['.sode__sets-run', 'x'],
  ['.sode__box--levels .sode__levels', 'x'],
  ['.sode-log__scroll', 'x'],
  ['.sode-ed__side', 'y'],
  ['.sode-live__art', 'y'],
  ['.sode-edge__list', 'y'],
  ['.sode-rlist', 'y'],
  ['.sode-side__icons', 'y'],
  ['.sode-find__list', 'y'],
  ['.sode-tpl__list', 'y'],
  ['.sode-ov__people', 'y'],
  ['.sode-ov__keys', 'y'],
];

const PAGE = 0.9; // a track press pages by this much of the view, so a line stays in sight
const HOLD_MS = 350; // a held press starts repeating after this
const REPEAT_MS = 60;
const bars = new WeakMap(); // scroller -> its handle
const owners = new WeakMap(); // bar or corner -> its scroller

// 'xy' adds a corner square where the bars meet; the page's bar is always big and fixed at the window's edge. watch
// replaces the subtree MutationObserver for a scroller whose nodes change at each scroll (the Grid). Null for a
// scroller out of the page. room() is the px the bars take inside the client box. A scroll changes only the thumb's
// transform (the Grid needs this).
export function bigScroll(scroller, axis = 'y', size = 'big', { watch } = {}) {
  const had = bars.get(scroller);
  if (had) return had;
  const page = scroller === document.scrollingElement;
  if (!page && !scroller.parentElement?.isConnected) return null;
  const little = size === 'little' && !page;
  scroller.dataset.bigscroll = axis;
  if (!page) {
    const host = scroller.parentElement;
    if (getComputedStyle(host).position === 'static') host.classList.add('sode-bigscroll-host');
  }
  // y first: its gutter can make the x axis overflow.
  const parts = (axis === 'xy' ? ['y', 'x'] : [axis]).map((a) => track(scroller, a, little, page));
  const py = parts.find((p) => p.axis === 'y');
  const px = parts.find((p) => p.axis === 'x');
  let corner = null;
  if (py && px) {
    corner = document.createElement('div');
    corner.className = 'sode-bigscroll__corner';
    corner.setAttribute('aria-hidden', 'true');
    corner.hidden = true;
    scroller.after(corner);
    owners.set(corner, scroller);
  }
  for (const p of parts) owners.set(p.bar, scroller);

  let raf = 0;
  function layout() {
    raf = 0;
    // The phone layout keeps native bars (base.css).
    const phone = document.documentElement.dataset.layout === 'phone';
    const shown = !phone && (page || scroller.getClientRects().length > 0);
    for (const p of parts) p.decide(shown);
    // The x gutter can make the y axis overflow in turn.
    if (corner) py.decide(shown);
    for (const p of parts) p.draw(p === py ? px : py);
    if (!corner) return;
    const both = py.want && px.want;
    if (corner.hidden !== !both) corner.hidden = !both;
    if (!both) return;
    const [t, l] = origin(scroller, corner);
    const s = corner.style;
    const want = [`${t + scroller.clientHeight - px.thick}px`, `${l + scroller.clientWidth - py.thick}px`, `${py.thick}px`, `${px.thick}px`];
    if (s.top !== want[0]) s.top = want[0];
    if (s.left !== want[1]) s.left = want[1];
    if (s.width !== want[2]) s.width = want[2];
    if (s.height !== want[3]) s.height = want[3];
  }
  const update = () => {
    if (!raf) raf = requestAnimationFrame(layout);
  };
  let moving = 0;
  const onScroll = () => {
    if (!moving)
      moving = requestAnimationFrame(() => {
        moving = 0;
        for (const p of parts) p.move();
      });
  };

  const target = page ? window : scroller;
  target.addEventListener('scroll', onScroll, { passive: true });
  // A sticky scroller moves with the page without a resize.
  const sticky = !page && getComputedStyle(scroller).position === 'sticky';
  if (sticky) window.addEventListener('scroll', update, { passive: true });
  const ro = new ResizeObserver(update);
  for (const p of parts) ro.observe(p.bar);
  let mo = null;
  if (page) {
    // The body is often one window high with the page overflowing it, so its children are watched.
    const look = () => {
      for (const c of document.body.children) ro.observe(c);
      update();
    };
    mo = new MutationObserver(look);
    mo.observe(document.body, { childList: true });
    look();
    window.addEventListener('resize', update);
  } else {
    ro.observe(scroller);
    ro.observe(scroller.parentElement);
    if (watch) ro.observe(watch);
    else {
      // Rows drawn into a capped list change scrollHeight with no resize of the list.
      mo = new MutationObserver(update);
      mo.observe(scroller, { childList: true, subtree: true, attributes: true, attributeFilter: ['style', 'hidden'] });
    }
  }
  update();

  function stop() {
    if (bars.get(scroller) !== handle) return;
    bars.delete(scroller);
    cancelAnimationFrame(raf);
    cancelAnimationFrame(moving);
    ro.disconnect();
    mo?.disconnect();
    target.removeEventListener('scroll', onScroll);
    if (sticky) window.removeEventListener('scroll', update);
    if (page) window.removeEventListener('resize', update);
    for (const p of parts) p.stop();
    corner?.remove();
    delete scroller.dataset.bigscroll;
  }
  const room = () => ({ right: py?.want ? py.thick : 0, bottom: px?.want ? px.thick : 0 });
  // now() lays out at once, for a caller that reads room() next.
  const now = () => {
    cancelAnimationFrame(raf);
    layout();
  };
  const handle = { update, now, stop, bar: parts[0].bar, room };
  bars.set(scroller, handle);
  return handle;
}

// The scroller's padding box top left, in the containing block of node (its sibling).
function origin(scroller, node) {
  let t = scroller.offsetTop + scroller.clientTop;
  let l = scroller.offsetLeft + scroller.clientLeft;
  const box = node.offsetParent;
  // A host with display: contents leaves the bar's containing block elsewhere.
  if (box && box !== scroller.offsetParent) {
    const sr = scroller.getBoundingClientRect();
    const br = box.getBoundingClientRect();
    t = sr.top - br.top - box.clientTop + box.scrollTop + scroller.clientTop;
    l = sr.left - br.left - box.clientLeft + box.scrollLeft + scroller.clientLeft;
  }
  return [t, l];
}

// One bar on one axis. decide() sets whether it shows, and its gutter. draw(other) places it,
// shorter by other's thickness while other shows.
function track(scroller, axis, little, page) {
  const y = axis === 'y';
  const width = little ? 'var(--sode-littlescroll-w)' : 'var(--sode-bigscroll-w)';
  const thumb = document.createElement('div');
  thumb.className = 'sode-bigscroll__thumb';
  const bar = document.createElement('div');
  bar.className = `sode-bigscroll sode-bigscroll--${axis}${page ? ' sode-bigscroll--page' : ''}${little ? ' sode-bigscroll--little' : ''}`;
  // Pointer only: keys already scroll, and the bar must not reach tab order or screen readers.
  bar.setAttribute('aria-hidden', 'true');
  bar.hidden = true;
  bar.append(thumb);
  if (page) document.body.append(bar);
  else scroller.after(bar);

  let geo = null; // { track, len, travel, max } from the last layout read
  let at = -1;
  let laid = false;
  let pad = null; // the scroller's own end padding in px, read once while it is drawn

  const view = () => (y ? scroller.clientHeight : scroller.clientWidth);
  const content = () => (y ? scroller.scrollHeight : scroller.scrollWidth);
  const pos = () => (y ? scroller.scrollTop : scroller.scrollLeft);
  const setPos = (v) => (y ? (scroller.scrollTop = v) : (scroller.scrollLeft = v));

  // thick is the bar's own px across, read at each draw; want is true while it shows.
  const part = { axis, bar, want: false, thick: 0, decide, draw, move, stop };

  function place(cut) {
    if (page) return;
    const [t, l] = origin(scroller, bar);
    const s = bar.style;
    const want = y
      ? [`${t}px`, `${l + scroller.clientWidth - part.thick}px`, `${scroller.clientHeight - cut}px`, '']
      : [`${t + scroller.clientHeight - part.thick}px`, `${l}px`, '', `${scroller.clientWidth - cut}px`];
    if (s.top !== want[0]) s.top = want[0];
    if (s.left !== want[1]) s.left = want[1];
    if (s.height !== want[2]) s.height = want[2];
    if (s.width !== want[3]) s.width = want[3];
  }

  // The gutter is the bar's own room, so the thumb never covers a row's end. It is there only while
  // the bar shows, as with a classic bar.
  function gutter(on) {
    if (on === laid) return;
    laid = on;
    if (page) {
      if (on) document.documentElement.dataset.bigscrollOn = '';
      else delete document.documentElement.dataset.bigscrollOn;
      return;
    }
    const side = y ? 'paddingRight' : 'paddingBottom';
    if (pad === null) pad = getComputedStyle(scroller)[side];
    scroller.style[side] = on ? `calc(${pad} + ${width})` : '';
  }

  function decide(shown) {
    part.want = shown && content() - view() > 1;
    gutter(part.want);
    if (bar.hidden !== !part.want) bar.hidden = !part.want;
    if (!part.want) geo = null;
  }

  // other is the bar on the other axis. It can be drawn after this one, so its thickness is read here.
  function draw(other) {
    if (!part.want) return;
    part.thick = y ? bar.offsetWidth : bar.offsetHeight;
    place(other?.want ? (y ? other.bar.offsetHeight : other.bar.offsetWidth) : 0);
    const track = y ? bar.clientHeight : bar.clientWidth;
    const min = parseFloat(getComputedStyle(thumb)[y ? 'minHeight' : 'minWidth']) || 0;
    const { len, travel } = thumbFor(view(), content(), track, min);
    const size = `${len}px`;
    if (thumb.style[y ? 'height' : 'width'] !== size) thumb.style[y ? 'height' : 'width'] = size;
    geo = { track, len, travel, max: content() - view() };
    at = -1;
    move();
  }

  function move() {
    if (!geo) return;
    const off = thumbAt(pos(), geo.max, geo.travel);
    if (off === at) return;
    at = off;
    thumb.style.transform = y ? `translateY(${off}px)` : `translateX(${off}px)`;
  }

  // Drag and track presses. pointerdown and mousedown both keep focus where it is, so an open
  // match list stays open.
  let drag = null;
  let hold = 0;
  const along = (e) => {
    const r = bar.getBoundingClientRect();
    return y ? e.clientY - r.top : e.clientX - r.left;
  };
  function pageStep(pt) {
    if (!geo) return false;
    const d = pageDir(pt, at, geo.len);
    if (!d) return false;
    setPos(pos() + d * view() * PAGE);
    return true;
  }
  function stopHold() {
    clearTimeout(hold);
    hold = 0;
  }
  function onDown(e) {
    e.preventDefault();
    e.stopPropagation();
    if (e.button !== 0 || !geo) return;
    bar.setPointerCapture(e.pointerId);
    const pt = along(e);
    if (e.target === thumb) {
      drag = { from: pos(), start: pt, pt };
      bar.classList.add('sode-bigscroll--drag');
      return;
    }
    drag = { pt };
    if (!pageStep(pt)) return;
    const again = () => {
      if (drag && pageStep(drag.pt)) hold = setTimeout(again, REPEAT_MS);
    };
    hold = setTimeout(again, HOLD_MS);
  }
  function onMove(e) {
    if (!drag) return;
    drag.pt = along(e);
    if (drag.from !== undefined && geo) setPos(dragTo(drag.from, drag.pt - drag.start, geo.max, geo.travel));
  }
  function onUp() {
    drag = null;
    stopHold();
    bar.classList.remove('sode-bigscroll--drag');
  }
  const swallow = (e) => {
    e.preventDefault();
    e.stopPropagation();
  };
  // Over the window's edge the wheel reaches the page by itself; next to an element it would not.
  function onWheel(e) {
    // A pinch comes as a wheel with ctrlKey; it keeps the browser's zoom.
    if (page || e.ctrlKey) return;
    e.preventDefault();
    const d = wheelPx(e.deltaX, e.deltaY, e.deltaMode, axis, view());
    setPos(pos() + d);
  }

  bar.addEventListener('pointerdown', onDown);
  bar.addEventListener('pointermove', onMove);
  bar.addEventListener('pointerup', onUp);
  bar.addEventListener('pointercancel', onUp);
  bar.addEventListener('lostpointercapture', onUp);
  bar.addEventListener('mousedown', swallow);
  bar.addEventListener('click', swallow);
  bar.addEventListener('wheel', onWheel, { passive: false });

  function stop() {
    stopHold();
    gutter(false);
    bar.remove();
  }
  return part;
}

// Draws a bar on every scroller in SCROLLERS under root, now and as they are added, plus the page's
// own bar. A scroller taken out of the page loses its bar.
export function bigScrollAll(root = document.body) {
  bigScroll(document.scrollingElement, 'y');
  // One query per added or removed node: match lists and History redraw many rows per keystroke.
  const all = SCROLLERS.map(([sel]) => sel).join(',');
  const attach = (m) => {
    const [, axis, size] = SCROLLERS.find(([sel]) => m.matches(sel));
    bigScroll(m, axis, size);
  };
  const scan = (n) => {
    if (n.matches(all)) attach(n);
    if (n.firstElementChild) for (const m of n.querySelectorAll(all)) attach(m);
  };
  const drop = (n) => {
    bars.get(n)?.stop();
    if (n.firstElementChild) for (const m of n.querySelectorAll('[data-bigscroll]')) bars.get(m)?.stop();
  };
  scan(root);
  new MutationObserver((list) => {
    for (const r of list) {
      for (const n of r.removedNodes) {
        if (n.nodeType !== 1 || n.isConnected) continue;
        // A host that redraws its children can take the bar and keep the scroller.
        const s = owners.get(n);
        if (s?.isConnected && bars.has(s)) {
          s.after(n);
          bars.get(s).update();
        } else drop(n);
      }
      for (const n of r.addedNodes)
        if (n.nodeType === 1 && n.isConnected && !n.classList.contains('sode-bigscroll') && !n.classList.contains('sode-bigscroll__corner')) scan(n);
    }
  }).observe(root, { childList: true, subtree: true });
}
