// Settings > Server > Export & Import: every category (transferdialog.js holds the dialog).
import { $, el } from './core.js';
import { ROLES } from './usersrules.js';
import * as R from './exportrules.js';
import { browserValues, downloadBackup } from './exportcalls.js';
import { transferDialog } from './transferdialog.js';
import { tick, caret, catHead } from './transfertree.js';

const pinsBox = el('input', { class: 'pb-switch__input', type: 'checkbox', id: 'xi-pins', role: 'switch' });
// The legend's own text: its badges (Role, Approval) are not part of the name.
const sectionLabel = (id) => document.querySelector(`fieldset[data-section="${id}"] > legend`)?.firstChild.textContent.trim() || id;
const roleLabel = (v) => ROLES.find((r) => r.value === v)?.label || v;
const browserBytes = () => new TextEncoder().encode(JSON.stringify(browserValues())).length;

function itemRow(key, text, detail, size, checked, onChange, disabled = false) {
  return el(
    'div',
    'sode__xi-item',
    tick(key, text, { checked, disabled, onChange }),
    el('span', 'sode__hint', detail),
    el('span', 'sode__xi-size', R.sizeText(size)),
  );
}

function draw({ list, pick, expanded, render, changed }) {
  pinsBox.checked = pick.pins;
  const by = R.routerBroughtBy(list, pick);
  const icons = R.iconsBroughtBy(list, pick);
  const rows = [];
  for (const [cat, text] of R.CATEGORIES) {
    const c = list.categories.find((x) => x.name === cat) || { items: [], sections: [] };
    const state = R.catState(list, pick, cat);
    const forced = cat === 'routers' && !pick.routers && !!by;
    const parts = cat === 'settings' ? c.sections || [] : c.items || [];
    const empty = cat !== 'browser' && !parts.length;
    const opens = !empty && cat !== 'browser' && cat !== 'routers';
    const count = parts.length > 1 && cat !== 'routers' ? ` ${cat === 'settings' ? pick.sections.size : pick.items[cat].size}/${parts.length}` : '';
    const box = tick(`cat:${cat}`, text + count, {
      checked: state !== 'none' || forced,
      mixed: state === 'some',
      disabled: empty || forced,
      onChange: (on) => {
        R.setCat(list, pick, cat, on);
        changed();
      },
    });
    rows.push(
      catHead(
        cat,
        opens && caret(expanded, cat, text, render),
        box,
        empty ? 'None' : forced ? 'Always included' : '',
        R.catSize(list, pick, cat, browserBytes()),
      ),
    );
    if (forced && by) rows.push(el('div', 'sode__xi-items', el('span', 'sode__hint sode__xi-sub', `Brought by ${by.name}`)));
    if (!opens || !expanded.has(cat)) continue;
    const items = el('div', 'sode__xi-items');
    if (cat === 'settings') {
      for (const s of parts)
        items.append(
          itemRow(`item:${cat}:${s.id}`, sectionLabel(s.id), '', s.size, pick.sections.has(s.id), (on) => {
            R.toggle(pick.sections, s.id, on);
            changed();
          }),
        );
    } else {
      for (const it of parts) {
        const detail = it.detail ? roleLabel(it.detail) : R.KIND_LABEL[it.kind] || '';
        const bringer = pick.items[cat].has(it.id) ? null : icons.get(it.id);
        const row = itemRow(
          `item:${cat}:${it.id}`,
          it.name,
          detail,
          it.size,
          pick.items[cat].has(it.id) || !!bringer,
          (on) => {
            R.toggle(pick.items[cat], it.id, on);
            changed();
          },
          !!bringer,
        );
        if (bringer) row.append(el('span', 'sode__hint sode__xi-sub', `Brought by ${bringer.name}`));
        items.append(row);
      }
    }
    rows.push(items);
  }
  return rows;
}

const xi = transferDialog({
  id: 'xi',
  cls: 'sode__xi',
  api: 'api/export',
  newPick: R.newPick,
  bodyOf: (list, pick) => (R.nothingPicked(list, pick) ? null : R.exportBody(list, pick, browserValues())),
  draw,
  top: [el('div', 'sode__field', el('label', 'pb-check', pinsBox, el('span', 'pb-check__label', 'Include PINs')))],
  busyNote: 'Making the file…',
  backupFile: downloadBackup,
});
pinsBox.addEventListener('change', () => {
  if (!xi.pick) return;
  xi.pick.pins = pinsBox.checked;
  xi.sizeLater();
});
$('xi-open')?.addEventListener('click', xi.openDialog);
