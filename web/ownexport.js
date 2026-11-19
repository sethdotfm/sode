// My user > Export & Import: the user's own profile, presets and filter sets (docs/agents/profiles.md). The other
// categories show greyed as Administrators only. transferdialog.js holds the dialog.

import { el, on } from './core.js';
import * as R from './exportrules.js';
import { transferDialog } from './transferdialog.js';
import { tick, caret, catHead, adminRow, planRow } from './transfertree.js';

function draw({ list, pick, expanded, render, changed }) {
  const rows = [];
  for (const [cat, text] of R.OWN_CATEGORIES) {
    const items = list.categories.find((c) => c.name === cat)?.items || [];
    const state = R.ownState(list, pick, cat);
    const many = cat !== 'profile' && items.length > 0;
    const box = tick(`cat:${cat}`, many && items.length > 1 ? `${text} ${pick.items[cat].size}/${items.length}` : text, {
      checked: state !== 'none',
      mixed: state === 'some',
      disabled: !items.length,
      onChange: (on) => {
        pick.items[cat] = new Set(on ? items.map((i) => i.id) : []);
        changed();
      },
    });
    rows.push(catHead(cat, many && caret(expanded, cat, text, render), box, items.length ? '' : 'None', R.ownSize(list, pick, cat)));
    if (!many || !expanded.has(cat)) continue;
    const row = (it) =>
      planRow({
        cat,
        it: { key: it.id, name: it.name },
        ticked: pick.items[cat].has(it.id),
        onTick: (on) => {
          R.toggle(pick.items[cat], it.id, on);
          changed();
        },
        hint: '',
        size: it.size,
      });
    rows.push(el('div', 'sode__xi-items', ...items.map(row)));
  }
  for (const [cat, text] of R.ADMIN_ONLY) rows.push(adminRow(cat, text));
  return rows;
}

const own = transferDialog({
  id: 'own-xi',
  cls: 'sode__xi-own', // sode__xi stays unique to the Settings > Server dialog
  own: true,
  api: 'api/own/export',
  newPick: R.newOwnPick,
  bodyOf: (_, pick) => {
    const body = R.ownBody(pick);
    return body.categories.length ? body : null;
  },
  draw,
  busyNote: 'Exporting…',
});
on('openOwnExport', own.openDialog);
