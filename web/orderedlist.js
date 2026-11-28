// An ordered list's rows: drag to reorder, Move up, Move down, Remove, and the caller's own actions before them. Rules
// are orderedlistrules.js. The caller draws each row's body and holds the list; the editor's Pages bar is one user.
import { el, icon } from './core.js';
import { btn } from './formbits.js';
import { dropTo, canMove, edgeStep } from './orderedlistrules.js';

// opts: rowClass, toolsClass, move(from, to), remove(i), removeTip.
// Opt-in: bareFixed leaves Move and Remove out of a fixed row. Each has an empty place, so the caller's actions keep
// their place under a bar that ends in the same buttons.
// Opt-in: drop(from, over, after) places a drop by the pointer's half.
// Opt-in: grip(i) limits the drag to the grip and the body, and gives the drag chip's text.
// Opt-in: dragging(i | -1).
// Buttons carry data-act and data-i so a caller can put focus back.
const DRAG_AT = 12; // px from the chip's top left corner to the pointer
// Marks a drag as a row of this page's. A windowed list can drop the row that a drag started from. Then its dragend
// does not come, and nor does a drop outside the rows. A later drag without the mark (a file) must not move a row.
const ROW_TYPE = 'application/x-sode-row';
// Drag scroll: the edge band in px, the most px a frame, and how long the scroll goes on after the last dragover. A
// pointer at rest still gets a dragover about every 50 ms, so a longer gap means the drag went away.
const EDGE_PX = 64;
const STEP_PX = 24;
const STALE_MS = 400;

