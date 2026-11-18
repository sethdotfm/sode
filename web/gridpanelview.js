// Draws a Grid panel, virtualised on both axes (gridpanelwindow.js picks what shows). The Source bar is across
// the top and the Destination bar down the side. Pinned items sit in sticky bands at those edges.
// A node is moved to another crosspoint or name as the grid scrolls, so nodes hold no state of their own.
// Everything shown comes from the view that paint is given. docs/agents/panels.md has the layer plan.
import { el, icon } from './core.js';
import { iconHref } from './sprite.js';
import { pieFill } from './gridpanellevels.js';
import { bigScroll } from './bigscroll.js';
import {
  axisSpan,
  spanMax,
  slotAt,
  pageOf,
  placeAt,
  scrollFor,
  centreFor,
  wheelAcross,
  pinsShown,
  itemOf,
  slotOf,
  bandFor,
  bandCentre,
  wheelBand,
  OVER,
} from './gridpanelwindow.js';
import { pinBars } from './gridpanelpins.js';

const HIT_MS = 1400; // keep in step with the sode-gridpanel-hit animations in gridpanel.css (3 x 0.45s).
const GLIDE_MAX_MS = 3000; // a glide across 2500 sources takes about 1.5 s

// A node that starts the panel pulse (sode-panel-pulse) joins the others in phase. Every pulse counts from the
// timeline's time zero. One read per task, since getAnimations brings the style up to date.
const joining = new Set();
export function joinPulse(node) {
  if (!joining.size)
    queueMicrotask(() => {
      for (const n of joining) for (const a of n.getAnimations()) if (a.animationName === 'sode-panel-pulse') a.startTime = 0;
      joining.clear();
    });
  joining.add(node);
}

// The number each name of a press preview carries, the same as its crosspoint's, so the two line up by eye. A name in
// more than one numbered pair (a line's one source) gets '' and shows no number. key is 'src' or 'dst'.
const pairNumbers = (pairs, key) => {
  const m = new Map();
  pairs.forEach((p, k) => m.set(p[key], m.has(p[key]) ? '' : String(k + 1)));
  return m;
};

// A name node keeps one Text child, so a rename or a move is not a childList change.
const setText = (n, t) => {
  if (n.firstChild.data !== t) n.firstChild.data = t;
};
const setData = (n, k, v) => {
  if (v === undefined) {
    if (k in n.dataset) delete n.dataset[k];
  } else if (n.dataset[k] !== v) n.dataset[k] = v;
};
const pick = (n, on) => {
  setData(n, 'picked', on ? '' : undefined);
  const a = on ? 'true' : 'false';
  if (n.getAttribute('aria-selected') !== a) n.setAttribute('aria-selected', a);
};
const setVar = (n, k, v) => {
  const s = String(v);
  if (n.style.getPropertyValue(k) !== s) n.style.setProperty(k, s);
};

function paintXp(xp, s, d, view) {
  const on = view.pie(s, d);
  const fill = pieFill(view.levels, on);
  const [pie, ring] = xp.children;
  // The browser rewrites a style it reads back, so the last value set is kept on the node.
  if (pie._fill !== fill) {
    pie._fill = fill;
    pie.style.background = fill;
  }
  // data-s staged is the attribute the panel pulse watches (panels.css). A route left out does not pulse.
  const staged = view.staged(s, d);
  // The staged ring is a pie of the staged levels, in the same slices as the pie.
  const ringFill = staged.length ? pieFill(view.levels, staged) : '';
  if (ring._fill !== ringFill) {
    ring._fill = ringFill;
    ring.style.background = ringFill;
  }
  // A picked row or column is washed in its role colour (gridpanel.css).
  setData(xp, 'row', view.picked('dsts', d) ? '' : undefined);
  setData(xp, 'col', view.picked('srcs', s) ? '' : undefined);
  const out = staged.length > 0 && view.leftOut(s, d);
  const prot = view.protected(d);
  const st = out ? 'left-out' : staged.length ? 'staged' : undefined;
  // A staged crosspoint runs the pulse itself (gridpanel.css).
  const starts = st === 'staged' && xp.dataset.s !== 'staged';
  setData(xp, 's', st);
  if (starts) joinPulse(xp);
  setData(xp, 'on', on.length ? on.join(',') : undefined);
  if (xp.getAttribute('aria-disabled') !== String(prot)) xp.setAttribute('aria-disabled', String(prot));
  const state = out ? ', staged, left out of the take' : staged.length ? ', staged. Right-click or Alt-click to unstage' : '';
  const label = `${view.names.source(s)} to ${view.names.destination(d)}${on.length ? `, levels ${view.levelText(on)}` : ''}${prot ? ', protected' : ''}${state}`;
  if (xp.getAttribute('aria-label') !== label) xp.setAttribute('aria-label', label);
}

