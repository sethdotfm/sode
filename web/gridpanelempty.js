// A Grid panel with no items on an axis has no crosspoints, so the grid says so (pb-empty) under the corner. With no
// crosspoint to route, the deck's controls are disabled, and with no item at all the corner's words go too.
import { el, icon } from './core.js';
import { editHref } from './panelsrules.js';

const say = (n, text) => {
  if (n.textContent !== text) n.textContent = text;
  n.hidden = !text;
};

// paint(g, t, roots): t is emptyText's (gridpanelhead.js), or null to hide the note. roots hold the controls to disable
// while the note shows. free() gives back the controls paint disabled; the run calls it before its own paints, which
// then set each control's state again.
export function gridEmpty(ctx, ui) {
  let empty = null;
  const off = new Set();
  const free = () => {
    for (const n of off) n.disabled = false;
    off.clear();
  };
  const paint = (g, t, roots = []) => {
    if (g) g.head.toggleAttribute('data-empty', !!t?.none);
    if (!t) {
      if (empty) empty.node.hidden = true;
      free();
      return;
    }
    for (const r of roots) {
      for (const n of r?.querySelectorAll('button, input') || []) {
        if (n.disabled) continue;
        n.disabled = true;
        off.add(n);
      }
    }
    if (!empty) {
      const title = el('p', 'pb-empty__title', document.createTextNode(''));
      const msg = el('p', 'pb-empty__msg', document.createTextNode(''));
      const edit = el('a', 'pb-btn pb-btn--outline pb-btn--sm', icon('pencil'), 'Edit panel');
      edit.href = editHref(ctx.id);
      const node = el('div', 'pb-empty sode-gridpanel__empty', title, msg, el('div', 'pb-empty__actions', edit));
      node.setAttribute('role', 'status');
      g.scroll.append(node);
      empty = { node, title, msg, edit };
    }
    say(empty.title, t.title);
    say(empty.msg, t.msg);
    // In the Live Editor (ui.back) the editor is already open, so the note has no link.
    empty.edit.hidden = !t.edit || !!ui.back;
    empty.node.hidden = false;
  };
  return { paint, free };
}
