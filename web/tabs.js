// A tab strip on patchbay's pb-tabs, with arrow, Home and End keys (stepTab in rightsrules.js).
// items: [{id, text, panel}]. The panels are the caller's nodes; this shows one and hides the rest.

import { el } from './core.js';
import { stepTab } from './rightsrules.js';

export function makeTabs(items, onChange) {
  const bar = el('div', 'pb-tabs sode__tabs');
  bar.setAttribute('role', 'tablist');
  const tabs = new Map();
  let cur = items[0].id;
  function select(id, focus = false) {
    cur = id;
    for (const it of items) {
      const on = it.id === id;
      const t = tabs.get(it.id);
      t.setAttribute('aria-selected', String(on));
      t.tabIndex = on ? 0 : -1;
      it.panel.classList.toggle('active', on);
      it.panel.hidden = !on;
    }
    if (focus) tabs.get(id).focus();
    onChange?.(id);
  }
  for (const it of items) {
    const t = el('button', 'pb-tab', it.text);
    t.type = 'button';
    t.id = `tab-${it.id}`;
    t.setAttribute('role', 'tab');
    t.setAttribute('aria-controls', `panel-${it.id}`);
    it.panel.id = `panel-${it.id}`;
    it.panel.setAttribute('role', 'tabpanel');
    it.panel.setAttribute('aria-labelledby', t.id);
    it.panel.classList.add('pb-tabpanel');
    t.addEventListener('click', () => select(it.id));
    tabs.set(it.id, t);
    bar.append(t);
  }
  bar.addEventListener('keydown', (e) => {
    const next = stepTab(
      items.map((i) => i.id),
      cur,
      e.key,
    );
    if (!next) return;
    e.preventDefault();
    select(next, true);
  });
  select(cur);
  return { node: bar, select, current: () => cur };
}