const makeXp = () => {
  const xp = el('button', 'sode-gridpanel__xp', el('span', 'sode-gridpanel__pie'), el('span', 'sode-gridpanel__stage'));
  xp.type = 'button';
  xp.tabIndex = -1;
  return xp;
};
// No header role: the grid is not an ARIA grid, so a columnheader or rowheader here has no valid parent.
const makeName = (cls) => () => {
  const n = el('div', `pb-xpt ${cls}`);
  n.append(document.createTextNode(''));
  return n;
};
// The artwork beside a name: Icon A as a sprite icon, an image or a mask, then Icon B. A mask is a library SVG in
// the text colour. All four exist from the start and a paint shows some, so a scroll changes attributes and never
// childList. The last child of a name; it comes before the text in the layout (CSS order).
const makeArt = () => {
  const row = el(
    'span',
    'sode-gridpanel__art',
    icon('circle', false),
    el('img', 'sode-gridpanel__art-a'),
    el('span', 'sode-gridpanel__art-a sode-gridpanel__art-mask'),
    el('span', 'sode-gridpanel__art-b', document.createTextNode('')),
  );
  row.setAttribute('aria-hidden', 'true');
  row.hidden = true;
  const [svg, img, mask] = row.children;
  svg.setAttribute('class', 'pb-icon sode-gridpanel__art-a');
  img.hidden = mask.hidden = true;
  svg.setAttribute('hidden', '');
  img.alt = '';
  return row;
};
const makeSrc = () => {
  const n = makeName('pb-xpt--source sode-gridpanel__src')();
  n.append(makeArt());
  return n;
};
// After the name's Text: the protected mark (the lock never stands alone) and the staged word. Then the current
// sources the grid does not show, then the artwork. Made once, so a paint changes only Text data and hidden.
const makeDst = () => {
  const n = makeName('pb-xpt--dest sode-gridpanel__dst')();
  const prot = el('small', 'sode-gridpanel__prot', icon('lock'), document.createTextNode(''));
  const held = el('small', 'sode-gridpanel__held', document.createTextNode(''));
  const other = el('small', 'sode-gridpanel__other', document.createTextNode(''));
  prot.hidden = held.hidden = other.hidden = true;
  n.append(prot, held, other, makeArt());
  return n;
};

// The attribute, not the property: the art row's svg has no hidden property.
const show = (n, on) => {
  if (n.hasAttribute('hidden') === on) n.toggleAttribute('hidden', !on);
};
// art is { icon, url, mask, b, hide } from view.art (a part can be missing), or null. hide marks the name, so CSS
// hides its text and keeps it for screen readers.
function paintArt(row, art) {
  const [svg, img, mask, b] = row.children;
  setText(b, art?.b || '');
  show(b, !!art?.b);
  show(svg, !!art?.icon);
  if (art?.icon) {
    const href = iconHref(art.icon);
    const use = svg.firstChild;
    if (use.getAttribute('href') !== href) use.setAttribute('href', href);
  }
  show(img, !!art?.url);
  if (art?.url && img.getAttribute('src') !== art.url) img.setAttribute('src', art.url);
  show(mask, !!art?.mask);
  if (art?.mask && mask.dataset.u !== art.mask) {
    mask.dataset.u = art.mask;
    mask.style.maskImage = `url("${art.mask}")`;
  }
  const any = !!(art && (art.icon || art.url || art.mask || art.b));
  show(row, any);
  setData(row.parentNode, 'hideText', any && art.hide ? '' : undefined);
}

// A mark not shown is hidden and has empty text, so the node's text is the name alone.
function paintDstMarks(n, d, view) {
  const [, prot, held, other] = n.childNodes;
  const p = view.protected(d);
  n.classList.toggle('pb-xpt--protected', p);
  if (prot.hidden === p) prot.hidden = !p;
  if (prot.lastChild.data !== (p ? ' Protected' : '')) prot.lastChild.data = p ? ' Protected' : '';
  // Protected wins: a protected destination has nothing staged.
  const h = !p && view.held(d);
  setText(held, h ? 'staged' : '');
  if (held.hidden === h) held.hidden = !h;
  const o = view.other(d);
  setText(other, o.text);
  if (other.hidden !== !o.text) other.hidden = !o.text;
  setData(other, 'pbTip', o.tip || undefined);
}

// A pool draws the cells of one layer: a key per cell, nodes reused for cells that come into view.
// put(node, i, j) points a node at the cell of display indices i (row) and j (column).
function pool(host, make, put) {
  const live = new Map();
  const spare = [];
  return {
    live,
    // cells is [[i, j], ...]; a node whose cell went is given to a new one, and only a shortfall makes nodes.
    sync(cells) {
      const want = new Map(cells.map(([i, j]) => [`${i},${j}`, [i, j]]));
      for (const [k, n] of live) {
        if (!want.has(k)) {
          live.delete(k);
          spare.push(n);
        }
      }
      for (const [k, [i, j]] of want) {
        if (live.has(k)) continue;
        let n = spare.pop();
        if (!n) host.append((n = make()));
        n.hidden = false;
        put(n, i, j);
        live.set(k, n);
      }
      // A spare node stays in the DOM, hidden, for the next scroll.
      for (const n of spare) if (!n.hidden) n.hidden = true;
    },
    // Makes nodes up to k, so a scroll never makes one. Called on a resize or new axes, never on a scroll.
    reserve(k) {
      for (let have = live.size + spare.length; have < k; have++) {
        const n = make();
        n.hidden = true;
        host.append(n);
        spare.push(n);
      }
    },
    clear() {
      for (const n of live.values()) spare.push(n);
      live.clear();
    },
  };
}

const range = (lo, hi) => Array.from({ length: Math.max(0, hi - lo) }, (_, k) => lo + k);
const cross = (rows, cols) => rows.flatMap((i) => cols.map((j) => [i, j]));

