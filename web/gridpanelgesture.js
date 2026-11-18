// Drawn line gestures on a Grid panel: a mouse or pen drag on the crosspoints, or a touch long-press.
// Also the touch gestures on names: a long-press adds a range and a double-tap jumps. A mouse or pen drag along a
// bar picks the names it passes. g is the drawn grid (gridpanelview.js). on: { start(a), move(b), end(b), cancel(),
// name(kind, n, how), span(kind, from, to, add), spanEnd() }. Points are in crosspoint units. kind is 'srcs' or
// 'dsts' and how is 'long' or 'double'. add is true with Cmd or Ctrl held.
// touch-action cannot change after pointerdown. So touch uses Touch Events, and a non-passive touchmove calls
// preventDefault only after the long-press timer. A plain touch drag stays a native scroll.
import { isLine } from './gridpanelline.js';

const LONG_MS = 450;
const DOUBLE_MS = 350; // the most time between the two taps of a double-tap.
const SLOP_PX = 10; // a touch that moves this far before the timer is a scroll
// Edge auto-scroll, in crosspoints, so phone and desktop feel alike. Time-based, so a 120 Hz screen is not twice as
// fast. The depth is capped at the zone: a finger past the body (over the names or the band) does not speed it up.
const EDGE_MAX = 10; // crosspoints per second at the full depth.
const EDGE_ZONE = 0.15; // the zone's part of the body's side, at most one crosspoint.
const EDGE_RAMP_MS = 400; // from rest to full speed. A press that starts near an edge does not jump.

