// The one Save review dialog (GLOSSARY.md, Review). Its steps run in this order: the review, then the PIN recheck, then
// the Save and reconnect confirm. Nothing is sent before the last step (spec #543, The one Save).

import { el } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { askRecheck } from './recheck.js';
import { groupRows, saveLabel } from './savereviewrules.js';

let ui = null;
let step = null; // the open run's handlers: { esc }

function build() {
  if (ui) return;
  const title = Object.assign(el('span', 'pb-modal__title'), { id: 'review-title' });
  const body = el('div', 'pb-modal__body');
  const back = el('button', 'pb-btn pb-btn--outline');
  back.type = 'button';
  back.id = 'review-back';
  const go = el('button', 'pb-btn pb-btn--ok');
  go.type = 'button';
  go.id = 'review-save';
  const root = el('div', 'pb-modal-backdrop sode__scrim');
  root.hidden = true;
  const dialog = el('div', 'pb-modal pb-modal--md');
  dialog.id = 'review';
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'review-title');
  dialog.tabIndex = -1;
  dialog.append(el('div', 'pb-modal__header', title), body, el('div', 'pb-modal__footer', back, go));
  root.append(dialog);
  document.body.append(root);
  ui = { title, body, back, go, dlg: trapDialog(root, dialog, () => step?.esc()) };
}

function button(b, text, fn) {
  b.textContent = text;
  b.onclick = fn;
}

function table(groups, { label, show, notes }) {
  const tbody = el('tbody');
  for (const t of groups) {
    const head = el('th', 'sode__review-topic', t.topic);
    head.colSpan = 4;
    tbody.append(el('tr', null, head));
    for (const s of t.sections) {
      const sec = el('th', 'sode__label', s.section);
      sec.colSpan = 4;
      tbody.append(el('tr', null, sec));
      for (const r of s.rows) {
        const row = el(
          'tr',
          null,
          el('td', null, label(r)),
          el('td', 'sode__review-was', show(r, r.was)),
          el('td', null, show(r, r.now)),
          el('td', null, ...notes(r)),
        );
        tbody.append(row);
      }
    }
  }
  const head = el('tr', null, el('th', null, 'Setting'), el('th', null, 'Was'), el('th', null, 'Now'), el('th'));
  return el('table', 'pb-table pb-table--compact', el('thead', null, head), tbody);
}

const badge = (kind, text) => el('span', `pb-badge pb-badge--${kind}`, text);

// rows: the changes (savereviewrules.js diffRows). kept(row): why it stays unsaved, or ''. place(row): {topic, section,
// order}. label(row), show(row, value): text. critical: a row the server checks for a PIN recheck. reconnects(row): a
// router connection field. address: the router address it reconnects to.
// Resolves true when the save is to be sent, false when the person goes back to editing.
export function reviewSave({ rows, kept, place, label, show, critical, reconnects, address, recheckNeeded }) {
  build();
  step?.esc();
  const saves = rows.filter((r) => !kept(r));
  const notes = (r) => [kept(r) && badge('warn', `Stays unsaved: ${kept(r)}`), reconnects(r) && badge('warn', 'Reconnects the router')].filter(Boolean);
  let rechecked = false;
  return new Promise((resolve) => {
    const end = (send) => {
      step = null;
      ui.dlg.hide();
      resolve(send);
    };

    function review() {
      step = { esc: () => end(false) };
      const n = rows.length;
      ui.title.textContent = `Review ${n} ${n === 1 ? 'change' : 'changes'}`;
      ui.body.replaceChildren(table(groupRows(rows, place), { label, show, notes }));
      button(ui.back, 'Keep editing', () => end(false));
      button(ui.go, saveLabel(saves.length, rows.length - saves.length), next);
      ui.go.className = 'pb-btn pb-btn--ok';
      ui.go.disabled = !saves.length;
      ui.dlg.show();
      ui.go.focus();
    }

    async function next() {
      if (!rechecked && recheckNeeded && saves.some(critical)) {
        ui.dlg.hide();
        if (!(await askRecheck('Saving these changes'))) return review();
        rechecked = true;
      }
      if (saves.some(reconnects)) return confirmReconnect();
      end(true);
    }

    function confirmReconnect() {
      step = { esc: review };
      ui.title.textContent = 'Save and reconnect?';
      ui.body.replaceChildren(
        el('p', null, `Saving reconnects Sode to the router${address ? ` at ${address}` : ''}.`),
        el('div', 'pb-note pb-note--warn', 'Takes stop until the router answers. A take that is in progress when Sode reconnects fails.'),
      );
      button(ui.back, 'Back', review);
      button(ui.go, 'Save and reconnect', () => end(true));
      ui.go.className = 'pb-btn pb-btn--danger';
      ui.go.disabled = false;
      ui.dlg.show();
      ui.go.focus();
    }

    review();
  });
}
