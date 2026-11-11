// The Cell tab's Page picker for a Go to Page cell: the panel's Pages by number and name, and a warning when the
// target was deleted, is disabled or has no cells. Rules are targetChoices and targetWarning (panelnav.js).
import { el } from './core.js';
import { opt } from './formbits.js';
import { targetChoices, targetWarning } from './panelnav.js';

// patch(fields) changes the selected cell.
export function createPageTarget({ ed, patch }) {
  const select = el('select', 'pb-select');
  select.id = 'ed-cell-page';
  const label = Object.assign(el('label', null, 'Page'), { htmlFor: 'ed-cell-page' });
  const warn = el('div', 'pb-note pb-note--warn');
  warn.id = 'ed-cell-page-warn';
  const node = el('div', 'sode__field', label, el('span', 'pb-select-wrap', select), warn);
  select.addEventListener('change', () => patch({ page: select.value || undefined }));

  return {
    node,
    update(c) {
      node.hidden = c.kind !== 'goToPage';
      if (node.hidden) return;
      const pages = ed.pages();
      const choices = targetChoices(pages);
      const known = choices.some((x) => x.value && x.value === c.page);
      // A deleted target stays chosen under its own line, so the cell is not changed by just looking at it.
      const lost = c.page && !known ? [{ value: c.page, text: 'A deleted Page', disabled: true }] : [];
      select.replaceChildren(opt('', 'Choose a Page'), ...[...lost, ...choices].map((x) => Object.assign(opt(x.value, x.text), { disabled: x.disabled })));
      select.value = c.page || '';
      select.disabled = ed.readOnly();
      const text = targetWarning(pages, c, ed.storedGrid().cells);
      warn.textContent = text;
      warn.hidden = !text;
    },
  };
}