export function createOrderedList(opts) {
  let dragFrom = -1;
  const ours = (e) => {
    if (e.dataTransfer?.types.includes(ROW_TYPE)) return dragFrom >= 0;
    if (dragFrom >= 0) {
      dragFrom = -1;
      opts.dragging?.(-1);
    }
    return false;
  };

  const iconBtn = (name, tip, act, fn) => {
    const b = btn('pb-btn--ghost pb-btn--icon pb-btn--sm', '', fn);
    b.append(icon(name));
    b.setAttribute('aria-label', tip);
    b.dataset.pbTip = tip;
    b.dataset.act = act;
    return b;
  };

  // r: i, n (rows in the list), id, readOnly, body (nodes above the buttons), actions ([{ icon, tip, act, run, off }]).
  // removeOff keeps Remove off, for example on a row the list must keep; upOff and downOff keep a Move off. fixed
  // turns off Move, Remove and the drag but leaves the caller's actions on, with fixed as the tip of each one off.
  function row(r) {
    const { i, n, readOnly, fixed } = r;
    const can = canMove(i, n);
    const own = (r.actions || []).map((a) => [iconBtn(a.icon, a.tip, a.act, () => a.run(i)), a.off]);
    const tools = el(
      'div',
      opts.toolsClass,
      ...[
        ...own,
        [iconBtn('arrow-up', 'Move up', 'up', () => opts.move(i, i - 1)), !can.up || r.upOff, true],
        [iconBtn('arrow-down', 'Move down', 'down', () => opts.move(i, i + 1)), !can.down || r.downOff, true],
        [iconBtn('trash-2', opts.removeTip, 'delete', () => opts.remove(i)), r.removeOff, true],
      ].map(([b, off, ours]) => {
        if (ours && fixed && opts.bareFixed) {
          const blank = el('span', 'sode-pages__blank');
          blank.setAttribute('aria-hidden', 'true');
          return blank;
        }
        b.dataset.i = String(i);
        b.disabled = !!(readOnly || off || (ours && fixed));
        if (ours && fixed) b.dataset.pbTip = fixed;
        return b;
      }),
    );
    const grip = opts.grip ? el('span', 'sode-grip', icon('grip-vertical')) : null;
    if (grip) {
      grip.setAttribute('aria-hidden', 'true');
      // From the start, since the grip sits at the list's left edge and a centred tip would leave the window.
      grip.classList.add('pb-tip--start');
      grip.dataset.pbTip = fixed || 'Drag to move';
    }
    const li = el('li', opts.rowClass, ...(grip ? [grip] : []), ...(r.body || []), tools);
    li.dataset.id = r.id;
    if (!readOnly && !fixed) dragRow(li, i, grip && [grip, ...(r.body || [])]);
    else if (grip) grip.classList.add(fixed && opts.bareFixed ? 'is-gone' : 'is-off');
    return li;
  }

  const sideOf = (li, e) => {
    const r = li.getBoundingClientRect();
    return e.clientY > r.top + r.height / 2;
  };
  const unmark = (li) => li.classList.remove('is-drop', 'is-drop-after', 'is-drop-before');
  // handles: the nodes a drag starts from in grip mode, or null for the whole row.
  function dragRow(li, i, handles) {
    for (const from of handles || [li]) {
      from.draggable = true;
      from.addEventListener('dragstart', (e) => {
        dragFrom = i;
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', String(i + 1));
        e.dataTransfer.setData(ROW_TYPE, '');
        if (handles) dragImage(e, opts.grip(i));
        opts.dragging?.(i);
      });
      // A row a scroll redraws is a new node, so the drop, not the old row's dragend, can be the last event.
      from.addEventListener('dragend', () => {
        dragFrom = -1;
        opts.dragging?.(-1);
      });
    }
    li.addEventListener('dragover', (e) => {
      if (!ours(e)) return;
      e.preventDefault();
      if (!opts.drop) return li.classList.add('is-drop');
      const after = sideOf(li, e);
      li.classList.toggle('is-drop-after', after);
      li.classList.toggle('is-drop-before', !after);
    });
    li.addEventListener('dragleave', () => unmark(li));
    li.addEventListener('drop', (e) => {
      if (!ours(e)) return;
      e.preventDefault();
      unmark(li);
      const from = dragFrom;
      dragFrom = -1;
      if (from < 0) return;
      opts.dragging?.(-1);
      if (opts.drop) opts.drop(from, i, sideOf(li, e));
      else opts.move(from, dropTo(from, i));
    });
  }

  // The browser draws the drag image from a node in the page at dragstart, so the chip shows for that moment only.
  // Chrome cuts the image at the window's edge, so the chip is drawn in the window, under the pointer, where the
  // image goes anyway.
  function dragImage(e, text) {
    const chip = el('span', 'pb-chip sode-drag-chip', icon('grip-vertical'), text);
    const x = Math.max(0, e.clientX - DRAG_AT);
    const y = Math.max(0, e.clientY - DRAG_AT);
    chip.style.left = `${x}px`;
    chip.style.top = `${y}px`;
    document.body.append(chip);
    e.dataTransfer.setDragImage(chip, e.clientX - x, e.clientY - y);
    setTimeout(() => chip.remove());
  }

  // While a row drag is near the scroller's top or bottom edge, the scroller scrolls, faster nearer the edge. It moves
  // the real scroller, so the big scroll bar (bigscroll.js) and a windowed list's rows follow its own scroll events.
  function scrollWhileDragging(scroller) {
    let step = 0;
    let last = 0;
    let frame = 0;
    const stop = () => {
      step = 0;
      cancelAnimationFrame(frame);
      frame = 0;
    };
    const tick = () => {
      frame = 0;
      if (!step || dragFrom < 0 || performance.now() - last > STALE_MS) return stop();
      scroller.scrollTop += step;
      frame = requestAnimationFrame(tick);
    };
    scroller.addEventListener('dragover', (e) => {
      if (!ours(e)) return stop();
      const r = scroller.getBoundingClientRect();
      step = edgeStep(e.clientY, r.top, r.bottom, EDGE_PX, STEP_PX);
      last = performance.now();
      if (step && !frame) frame = requestAnimationFrame(tick);
    });
    scroller.addEventListener('dragleave', (e) => {
      if (!scroller.contains(e.relatedTarget)) stop();
    });
    // The row a drag started from can be gone (redrawn), so its dragend does not reach the scroller; the tick stops on
    // dragFrom then.
    scroller.addEventListener('drop', stop);
    scroller.addEventListener('dragend', stop);
  }

  return { row, scrollWhileDragging };
}
