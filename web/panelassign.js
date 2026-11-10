// The Assign to modal on the panels page: the open Sode panel pages, and a click sends one to the chosen panel.
// The server checks who may (a Trusted user or the Advanced Settings PIN) and sends the key to the instance alone.
import { el, icon, on } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { assignRows, assignChangesMode } from './panelassignrules.js';
import { pinNeeded } from './templaterules.js';
import { MON_POLL_MS } from './panelsmon.js';

let closeOpen = null;
on('locking', () => closeOpen?.());

// Where the editor's Authentication mode Confirm would ask before an Assign that lands on another mode. Nothing asks
// yet: an Assign never edits a panel, so it goes ahead.
export const confirmModeChange = async () => true;

// ctx: {api, panels() (every panel the page has, with instances where the user may edit them), onDone() (re-reads them)}.
export function openAssign(panel, ctx) {
  const opener = document.activeElement;
  let busy = false;
  const note = el('span', 'sode__hint');
  note.setAttribute('role', 'status');
  const say = (t) => (note.textContent = t);
  const closeBtn = el('button', 'pb-btn pb-btn--outline', 'Close');
  closeBtn.type = 'button';

  const pin = el('input', 'pb-input');
  pin.type = 'password';
  pin.autocomplete = 'off';
  pin.id = 'assign-pin';
  pin.setAttribute('aria-label', 'Advanced Settings PIN');
  const pinRow = el('label', 'sode-tpl__pin', 'Assign needs a Trusted user or the Advanced Settings PIN', pin);
  pinRow.hidden = true;

  const list = el('div', 'sode-ov__people');
  list.id = 'assign-list';
  let shown = '';
  // The page re-reads the instances every MON_POLL_MS; only a change is redrawn, so a button being pressed stays.
  function paint(force = true) {
    const rows = assignRows(ctx.panels(), panel.id, Date.now());
    const now = JSON.stringify([rows, busy]);
    if (!force && now === shown) return;
    shown = now;
    list.replaceChildren();
    if (!rows.length) return list.append(el('p', 'sode__hint', 'No Sode panel page was open in the last 24 hours.'));
    for (const r of rows) {
      const b = el('button', 'pb-btn pb-btn--outline pb-btn--sm', icon('send'), ' Assign');
      b.type = 'button';
      b.dataset.from = r.panel;
      b.dataset.instance = r.device;
      b.disabled = busy || !r.assignable;
      const facts = [r.addr, r.state, r.uptime].filter(Boolean).join(', ');
      const who = el(
        'span',
        `sode-ov__who${r.assignable ? '' : ' sode-ov__muted'}`,
        el('span', null, el('strong', null, r.device), el('span', 'sode__hint', ` on ${r.panelName}`)),
        el('span', 'sode__hint', facts),
      );
      list.append(el('div', 'sode-ov__cp', who, b));
    }
  }

  const root = el('div', 'pb-modal-backdrop sode__scrim');
  const dialog = el('div', 'pb-modal sode-ov__share');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'assign-title');
  dialog.tabIndex = -1;
  dialog.append(
    el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', `Assign to ${panel.name}`), { id: 'assign-title' })),
    el(
      'div',
      'pb-modal__body',
      el(
        'p',
        'sode__hint',
        `The instance you pick opens ${panel.name} at once, with a key of its own for it. Its key for the panel it shows now keeps working.`,
      ),
      list,
      pinRow,
    ),
    el('div', 'pb-modal__footer', note, closeBtn),
  );
  root.append(dialog);
  document.body.append(root);
  const dlg = trapDialog(root, dialog, close);

  const timer = setInterval(() => paint(false), MON_POLL_MS);
  function close() {
    clearInterval(timer);
    closeOpen = null;
    dlg.hide();
    root.remove();
    if (opener?.isConnected) opener.focus();
  }
  closeOpen = close;
  closeBtn.addEventListener('click', close);

  list.addEventListener('click', async (e) => {
    const b = e.target.closest('[data-instance]');
    if (!b || busy) return;
    const from = ctx.panels().find((p) => p.id === b.dataset.from);
    if (assignChangesMode(from, panel) && !(await confirmModeChange(from, panel))) return;
    busy = true;
    paint();
    say('');
    try {
      const r = await ctx.api(`api/v1/panels/${panel.id}/assign`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ from: b.dataset.from, instance: b.dataset.instance, pin: pin.value }),
      });
      pin.value = '';
      say(`Sent ${r.instance} to ${panel.name}.`);
      await ctx.onDone();
    } catch (err) {
      say(err.message);
      if (pinNeeded(err.status, err.code)) {
        pinRow.hidden = false;
        pin.focus();
      }
    }
    busy = false;
    if (root.isConnected) paint();
  });

  paint();
  dlg.show();
  dialog.focus();
}