// host is the run's grid element; axes is gridAxes plus srcPins and dstPins. onPress({ src, dst }, e) is a crosspoint
// press. onName(kind, n, e) is a click on a name, kind 'srcs' or 'dsts'. onPoint(clientX, clientY) is each pointer
// move over the scroller. onJump(kind, n) is a right-click: kind 'xp' with n { src, dst }, or 'srcs' or 'dsts' with a
// router number. Returns the drawn grid (docs/agents/panels.md).
export function buildGridPanel(host, axes, { onPress, onName, onPoint, onJump }) {
  host.replaceChildren();
  host.removeAttribute('style');
  host.classList.add('sode-gridpanel');
  const layer = (cls) => el('div', `sode-gridpanel__layer ${cls}`);
  const body = layer('sode-gridpanel__body');
  const overlay = layer('sode-gridpanel__overlay');
  const top = layer('sode-gridpanel__top');
  const side = layer('sode-gridpanel__side');
  const corner = layer('sode-gridpanel__corner');
  // Where the bars meet: the run puts the search boxes here (gridpanelfindbox.js).
  // The run puts the search boxes first, so the Destinations word stays last, next to the Destination bar. The
  // Sources word is on its side at the corner's right edge, next to the Source bar. Its label is turned 180 degrees
  // (gridpanel.css), so arrow-left draws pointing right, along the Source bar.
  const label = (kind, word, arrow) => el('span', `sode-gridpanel__label sode-gridpanel__label--${kind}`, el('span', null, word), icon(arrow));
  const head = el('div', 'sode-gridpanel__head', label('src', 'Sources', 'arrow-left'), label('dst', 'Destinations', 'arrow-down'));
  corner.append(head);
  // A pinned band's pins sit in a box that moves by the part of a pin (bandVars). It is in a box that clips at the
  // band's edges (gridpanel.css). So a band scroll moves one node and restyles no pin. axes is 'x', 'y' or 'xy'.
  const bandBox = (host, axes) => {
    const box = el('div', 'sode-gridpanel__shift');
    host.append(el('div', `sode-gridpanel__clip sode-gridpanel__clip--${axes}`, box));
    return box;
  };
  const bands = { top: bandBox(top, 'y'), side: bandBox(side, 'x'), corner: bandBox(corner, 'xy'), srcs: bandBox(corner, 'x'), dsts: bandBox(corner, 'y') };
  // The cursor (gridpanelcursor.js) is one node, moved by transform only. It is not in the overlay: on a bar or a
  // pinned item it must draw above the sticky bands.
  const cursor = el('div', 'sode-gridpanel__cursor', el('span', 'sode-gridpanel__ring'));
  cursor.setAttribute('aria-hidden', 'true');
  // The Drawn line, in content px (gridpanelline.js has the geometry).
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.classList.add('sode-gridpanel__lines');
  svg.setAttribute('aria-hidden', 'true');
  const lineNode = document.createElementNS('http://www.w3.org/2000/svg', 'line');
  lineNode.classList.add('sode-gridpanel__line');
  svg.append(lineNode);
  // An svg element has no hidden property: only the attribute hides it.
  svg.setAttribute('hidden', '');
  overlay.append(svg);
  const field = el('div', 'sode-gridpanel__field', body, overlay, side, top, corner, cursor);
  field.setAttribute('role', 'group');
  field.setAttribute('aria-label', 'Crosspoints');
  // The probe is the Destination bar width by the Source bar height, with one crosspoint inside. CSS sets the
  // sizes in rem, and the window maths needs px.
  const probeXp = el('div', 'sode-gridpanel__probe-xp');
  const probe = el('div', 'sode-gridpanel__probe', probeXp);
  const scroll = el('div', 'pb-grid-body sode-gridpanel__scroll', field, probe);
  // Focus goes back here from a search box, so an embedded run (which needs focus in the grid) gets its keys.
  scroll.tabIndex = -1;
  const frameNode = el('div', 'pb-grid sode-gridpanel__frame', scroll);
  host.append(frameNode);
  // Sode's own scroll bars on both axes (bigscroll.js). They are the scroller's siblings, so the grid's own pointer and
  // wheel listeners never see a press or a wheel on them. The field size changes with the axes, so no MutationObserver.
  const scrollBars = bigScroll(scroll, 'xy', 'big', { watch: field });
  // A band start moved by its bar: the band is drawn again, the rest stays.
  const pinBar = pinBars(frameNode, scroll, (kind, k) => setBand(kind, k, false));

  let ax = axes;
  let srcAt = new Map(); // router number to display index
  let dstAt = new Map();
  let view = null;
  // The press preview (g.preview): numbered crosspoints by "s:d" and outlined names.
  let mark = { xps: new Map(), skip: new Set(), srcs: new Map(), dsts: new Map() };
  const marked = new Set(); // crosspoint nodes that carry a number
  let cur = { i: -1, j: -1 }; // The cursor's display position, -1 on a bar.
  // What g.glide or g.jump went to, { srcs, dsts, seq } with router numbers (null for none), until its pulse ends. The
  // crosspoint pulses when both are set. seq is 0 or 1 and picks one of two equal animations. Thus a new glide restarts
  // the pulse on a node that still has the last one. Set when the scroll ends, so every part pulses in one phase.
  let hit = null;
  let seq = 1;
  let hitEnd = null; // { timer, done } of the glide or pulse in progress
  const hitOf = (kind, n) => (hit && hit[kind] === n ? String(hit.seq) : undefined);
  const hitXp = (s, d) => (hit && hit.srcs === s && hit.dsts === d ? String(hit.seq) : undefined);
  const paintHits = () => {
    for (const p of [pools.srcs, pools.srcPinned]) for (const n of p.live.values()) setData(n, 'hit', hitOf('srcs', Number(n.dataset.src)));
    for (const p of [pools.dsts, pools.dstPinned]) for (const n of p.live.values()) setData(n, 'hit', hitOf('dsts', Number(n.dataset.dst)));
    for (const p of xpPools) for (const n of p.live.values()) setData(n, 'hit', hitXp(Number(n.dataset.src), Number(n.dataset.dst)));
  };
  // Scrolls to `to` ({ left, top }, either one) smoothly, then pulses `next` ({ srcs, dsts }) for HIT_MS.
  const goTo = (to, next) => {
    const m = read();
    const calm = matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (hitEnd) {
      clearTimeout(hitEnd.timer);
      scroll.removeEventListener('scrollend', hitEnd.done);
    }
    hit = null;
    paintHits();
    const done = () => {
      scroll.removeEventListener('scrollend', done);
      clearTimeout(hitEnd.timer);
      seq = 1 - seq;
      hit = { ...next, seq };
      paintHits();
      hitEnd.timer = setTimeout(() => {
        hit = null;
        hitEnd = null;
        paintHits();
      }, HIT_MS);
    };
    hitEnd = { timer: 0, done };
    // No scrollend comes for a move under a pixel or on a browser without the event, so a timer ends it too.
    const still = Object.entries(to).every(([k, v]) => Math.abs(v - (k === 'left' ? m.left : m.top)) < 1);
    if (still) return done();
    hitEnd.timer = setTimeout(done, GLIDE_MAX_MS);
    scroll.addEventListener('scrollend', done);
    scroll.scrollTo({ ...to, behavior: calm ? 'instant' : 'smooth' });
  };
  let px = { xp: 1, w: 1, h: 1 };
  const listeners = new Set();

  const measure = () => {
    const r = probe.getBoundingClientRect();
    px = { xp: probeXp.getBoundingClientRect().width || 1, w: r.width || 1, h: r.height || 1 };
  };
  const place = (n, i, j) => {
    setVar(n, '--sode-i', i);
    setVar(n, '--sode-j', j);
  };
  const markXp = (n, s, d) => {
    const k = mark.xps.get(`${s}:${d}`);
    const skip = mark.skip.has(`${s}:${d}`);
    setData(n, 'n', k === undefined ? undefined : String(k));
    setData(n, 'skip', skip ? '' : undefined);
    if (k === undefined && !skip) marked.delete(n);
    else marked.add(n);
  };
  // v is the name's number in mark (a string), '' for a name in more than one numbered pair, undefined for none.
  const markName = (n, v) => {
    setData(n, 'target', v !== undefined ? '' : undefined);
    setData(n, 'n', v || undefined);
  };
  // The names of the crosspoint under the cursor; none while the cursor is on a bar.
  const cueOf = () => (cur.i >= 0 && cur.j >= 0 ? { src: ax.srcs[cur.j], dst: ax.dsts[cur.i] } : {});
  // Pools and the window maths work in slots (gridpanelwindow.js itemOf); ax, the cursor and the run in items.
  const cueName = (n, on) => setData(n, 'cue', on ? '' : undefined);
  const cueNames = () => {
    const c = cueOf();
    for (const p of [pools.srcs, pools.srcPinned]) for (const n of p.live.values()) cueName(n, Number(n.dataset.src) === c.src);
    for (const p of [pools.dsts, pools.dstPinned]) for (const n of p.live.values()) cueName(n, Number(n.dataset.dst) === c.dst);
  };
  const paintSrc = (n, s) => {
    if (view) {
      setText(n, view.names.source(s));
      setData(n, 'num', String(s + (view.names.off || 0)));
      pick(n, view.picked('srcs', s));
      setData(n, 'found', view.found('srcs', s) ? '' : undefined);
      paintArt(n.lastChild, view.art('srcs', s));
    }
    markName(n, mark.srcs.get(s));
    cueName(n, cueOf().src === s);
    setData(n, 'hit', hitOf('srcs', s));
  };
  const paintDst = (n, d) => {
    if (view) {
      setText(n, view.names.destination(d));
      setData(n, 'num', String(d + (view.names.off || 0)));
      pick(n, view.picked('dsts', d));
      setData(n, 'found', view.found('dsts', d) ? '' : undefined);
      paintDstMarks(n, d, view);
      paintArt(n.lastChild, view.art('dsts', d));
    }
    markName(n, mark.dsts.get(d));
    cueName(n, cueOf().dst === d);
    setData(n, 'hit', hitOf('dsts', d));
  };
  // A band pool's nodes are keyed and placed by pin index, and its box moves by the band's start (bandVars). So a
  // band scroll past a whole pin redraws one row or column, not the band. The other pools are in slots (itemOf).
  const srcIn = (inBand) => (j) => ax.srcs[inBand ? j : itemOf(j, band.srcs)];
  const dstIn = (inBand) => (i) => ax.dsts[inBand ? i : itemOf(i, band.dsts)];
  const xpPut = (rowBand, colBand) => {
    const srcOf = srcIn(colBand);
    const dstOf = dstIn(rowBand);
    return (n, i, j) => {
      place(n, i, j);
      const s = srcOf(j);
      const d = dstOf(i);
      n.dataset.src = s;
      n.dataset.dst = d;
      if (view) paintXp(n, s, d, view);
      markXp(n, s, d);
      setData(n, 'hit', hitXp(s, d));
    };
  };
  const srcPut = (inBand) => {
    const srcOf = srcIn(inBand);
    return (n, i, j) => {
      place(n, 0, j);
      const s = srcOf(j);
      n.dataset.src = s;
      paintSrc(n, s);
    };
  };
  const dstPut = (inBand) => {
    const dstOf = dstIn(inBand);
    return (n, i) => {
      place(n, i, 0);
      const d = dstOf(i);
      n.dataset.dst = d;
      paintDst(n, d);
    };
  };
  const pools = {
    body: pool(body, makeXp, xpPut(false, false)),
    top: pool(bands.top, makeXp, xpPut(true, false)),
    side: pool(bands.side, makeXp, xpPut(false, true)),
    corner: pool(bands.corner, makeXp, xpPut(true, true)),
    srcs: pool(top, makeSrc, srcPut(false)),
    dsts: pool(side, makeDst, dstPut(false)),
    srcPinned: pool(bands.srcs, makeSrc, srcPut(true)),
    dstPinned: pool(bands.dsts, makeDst, dstPut(true)),
  };
  const xpPools = [pools.body, pools.top, pools.side, pools.corner];

  // The scroller's metrics. Each read can force a layout, so a draw reads them once (frame) and all it calls share them.
  let frame = null;
  const read = () => {
    const r = scroll.getBoundingClientRect();
    // The drawn scroll bars sit over the client box (native ones sit outside it), so their room is taken off.
    const room = scrollBars ? scrollBars.room() : { right: 0, bottom: 0 };
    return {
      left: scroll.scrollLeft,
      top: scroll.scrollTop,
      cw: scroll.clientWidth - room.right,
      ch: scroll.clientHeight - room.bottom,
      x: r.left + scroll.clientLeft,
      y: r.top + scroll.clientTop,
    };
  };
  const metricsNow = () => frame || read();
  // Each edge's pinned band (gridpanelwindow.js itemOf): slots is how many pins draw as pinned (pinsShown), set by
  // setAxes and a resize. On the desktop layout the pins past them scroll in the band: its bar, the wheel, a key or
  // Find moves it. The phone has no band bars, so there the pins past them scroll first with the rest.
  // f is the part of a pin the band has moved past at (0 to 1). Thus the band moves with the wheel px by px, as the
  // grid does. While the band can move on, its pools draw one slot more, the pin that shows in part at the far end.
  const band = { srcs: { pins: 0, slots: 0, at: 0, f: 0 }, dsts: { pins: 0, slots: 0, at: 0, f: 0 } };
  const extra = (b) => (b.at + b.slots < b.pins ? 1 : 0);
  const bandsNow = (m = metricsNow()) => {
    const one = (pins, head, view, was) => {
      const slots = pinsShown({ pins, size: px.xp, head, view });
      const all = document.documentElement.dataset.layout === 'phone' ? slots : pins;
      const at = Math.min(was.at, all - slots);
      return { pins: all, slots, at, f: at === was.at && at < all - slots ? was.f : 0 };
    };
    return { srcs: one(ax.srcPins || 0, px.w, m.cw, band.srcs), dsts: one(ax.dstPins || 0, px.h, m.ch, band.dsts) };
  };
  const sameBands = (b) => ['srcs', 'dsts'].every((k) => b[k].pins === band[k].pins && b[k].slots === band[k].slots);
  const axis = (cols, m = metricsNow()) => ({
    n: (cols ? ax.srcs.length : ax.dsts.length) - (cols ? band.srcs : band.dsts).pins + (cols ? band.srcs : band.dsts).slots,
    pins: (cols ? band.srcs : band.dsts).slots,
    size: px.xp,
    head: cols ? px.w : px.h,
    offset: cols ? m.left : m.top,
    view: cols ? m.cw : m.ch,
    over: OVER,
  });
  const spans = (m = metricsNow()) => ({ rows: axisSpan(axis(false, m)), cols: axisSpan(axis(true, m)) });
  // A client point in scroller px, from its padding edge.
  const local = (x, y, m = metricsNow()) => ({ x: x - m.x, y: y - m.y });

  // A bar or pinned item is sticky, so each draw places the cursor again.
  let curKey = '';
  let ringOn = false;
  const placeCursor = () => {
    const m = metricsNow();
    const ra = axis(false, m);
    const ca = axis(true, m);
    // A pin that its band does not show whole now has no slot, so the cursor hides until the band shows it. A band's
    // pin moves back by the part of a pin the band has moved.
    const ci = slotOf(cur.i, band.dsts);
    const cj = slotOf(cur.j, band.srcs);
    const shift = (k, a, b) => (k !== null && k >= 0 && k < a.pins ? b.f * a.size : 0);
    const x = placeAt(cj ?? 0, ca) - shift(cj, ca, band.srcs);
    const y = placeAt(ci ?? 0, ra) - shift(ci, ra, band.dsts);
    const w = cur.j < 0 ? px.w : px.xp;
    const h = cur.i < 0 ? px.h : px.xp;
    const inBand = ci < ra.pins || cj < ca.pins;
    // Above the bands, a part that scrolls is hidden once it is under them, so it does not cover a name.
    const under = (k, a, at) => k >= a.pins && at + a.size <= a.offset + a.head + a.pins * a.size;
    const hide = ci === null || cj === null || (inBand && (under(ci, ra, y) || under(cj, ca, x)));
    const key = `${x},${y},${w},${h},${inBand},${hide}`;
    if (key === curKey) return;
    curKey = key;
    cursor.style.transform = `translate(${x}px, ${y}px)`;
    cursor.style.width = `${w}px`;
    cursor.style.height = `${h}px`;
    setData(cursor, 'band', inBand ? '' : undefined);
    const bar = cur.i < 0 || cur.j < 0;
    setData(cursor, 'bar', bar ? '' : undefined);
    cursor.hidden = hide;
    // A ring shown again starts its animation again.
    const on = !hide && !bar;
    if (on && !ringOn) joinPulse(cursor.firstChild);
    ringOn = on;
  };

  const reserve = () => {
    const m = metricsNow();
    const r = spanMax(axis(false, m));
    const c = spanMax(axis(true, m));
    // A band that can scroll draws a part pin past its slots at some starts and not at its end, so it is reserved.
    const rp = band.dsts.slots + (band.dsts.pins > band.dsts.slots ? 1 : 0);
    const cp = band.srcs.slots + (band.srcs.pins > band.srcs.slots ? 1 : 0);
    pools.body.reserve(r * c);
    pools.top.reserve(rp * c);
    pools.side.reserve(r * cp);
    pools.corner.reserve(rp * cp);
    pools.srcs.reserve(c);
    pools.dsts.reserve(r);
    pools.srcPinned.reserve(cp);
    pools.dstPinned.reserve(rp);
  };

  const draw = () => {
    frame = read();
    try {
      drawIn();
    } finally {
      frame = null;
    }
  };
  const drawIn = () => {
    const { rows, cols } = spans();
    const rs = range(rows.lo, rows.hi);
    const cs = range(cols.lo, cols.hi);
    const rp = range(band.dsts.at, band.dsts.at + band.dsts.slots + extra(band.dsts));
    const cp = range(band.srcs.at, band.srcs.at + band.srcs.slots + extra(band.srcs));
    pools.body.sync(cross(rs, cs));
    pools.top.sync(cross(rp, cs));
    pools.side.sync(cross(rs, cp));
    pools.corner.sync(cross(rp, cp));
    pools.srcs.sync(cs.map((j) => [0, j]));
    pools.dsts.sync(rs.map((i) => [i, 0]));
    pools.srcPinned.sync(cp.map((j) => [0, j]));
    pools.dstPinned.sync(rp.map((i) => [i, 0]));
    placeCursor();
    for (const f of listeners) f();
  };

  const setAxes = (next) => {
    ax = next;
    srcAt = new Map(ax.srcs.map((n, j) => [n, j]));
    dstAt = new Map(ax.dsts.map((n, i) => [n, i]));
    // The field is in slots: a band's pins past its slots are not in the grid's scroll. The bands are read again once
    // the scroll bars are set for the new size, since the bars take room.
    const size = () => {
      Object.assign(band, bandsNow());
      setVar(field, '--sode-cols', ax.srcs.length - band.srcs.pins + band.srcs.slots);
      setVar(field, '--sode-rows', ax.dsts.length - band.dsts.pins + band.dsts.slots);
    };
    size();
    // The scroll bars for the new size before the pools are sized: a pool never shrinks.
    scrollBars?.now();
    size();
    setVar(field, '--sode-col-pins', band.srcs.slots);
    setVar(field, '--sode-row-pins', band.dsts.slots);
    bandVars();
    pinBar.srcs.set(band.srcs, px);
    pinBar.dsts.set(band.dsts, px);
    for (const p of Object.values(pools)) p.clear();
    reserve();
    draw();
  };

  // The band's start moves the band boxes (bandBox), so it writes no pin. Only a band that scrolls clips its boxes,
  // since a clip also cuts a name's tip.
  function bandVars() {
    const x = -(band.srcs.at + band.srcs.f) * px.xp;
    const y = -(band.dsts.at + band.dsts.f) * px.xp;
    const move = (n, t) => {
      if (n.style.transform !== t) n.style.transform = t;
    };
    move(bands.top, y ? `translateY(${y}px)` : '');
    move(bands.dsts, y ? `translateY(${y}px)` : '');
    move(bands.side, x ? `translateX(${x}px)` : '');
    move(bands.srcs, x ? `translateX(${x}px)` : '');
    move(bands.corner, x || y ? `translate(${x}px, ${y}px)` : '');
    setData(field, 'bandX', band.srcs.pins > band.srcs.slots ? '' : undefined);
    setData(field, 'bandY', band.dsts.pins > band.dsts.slots ? '' : undefined);
  }

  const onScroll = () => draw();
  // Moves a band's start to v (pins, with the part of a pin) and draws it again; mark is false when its bar moved it.
  // Past a whole pin, the sync gives the pin that left its node for the pin that came in (no node is made).
  const setBand = (kind, v, mark = true) => {
    const b = band[kind];
    const to = Math.max(0, Math.min(v, b.pins - b.slots));
    if (to === b.at + b.f) return;
    const at = Math.floor(to);
    b.at = at;
    b.f = to - at;
    if (mark) pinBar[kind].to(to);
    bandVars();
    draw();
  };
  // Moves a band to show pin k, with to (bandFor or bandCentre) picking the start. An item past the pins keeps the
  // band where the wheel put it, with the part of a pin. So does a pin that a reveal finds whole.
  const showPin = (kind, k, to) => {
    const b = band[kind];
    if (k < 0 || k >= b.pins || (to === bandFor && k > b.at && k < b.at + b.slots)) return;
    setBand(kind, to(k, b));
  };
  scroll.addEventListener('scroll', onScroll, { passive: true });
  // A new window or layout size changes how many items show, and a font size change changes px.
  const ro = new ResizeObserver(() => {
    measure();
    curKey = '';
    // A size that changes how many pins fit redraws as new axes would; a scroll never gets here.
    if (!sameBands(bandsNow())) return setAxes(ax);
    bandVars();
    pinBar.srcs.set(band.srcs, px);
    pinBar.dsts.set(band.dsts, px);
    reserve();
    draw();
  });
  ro.observe(scroll);
  ro.observe(probe);
  const xpOf = (xp) => ({ src: Number(xp.dataset.src), dst: Number(xp.dataset.dst) });
  // A press does not focus a crosspoint or a name: a focused button would draw its focus box at the next key. Focus
  // goes to the scroller, so an embedded run still gets its keys.
  field.addEventListener('mousedown', (e) => {
    if (!e.target.closest('.sode-gridpanel__xp, .sode-gridpanel__src, .sode-gridpanel__dst')) return;
    e.preventDefault();
    scroll.focus({ preventScroll: true });
  });
  field.addEventListener('click', (e) => {
    const xp = e.target.closest('.sode-gridpanel__xp');
    if (xp) return onPress(xpOf(xp), e);
    const name = e.target.closest('.sode-gridpanel__src, .sode-gridpanel__dst');
    if (name) onName(name.dataset.src ? 'srcs' : 'dsts', Number(name.dataset.src ?? name.dataset.dst), e);
  });
  // Nodes move between items on a scroll, so the item is read from the node at the click.
  field.addEventListener('contextmenu', (e) => {
    // Android sends a long-press as a menu; on touch a long-press is a line or a range (gridpanelgesture.js).
    if (e.pointerType === 'touch') return e.preventDefault();
    const xp = e.target.closest('.sode-gridpanel__xp');
    const name = !xp && e.target.closest('.sode-gridpanel__src, .sode-gridpanel__dst');
    if (!xp && !name) return;
    e.preventDefault();
    if (xp) onJump?.('xp', xpOf(xp));
    else onJump?.(name.dataset.src ? 'srcs' : 'dsts', Number(name.dataset.src ?? name.dataset.dst));
  });
  // A Shift-click would otherwise extend a text selection on the page.
  field.addEventListener('mousedown', (e) => {
    if (e.shiftKey && !head.contains(e.target)) e.preventDefault();
  });
  // Over the search boxes the cursor stays where Find put it.
  // A finger has no hover: its moves are a scroll or a line, so the cursor stays and the targets show from the picks.
  const onMove = (e) => {
    if (e.pointerType === 'touch') return;
    setData(field, 'touch', undefined);
    if (!head.contains(e.target)) onPoint(e.clientX, e.clientY);
  };
  // A finger hides the cursor until the next mouse move or key move; it never follows a finger.
  scroll.addEventListener('pointerdown', (e) => e.pointerType === 'touch' && setData(field, 'touch', ''), { passive: true });
  scroll.addEventListener('pointermove', onMove, { passive: true });
  // Not passive, so a wheel over the horizontal scroll bar can scroll across instead of down. Chrome makes the rest of
  // a wheel turn async once its first event goes by.
  const onWheel = (e) => {
    const m = metricsNow();
    const box = { left: m.x, top: m.y, right: m.x + m.cw, bottom: m.y + m.ch, w: px.w, h: px.h, size: px.xp, line: px.xp, page: m.ch };
    const b = wheelBand(e, box, band);
    if (b) {
      e.preventDefault();
      pinBar[b.kind].by(b.px);
      return;
    }
    const bar = { left: m.x, right: m.x + m.cw, top: m.y + m.ch, bottom: m.y + scroll.offsetHeight, line: px.xp, page: m.cw };
    const dx = wheelAcross(e, bar);
    if (!dx) return;
    e.preventDefault();
    scroll.scrollLeft += dx;
  };
  scroll.addEventListener('wheel', onWheel, { passive: false });
  measure();
  setAxes(axes);

  const at = (map, n) => map.get(n) ?? -1;
  // The pool slot of item k: a band slot (the part pin too) or a slot past the band, or null for none drawn.
  const slotIn = (k, b) => {
    if (k < 0) return null;
    if (k >= b.pins) return { k: k - b.pins + b.slots, band: false };
    const s = k - b.at;
    return s >= 0 && s < b.slots + extra(b) ? { k, band: true } : null;
  };
  const nodeOf = (src, dst) => {
    const r = slotIn(at(dstAt, dst), band.dsts);
    const c = slotIn(at(srcAt, src), band.srcs);
    if (!r || !c) return null;
    const p = r.band ? (c.band ? pools.corner : pools.top) : c.band ? pools.side : pools.body;
    return p.live.get(`${r.k},${c.k}`) ?? null;
  };
  // The item under p px from the view's start on axis a, -1 on the name bar, or null past the last item. In the band
  // the pin under p counts the part of a pin the band has moved.
  const itemAt = (p, a, b) => {
    const s = slotAt(p, a);
    if (s === null || s < 0 || s >= a.pins) return s === null ? null : itemOf(s, b);
    return b.at + Math.floor((p - a.head) / a.size + b.f);
  };
  return {
    scroll,
    field,
    overlay,
    head,
    get axes() {
      return ax;
    },
    setAxes,
    // Cell px sizes: xp is one crosspoint, w the Destination bar width, h the Source bar height.
    metrics: () => ({ ...px }),
    spans,
    // A client point in crosspoint units (gridpanelline.js), display order. A pinned band does not scroll with the grid.
    unitsAt(x, y) {
      const m = metricsNow();
      const p = local(x, y, m);
      const unit = (v, a, b) => {
        const k = (v - a.head) / a.size;
        return k < a.pins ? k + b.at + b.f : k + a.offset / a.size - b.slots + b.pins;
      };
      return { x: unit(p.x, axis(true, m), band.srcs), y: unit(p.y, axis(false, m), band.dsts) };
    },
    // The scroller's body in client px: past the name bars and the pinned bands, inside the scroll bars.
    bodyBox() {
      const m = metricsNow();
      return { left: m.x + px.w + band.srcs.slots * px.xp, top: m.y + px.h + band.dsts.slots * px.xp, right: m.x + m.cw, bottom: m.y + m.ch };
    },
    // Draws the Drawn line from a to b, in crosspoint units, or hides it for null.
    line(seg) {
      svg.toggleAttribute('hidden', !seg);
      if (!seg) return;
      const set = (k, v) => lineNode.setAttribute(k, String(v));
      // Display units to slot units: a pin by its band's start, the rest past the band.
      const at = (u, b) => (u < b.pins ? u - b.at - b.f : u - b.pins + b.slots);
      set('x1', px.w + at(seg.a.x, band.srcs) * px.xp);
      set('y1', px.h + at(seg.a.y, band.dsts) * px.xp);
      set('x2', px.w + at(seg.b.x, band.srcs) * px.xp);
      set('y2', px.h + at(seg.b.y, band.dsts) * px.xp);
    },
    // The display cell { i, j } under a client point, -1 on a bar, or null past the last item or on a scroll bar.
    cellAt(x, y) {
      const m = metricsNow();
      const p = local(x, y, m);
      if (p.x >= m.cw || p.y >= m.ch) return null;
      const j = itemAt(p.x, axis(true, m), band.srcs);
      const i = itemAt(p.y, axis(false, m), band.dsts);
      return i === null || j === null ? null : { i, j };
    },
    // Places the cursor at display cell { i, j }. Only the cursor node changes.
    // keys is true when a key moved it: only then does the cursor draw its box on a bar.
    cursor(c, keys = false) {
      cur = c;
      setData(cursor, 'keys', keys ? '' : undefined);
      if (keys) setData(field, 'touch', undefined);
      setData(cursor, 'i', String(c.i));
      setData(cursor, 'j', String(c.j));
      placeCursor();
      cueNames();
    },
    // The body's view on each axis: first and last display positions shown (1-based, pins counted), the count, and
    // the scroll offset and its most (gridpanelpos.js).
    shown() {
      const m = metricsNow();
      // Positions count every pin, so they are items: a band's slots, then its pins past them.
      const one = (a, b, max) => {
        const v = a.view - a.head - a.pins * a.size;
        const n = a.n - b.slots + b.pins;
        const first = Math.min(n, b.pins + Math.floor(a.offset / a.size) + 1);
        return { first, last: Math.min(n, b.pins + Math.ceil((a.offset + v) / a.size)), n, offset: a.offset, max };
      };
      return {
        srcs: one(axis(true, m), band.srcs, scroll.scrollWidth - m.cw),
        dsts: one(axis(false, m), band.dsts, scroll.scrollHeight - m.ch),
      };
    },
    // Whole scrolling rows and columns one screen shows.
    page() {
      const m = metricsNow();
      return { rows: pageOf(axis(false, m)), cols: pageOf(axis(true, m)) };
    },
    // Scrolls as little as it can to show display cell { i, j } whole; a bar (-1) keeps that axis where it is.
    // Fresh metrics, since a draw's listener can scroll.
    // A pin its band does not show moves the band instead (gridpanelwindow.js bandFor).
    reveal({ i, j }) {
      if (j >= 0) {
        showPin('srcs', j, bandFor);
        scroll.scrollLeft = scrollFor(slotOf(j, band.srcs), axis(true, read()));
      }
      if (i >= 0) {
        showPin('dsts', i, bandFor);
        scroll.scrollTop = scrollFor(slotOf(i, band.dsts), axis(false, read()));
      }
    },
    // Find: scrolls one axis so display item k of kind ('srcs' or 'dsts') is in the middle, smoothly, then pulses its
    // name.
    glide(kind, k) {
      const cols = kind === 'srcs';
      showPin(kind, k, bandCentre);
      const to = centreFor(slotOf(k, band[kind]), axis(cols, read()));
      goTo({ [cols ? 'left' : 'top']: to }, { srcs: cols ? ax.srcs[k] : null, dsts: cols ? null : ax.dsts[k] });
    },
    // A jump: scrolls so display cell { i, j } is in the middle on both axes, smoothly, then pulses the crosspoint and
    // its two names. A bar (-1) keeps that axis where it is.
    jump({ i, j }) {
      const m = read();
      const to = {};
      if (j >= 0) showPin('srcs', j, bandCentre);
      if (i >= 0) showPin('dsts', i, bandCentre);
      if (j >= 0) to.left = centreFor(slotOf(j, band.srcs), axis(true, m));
      if (i >= 0) to.top = centreFor(slotOf(i, band.dsts), axis(false, m));
      goTo(to, { srcs: j >= 0 ? ax.srcs[j] : null, dsts: i >= 0 ? ax.dsts[i] : null });
    },
    // The crosspoint under a client point, or null on a name, a gap, a scroll bar or outside.
    hit(x, y) {
      const m = metricsNow();
      const p = local(x, y, m);
      if (p.x >= m.cw || p.y >= m.ch) return null;
      const j = itemAt(p.x, axis(true, m), band.srcs) ?? -1;
      const i = itemAt(p.y, axis(false, m), band.dsts) ?? -1;
      return i < 0 || j < 0 ? null : { src: ax.srcs[j], dst: ax.dsts[i], i, j };
    },
    // The drawn crosspoint node of src and dst, or null when it is out of view.
    node: (src, dst) => nodeOf(src, dst),
    // f() runs after each draw: a scroll, a resize or new axes.
    onDraw(f) {
      listeners.add(f);
      return () => listeners.delete(f);
    },
    // view: { names (namesFrom), levels (GET /tally levels), levelText(list), pie(src, dst) -> levels on,
    // staged(src, dst) -> levels staged, picked(kind, n), found(kind, n), leftOut(src, dst), protected(dst) }.
    // view.held(dst) is true when a route to dst is staged.
    // view.other(dst) is { text, tip }, and view.art(kind, n) is { icon, url, mask, b, hide } or null.
    // kind is 'srcs' or 'dsts'. Only drawn nodes are painted.
    paint(v) {
      view = v;
      for (const p of xpPools) for (const n of p.live.values()) paintXp(n, Number(n.dataset.src), Number(n.dataset.dst), v);
      for (const p of [pools.srcs, pools.srcPinned]) for (const n of p.live.values()) paintSrc(n, Number(n.dataset.src));
      for (const p of [pools.dsts, pools.dstPinned]) for (const n of p.live.values()) paintDst(n, Number(n.dataset.dst));
    },
    // The routes a press would stage, numbered, and their names outlined. pairs is [{ src, dst }] in press order, or
    // empty; over marks a press past the cap. skipped is the protected pairs a Drawn line passes over. Only drawn nodes that carry a mark, or get one, are touched.
    preview(pairs, over, skipped = []) {
      mark = {
        xps: new Map(pairs.map(({ src, dst }, k) => [`${src}:${dst}`, k + 1])),
        skip: new Set(skipped.map(({ src, dst }) => `${src}:${dst}`)),
        srcs: pairNumbers(pairs, 'src'),
        dsts: pairNumbers(pairs, 'dst'),
      };
      for (const n of [...marked]) markXp(n, Number(n.dataset.src), Number(n.dataset.dst));
      for (const { src, dst } of [...pairs, ...skipped]) {
        const xp = nodeOf(src, dst);
        if (xp) markXp(xp, src, dst);
      }
      // The drawn names are a few dozen, so each is checked.
      for (const p of [pools.srcs, pools.srcPinned]) for (const n of p.live.values()) markName(n, mark.srcs.get(Number(n.dataset.src)));
      for (const p of [pools.dsts, pools.dstPinned]) for (const n of p.live.values()) markName(n, mark.dsts.get(Number(n.dataset.dst)));
      setData(field, 'over', over && pairs.length ? '' : undefined);
      setData(field, 'marked', mark.srcs.size || mark.dsts.size ? '' : undefined);
    },
    previewing: () => mark.xps.size > 0 || mark.srcs.size > 0 || mark.skip.size > 0,
    stop() {
      scrollBars?.stop();
      pinBar.srcs.stop();
      pinBar.dsts.stop();
      ro.disconnect();
      scroll.removeEventListener('scroll', onScroll);
      scroll.removeEventListener('pointermove', onMove);
      scroll.removeEventListener('wheel', onWheel);
      if (hitEnd) clearTimeout(hitEnd.timer);
      listeners.clear();
    },
  };
}
