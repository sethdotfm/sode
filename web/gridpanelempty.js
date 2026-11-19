// A Grid panel with no items on an axis has no crosspoints, so the grid says so (pb-empty) under the corner.
import { el, icon } from './core.js';
import { editHref } from './panelsrules.js';

const say = (n, text) => {
  if (n.textContent !== text) n.textContent = text;
  n.hidden = !text;
};

// Returns paint(g, t): t is emptyText's (gridpanelhead.js), or null to hide the note.
export function gridEmpty(ctx, ui) {
  let empty = null;
  return (g, t) => {
    if (!t) {
      if (empty) empty.node.hidden = true;
      return;
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
}
