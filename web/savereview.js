// Steps: review, PIN recheck, Save and reconnect confirm. Nothing is sent before the last step
// (docs/agents/settings-saves.md).

import { $, el } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { askRecheck } from './recheck.js';
import { groupRows, saveLabel } from './savereviewrules.js';
import { saveStopsLabel, endsSomething, NO_RETURN } from './interruptionrules.js';
import { offersBackup, lastBackupText } from './backuprules.js';
import { downloadBackup, lastBackup } from './exportcalls.js';

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

// by(row): who staged it, or null when every row is the saver's own.
function table(groups, { label, show, notes, by }) {
  const cols = by ? 5 : 4;
  const tbody = el('tbody');
  for (const t of groups) {
    const head = el('th', 'sode__review-topic', t.topic);
    head.colSpan = cols;
    tbody.append(el('tr', null, head));
    for (const s of t.sections) {
      const sec = el('th', 'sode__label', s.section);
      sec.colSpan = cols;
      tbody.append(el('tr', null, sec));
      for (const r of s.rows) {
        const row = el(
          'tr',
          null,
          el('td', null, label(r)),
          ...(by ? [el('td', null, by(r))] : []),
          el('td', 'sode__review-was', show(r, r.was)),
          el('td', null, show(r, r.now)),
          el('td', null, ...notes(r)),
        );
        tbody.append(row);
      }
    }
  }
  const head = el('tr', null, el('th', null, 'Setting'), ...(by ? [el('th', null, 'Staged by')] : []), el('th', null, 'Was'), el('th', null, 'Now'), el('th'));
  return el('table', 'pb-table pb-table--compact', el('thead', null, head), tbody);
}

const badge = (kind, text) => el('span', `pb-badge pb-badge--${kind}`, text);

// list: the server's Interruptions, or null when it did not answer.
function interruptionsBlock(list) {
  if (list && !list.length) return [];
  const head = el('h3', 'sode__label', 'Interruptions');
  if (!list) return [head, el('div', 'pb-note pb-note--warn', 'Interruptions not loaded. Save checks them again.')];
  const out = [head, el('ul', 'sode__review-stops', ...list.map((x) => el('li', null, x.text)))];
  if (endsSomething(list)) out.push(el('div', 'pb-note pb-note--warn', NO_RETURN));
  return out;
}

// The backup offer (backuprules.js). toExport: ends the review, then opens Export & Import.
function backupBlock(toExport) {
  const last = el('span', 'sode__hint', lastBackupText(lastBackup(), Date.now()));
  const status = el('span', 'sode__hint');
  status.id = 'review-backup-note';
  status.setAttribute('role', 'status');
  status.setAttribute('aria-live', 'polite');
  const get = el('button', 'pb-btn pb-btn--outline pb-btn--sm', 'Download a backup');
  get.type = 'button';
  get.id = 'review-backup';
  get.onclick = async () => {
    get.disabled = true;
    status.textContent = 'Making the file…';
    status.classList.remove('sode__error-text');
    try {
      const got = await downloadBackup();
      status.textContent = `Saved ${got.name}.`;
      last.textContent = lastBackupText(lastBackup(), Date.now());
    } catch (e) {
      status.textContent = e.message;
      status.classList.add('sode__error-text');
    } finally {
      get.disabled = false;
    }
  };
  const link = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Export & Import');
  link.type = 'button';
  link.id = 'review-export';
  link.onclick = toExport;
  return [el('h3', 'sode__label', 'Backup'), el('div', 'sode__review-backup', get, link, last), status];
}

// unseen: counts the Role cannot see (stagedrules.js), not in rows. interruptions: null when the server did not
// answer. Resolves false when the person goes back.
export function reviewSave({
  rows,
  kept,
  place,
  label,
  show,
  critical,
  reconnects,
  address,
  recheckNeeded,
  by = null,
  unseen = [],
  interruptions = [],
  criticalAction = critical,
  role = '',
  preset = () => '',
}) {
  build();
  step?.esc();
  const saves = rows.filter((r) => !kept(r));
  const notes = (r) =>
    [
      preset(r) && badge('info', preset(r)),
      kept(r) && badge('warn', `Stays unsaved: ${kept(r)}`),
      reconnects(r) && badge('warn', 'Reconnects the router'),
    ].filter(Boolean);
  let rechecked = false;
  return new Promise((resolve) => {
    const end = (send) => {
      step = null;
      ui.dlg.hide();
      resolve(send);
    };

    function review() {
      step = { esc: () => end(false) };
      const n = rows.length + unseen.length;
      ui.title.textContent = `Review ${n} ${n === 1 ? 'change' : 'changes'}`;
      const counts = unseen.map((t) => el('div', 'pb-note pb-note--warn', `${t}. They stay unsaved.`));
      ui.body.replaceChildren(
        ...counts,
        ...(rows.length ? [table(groupRows(rows, place), { label, show, notes, by })] : []),
        ...interruptionsBlock(interruptions),
        ...(offersBackup({ saves, isCritical: criticalAction, interruptions, role })
          ? backupBlock(() => {
              end(false);
              $('xi-open')?.click();
            })
          : []),
      );
      button(ui.back, 'Keep editing', () => end(false));
      const keep = rows.length - saves.length;
      button(ui.go, saveStopsLabel(saves.length, keep, interruptions, saveLabel(saves.length, keep)), next);
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
      ui.title.textContent = `Save and reconnect${address ? ` to ${address}` : ''}?`;
      ui.body.replaceChildren(el('div', 'pb-note pb-note--warn', 'Takes stop until the router answers. A take in progress fails.'));
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
