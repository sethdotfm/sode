// The fill order picker in Find's flyout: one small icon button per order in gridmodel's FILL_ORDERS. Each icon
// is Patchbay's arrow for the direction the fill starts in; the two snakes use the arrow that turns back.
import { el, icon } from './core.js';
import { FILL_ORDERS } from './gridmodel.js';

const ORDER_ICON = {
  across: 'arrow-right',
  'across-rev': 'arrow-left',
  'snake-across': 'arrow-right-left',
  down: 'arrow-down',
  'down-rev': 'arrow-up',
  'snake-down': 'arrow-down-up',
};

// onPick(id) runs on a choice. The returned element has set(id) to show the current order.
export function createOrderPicker(current, onPick) {
  const box = el('div', 'sode-find__orders');
  box.setAttribute('role', 'group');
  box.setAttribute('aria-label', 'Fill order');
  const buttons = FILL_ORDERS.map((o) => {
    const b = el('button', 'pb-chip', icon(ORDER_ICON[o.id]));
    b.type = 'button';
    b.dataset.order = o.id;
    b.setAttribute('aria-label', o.name);
    b.dataset.pbTip = o.name;
    b.addEventListener('click', () => onPick(o.id));
    return b;
  });
  const set = (id) => buttons.forEach((b) => b.setAttribute('aria-pressed', String(b.dataset.order === id)));
  set(current);
  box.append(...buttons);
  box.set = set;
  return box;
}