export function lineGestures(g, on) {
  const { scroll } = g;
  let down = null; // { a, id }: a mouse or pen press on a crosspoint, not yet past the click threshold.
  let active = null; // { kind: 'pointer' | 'touch', x, y }: a line draws; x and y are the last client point.
  let span = null; // { kind, from, to, add, id, on, x, y }: a mouse or pen press on a name; on is true once it left that name.
  let timer = 0;
  let touchAt = null; // the client point of a pending long-press.
  let nameAt = null; // { kind, n } of a pending long-press on a name.
  let held = false; // a long-press on a name fired, so its touchend must not click.
  let lastTap = null; // { kind, n, t } of the last tap on a name, for a double-tap.
  let eat = false; // the click that follows a line's release
  let raf = 0;

  const units = (x, y) => g.unitsAt(x, y);
  const begin = (kind, a, x, y) => {
    active = { kind, x, y };
    on.start(a);
    // on.start cancels while a take is in flight or the page is offline.
    if (active) raf = requestAnimationFrame(edge);
  };
  // The name of a bar at a client point, clamped to the bar's ends, so the drag can leave the bar.
  const barName = (kind, x, y) => {
    const u = units(x, y);
    const list = g.axes[kind];
    return list[Math.max(0, Math.min(list.length - 1, Math.floor(kind === 'srcs' ? u.x : u.y)))];
  };
  const spanTo = (x, y) => {
    span.x = x;
    span.y = y;
    const to = barName(span.kind, x, y);
    if (to === span.to) return;
    span.to = to;
    span.on = true;
    on.span(span.kind, span.from, to, span.add);
  };
  const stop = () => {
    if (span?.on) on.spanEnd();
    span = null;
    active = null;
    down = null;
    clearTimeout(timer);
    timer = 0;
    touchAt = null;
    nameAt = null;
    cancelAnimationFrame(raf);
    edgeT = 0;
  };
  const moveTo = (x, y) => {
    if (!active) return;
    active.x = x;
    active.y = y;
    on.move(units(x, y));
  };
  const finish = (x, y) => {
    const b = units(x, y);
    stop();
    on.end(b);
  };
  const cancel = () => {
    const was = active;
    stop();
    if (was) on.cancel();
  };

  // Near an edge of the body the grid scrolls, faster the further in; the end point is read again after it. A drag on
  // a bar scrolls only along that bar. Speed goes with the square of the depth.
  let edgeT = 0; // the last frame's time, 0 at rest
  let edgeFrom = 0; // when this edge scroll started
  const carry = { x: 0, y: 0 }; // sub-pixel steps, which scrollBy would drop
  const step = (k, d) => {
    carry[k] += d;
    const whole = Math.trunc(carry[k]);
    carry[k] -= whole;
    return whole;
  };
  function edge(now) {
    const at = active || span;
    if (!at) return;
    const box = g.bodyBox();
    const xp = g.metrics().xp;
    const by = (p, lo, hi) => {
      const zone = Math.min(xp, (hi - lo) * EDGE_ZONE);
      const d = p < lo + zone ? -(lo + zone - p) : p > hi - zone ? p - (hi - zone) : 0;
      const f = Math.min(1, Math.abs(d) / zone);
      return Math.sign(d) * f * f;
    };
    const fx = span?.kind === 'dsts' ? 0 : by(at.x, box.left, box.right);
    const fy = span?.kind === 'srcs' ? 0 : by(at.y, box.top, box.bottom);
    if (fx || fy) {
      if (!edgeT) edgeT = edgeFrom = now;
      const dt = Math.min(50, now - edgeT);
      edgeT = now;
      const v = (EDGE_MAX * xp * Math.min(1, (now - edgeFrom) / EDGE_RAMP_MS) * dt) / 1000;
      scroll.scrollBy(step('x', fx * v), step('y', fy * v));
      if (active) on.move(units(active.x, active.y));
      else spanTo(span.x, span.y);
    } else edgeT = carry.x = carry.y = 0;
    raf = requestAnimationFrame(edge);
  }

  // Mouse and pen. Capture starts only past the click threshold: with capture the click goes to the scroller, so a
  // plain press must not have it.
  const onDown = (e) => {
    eat = false;
    if (e.pointerType === 'touch' || e.button !== 0 || active || span) return;
    const name = !g.head.contains(e.target) && e.target.closest?.('.sode-gridpanel__src, .sode-gridpanel__dst');
    if (name) {
      const kind = name.dataset.src ? 'srcs' : 'dsts';
      const n = Number(name.dataset.src ?? name.dataset.dst);
      span = { kind, from: n, to: n, add: e.metaKey || e.ctrlKey, id: e.pointerId, on: false, x: e.clientX, y: e.clientY };
      return;
    }
    if (!g.hit(e.clientX, e.clientY)) return;
    down = { a: units(e.clientX, e.clientY), id: e.pointerId };
  };
  const onMove = (e) => {
    if (active?.kind === 'pointer') return moveTo(e.clientX, e.clientY);
    if (span && e.pointerId === span.id) {
      const was = span.on;
      spanTo(e.clientX, e.clientY);
      if (span.on && !was) {
        scroll.setPointerCapture(e.pointerId);
        raf = requestAnimationFrame(edge);
      }
      return;
    }
    if (!down || e.pointerId !== down.id) return;
    if (!isLine(down.a, units(e.clientX, e.clientY))) return;
    scroll.setPointerCapture(e.pointerId);
    const { a } = down;
    down = null;
    begin('pointer', a, e.clientX, e.clientY);
    moveTo(e.clientX, e.clientY);
  };
  const onUp = (e) => {
    down = null;
    if (span) {
      eat = span.on;
      return stop();
    }
    if (active?.kind !== 'pointer') return;
    eat = true;
    finish(e.clientX, e.clientY);
  };
  const onPointerCancel = (e) => {
    if (e.pointerType === 'touch') return;
    down = null;
    if (span) return stop();
    if (active?.kind === 'pointer') cancel();
  };
  const onClick = (e) => {
    if (!eat) return;
    eat = false;
    e.stopPropagation();
    e.preventDefault();
  };

  // Touch. A stylus draws through the pointer path; its touchmove is held so the page does not pan under it.
  const stylus = (e) => [...e.changedTouches].some((t) => t.touchType === 'stylus');
  const onTouchStart = (e) => {
    if (stylus(e)) return;
    if (e.touches.length !== 1) return cancel();
    const t = e.touches[0];
    held = false;
    if (active) return;
    // A name node moves to another item on a scroll, so the item is read at the touch.
    const xp = g.hit(t.clientX, t.clientY);
    const name = !xp && e.target.closest?.('.sode-gridpanel__src, .sode-gridpanel__dst');
    if (!xp && !name) return;
    touchAt = { x: t.clientX, y: t.clientY };
    nameAt = name ? { kind: name.dataset.src ? 'srcs' : 'dsts', n: Number(name.dataset.src ?? name.dataset.dst) } : null;
    timer = setTimeout(() => {
      timer = 0;
      if (nameAt) {
        held = true;
        lastTap = null;
        on.name(nameAt.kind, nameAt.n, 'long');
      } else if (touchAt) begin('touch', units(touchAt.x, touchAt.y), touchAt.x, touchAt.y);
    }, LONG_MS);
  };
  // Non-passive, so it costs a scroll on every touch move: it returns at once when nothing is pending.
  const onTouchMove = (e) => {
    if (!timer && !active && !down) return;
    if (stylus(e)) {
      if ((down || active) && e.cancelable) e.preventDefault();
      return;
    }
    const t = e.touches[0];
    if (active?.kind === 'touch') {
      if (e.cancelable) e.preventDefault();
      return moveTo(t.clientX, t.clientY);
    }
    if (timer && Math.hypot(t.clientX - touchAt.x, t.clientY - touchAt.y) > SLOP_PX) {
      clearTimeout(timer);
      timer = 0;
      touchAt = null;
      nameAt = null;
    }
  };
  const onTouchEnd = (e) => {
    const tapped = timer && nameAt;
    clearTimeout(timer);
    timer = 0;
    touchAt = null;
    nameAt = null;
    // The long-press already picked; its click would pick again.
    if (held) {
      held = false;
      if (e.cancelable) e.preventDefault();
      return;
    }
    // Both taps still click, so a double-tap leaves the picks as they were.
    if (tapped) {
      const now = e.timeStamp;
      const twice = lastTap && lastTap.kind === tapped.kind && lastTap.n === tapped.n && now - lastTap.t < DOUBLE_MS;
      lastTap = twice ? null : { ...tapped, t: now };
      if (twice) on.name(tapped.kind, tapped.n, 'double');
    }
    if (active?.kind !== 'touch') return;
    // No compatibility mouse events and click after a line.
    if (e.cancelable) e.preventDefault();
    const t = e.changedTouches[0];
    finish(t.clientX, t.clientY);
  };
  const onTouchCancel = () => {
    held = false;
    if (timer || active?.kind === 'touch') cancel();
  };
  // A long-press opens the context menu on Android; while one is pending, drawing or held, it is the grid's.
  const onMenu = (e) => {
    if (timer || held || active?.kind === 'touch') e.preventDefault();
  };

  const listen = [
    [scroll, 'pointerdown', onDown],
    [scroll, 'pointermove', onMove],
    [scroll, 'pointerup', onUp],
    [scroll, 'pointercancel', onPointerCancel],
    [scroll, 'click', onClick, { capture: true }],
    [scroll, 'touchstart', onTouchStart, { passive: true }],
    [scroll, 'touchmove', onTouchMove, { passive: false }],
    [scroll, 'touchend', onTouchEnd, { passive: false }],
    [scroll, 'touchcancel', onTouchCancel],
    [scroll, 'contextmenu', onMenu],
    [window, 'blur', cancel],
  ];
  for (const [t, k, f, o] of listen) t.addEventListener(k, f, o);
  return {
    // Esc in the run.
    cancel,
    stop() {
      stop();
      for (const [t, k, f, o] of listen) t.removeEventListener(k, f, o);
    },
  };
}
