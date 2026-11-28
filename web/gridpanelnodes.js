// The Grid panel's nodes. Each is made one time and moves between crosspoints and names as the grid scrolls
// (gridpanelview.js). So a paint changes attributes and Text data, never childList.
import { el, icon } from './core.js';
import { iconHref } from './sprite.js';
import { pieFill } from './gridpanellevels.js';

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
export const pairNumbers = (pairs, key) => {
  const m = new Map();
  pairs.forEach((p, k) => m.set(p[key], m.has(p[key]) ? '' : String(k + 1)));
  return m;
};

// A name node keeps one Text child, so a rename or a move is not a childList change.
export const setText = (n, t) => {
  if (n.firstChild.data !== t) n.firstChild.data = t;
};
export const setData = (n, k, v) => {
  if (v === undefined) {
    if (k in n.dataset) delete n.dataset[k];
  } else if (n.dataset[k] !== v) n.dataset[k] = v;
};
export const pick = (n, on) => {
  setData(n, 'picked', on ? '' : undefined);
  const a = on ? 'true' : 'false';
  if (n.getAttribute('aria-selected') !== a) n.setAttribute('aria-selected', a);
};
export const setVar = (n, k, v) => {
  const s = String(v);
  if (n.style.getPropertyValue(k) !== s) n.style.setProperty(k, s);
};

export function paintXp(xp, s, d, view) {
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
  const ringFill = staged.length ? pieFill(view.levels, staged) : '';
  if (ring._fill !== ringFill) {
    ring._fill = ringFill;
    ring.style.background = ringFill;
  }
  // A picked row or column is washed in its role color (gridpanel.css).
  setData(xp, 'row', view.picked('dsts', d) ? '' : undefined);
  setData(xp, 'col', view.picked('srcs', s) ? '' : undefined);
  // The row and the column of a staged route carry a band in their role colors (gridpanel.css).
  setData(xp, 'stagedRow', view.held(d) ? '' : undefined);
  setData(xp, 'stagedCol', view.heldSrc?.(s) ? '' : undefined);
  const out = staged.length > 0 && view.leftOut(s, d);
  const prot = view.protected(d);
  const st = out ? 'left-out' : staged.length ? 'staged' : undefined;
  const starts = st === 'staged' && xp.dataset.s !== 'staged';
  setData(xp, 's', st);
  if (starts) joinPulse(xp);
  setData(xp, 'on', on.length ? on.join(',') : undefined);
  // A source that tie lines feed: the short Via mark (gridpanel.css) and the full chain in the tip.
  const via = view.via?.(s, d);
  const viaWords = via ? view.viaText(via) : '';
  const viaPart = viaWords && `, ${viaWords}`;
  setData(xp, 'tieVia', via ? '' : undefined);
  setData(xp, 'pbTip', via ? viaWords : undefined);
  const marked = !!view.marks?.xps.has(`${s},${d}`);
  setData(xp, 'mark', marked ? '' : undefined);
  if (xp.getAttribute('aria-disabled') !== String(prot)) xp.setAttribute('aria-disabled', String(prot));
  const state = out ? ', staged, left out of the take' : staged.length ? ', staged. Right-click or Alt-click to unstage' : '';
  const label = `${view.names.source(s)} to ${view.names.destination(d)}${on.length ? `, levels ${view.levelText(on)}` : ''}${viaPart}${prot ? ', protected' : ''}${state}${marked ? view.marks.words.route : ''}`;
  if (xp.getAttribute('aria-label') !== label) xp.setAttribute('aria-label', label);
}

export const makeXp = () => {
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
// Icon A (a sprite icon, an image, or a mask: a library SVG in the text color), then Icon B. All four exist from the
// start, so a scroll never changes childList. The name's last child; CSS order puts it before the text.
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
export const makeSrc = () => {
  const n = makeName('pb-xpt--source sode-gridpanel__src')();
  n.append(makeArt());
  return n;
};
// After the name's Text: the protected mark (its icon never stands alone) and the staged word. Then the current
// sources the grid does not show, then the artwork.
export const makeDst = () => {
  const n = makeName('pb-xpt--dest sode-gridpanel__dst')();
  const prot = el('small', 'sode-gridpanel__prot', icon('shield'), document.createTextNode(''));
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
export function paintArt(row, art) {
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
export function paintDstMarks(n, d, view) {
  const [, prot, held, other] = n.childNodes;
  const p = view.protected(d);
  n.classList.toggle('pb-xpt--protected', p);
  if (prot.hidden === p) prot.hidden = !p;
  // With a hold from the host, the mark names the owner, and its icon and tip say the rest; else a shield and Protected.
  const hold = p ? view.hold?.(d) : null;
  const word = p ? ` ${hold?.text || 'Protected'}` : '';
  if (prot.lastChild.data !== word) prot.lastChild.data = word;
  const href = iconHref(hold?.icon || 'shield');
  const use = prot.firstChild.firstChild;
  if (use.getAttribute('href') !== href) use.setAttribute('href', href);
  setData(prot, 'owner', hold?.owner);
  setData(prot, 'pbTip', hold?.tip);
  // Protected wins: a protected destination has nothing staged. Then a press policy's mark (gridpaneladd.js).
  const marked = !p && !view.held(d) && !!view.marks?.dsts.has(d);
  const heldText = p ? '' : marked ? view.marks.words.name : view.held(d) ? 'Staged' : '';
  const h = !!heldText;
  setText(held, heldText);
  setData(held, 'mark', marked ? '' : undefined);
  if (held.hidden === h) held.hidden = !h;
  const o = view.other(d);
  setText(other, o.text);
  if (other.hidden !== !o.text) other.hidden = !o.text;
  setData(other, 'pbTip', o.tip || undefined);
}

// A pool draws the cells of one layer: a key per cell, nodes reused for cells that come into view.
// put(node, i, j) points a node at the cell of display indices i (row) and j (column).
export function pool(host, make, put) {
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

export const range = (lo, hi) => Array.from({ length: Math.max(0, hi - lo) }, (_, k) => lo + k);
export const cross = (rows, cols) => rows.flatMap((i) => cols.map((j) => [i, j]));

// The grid's fixed DOM in host: the layers, the pinned bands' boxes, the cursor, the Drawn line and the probe.
export function gridFrame(host) {
  host.replaceChildren();
  host.removeAttribute('style');
  host.classList.add('sode-gridpanel');
  const layer = (cls) => el('div', `sode-gridpanel__layer ${cls}`);
  const body = layer('sode-gridpanel__body');
  const overlay = layer('sode-gridpanel__overlay');
  const top = layer('sode-gridpanel__top');
  const side = layer('sode-gridpanel__side');
  const corner = layer('sode-gridpanel__corner');
  // Where the bars meet. The run puts the search boxes (gridpanelfindbox.js) first, so the Destinations word stays next
  // to the Destination bar. The Sources label is turned 180 degrees (gridpanel.css), so arrow-left points along its bar.
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
  return { body, overlay, top, side, corner, head, bands, cursor, svg, lineNode, field, probeXp, probe, scroll, frameNode };
}
