// The Assign to modal on the panels page: the open Sode panel pages, and a click sends one to the chosen panel.
// The server checks who may (a Trusted user or the Advanced Settings PIN) and sends the key to the instance alone.
import { el, icon, on } from './core.js';
import { trapDialog } from './dialogkeys.js';
import { segmented } from './formbits.js';
import { assignRows, assignWays, assignChangesMode, rankAssign, curRow, stepRow, enterRow, clashOf, assignNameProblem } from './panelassignrules.js';
import { pinNeeded } from './templaterules.js';
import { MON_POLL_MS } from './panelsmon.js';
import { resultStep } from './keys.js';

let closeOpen = null;
let closeName = null;
let closeAnyway = null;
on('locking', () => {
  closeName?.();
  closeAnyway?.();
  closeOpen?.();
});

// Where the editor's Authentication mode Confirm would ask before an Assign that lands on another mode. Nothing asks
// yet: an Assign never edits a panel, so it goes ahead.
export const confirmModeChange = async () => true;

// The name clash dialog over the Assign to modal: the server's suggestion, editable. go(name) sends again under it;
// Cancel and Escape close only this dialog.
function askName(panel, clashed, suggest, go) {
  const opener = document.activeElement;
  const input = el('input', 'pb-input');
  input.id = 'assign-name';
  input.autocomplete = 'off';
  input.spellcheck = false;
  input.value = suggest;
  input.setAttribute('aria-label', `Instance name on ${panel.name}`);
  const err = el('div', 'pb-note pb-note--error');
  err.setAttribute('role', 'alert');
  err.hidden = true;
  const cancel = el('button', 'pb-btn pb-btn--outline', 'Cancel');
  cancel.type = 'button';
  const ok = el('button', 'pb-btn pb-btn--ok', icon('send'), ' Assign');
  ok.type = 'button';
  ok.id = 'assign-name-go';

  const root = el('div', 'pb-modal-backdrop sode__scrim');
  const dialog = el('div', 'pb-modal');
  dialog.setAttribute('role', 'dialog');
  dialog.setAttribute('aria-modal', 'true');
  dialog.setAttribute('aria-labelledby', 'assign-name-title');
  dialog.tabIndex = -1;
  dialog.append(
    el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Choose another name'), { id: 'assign-name-title' })),
    el(
      'div',
      'pb-modal__body',
      el('p', 'sode__hint', `${panel.name} already has an instance named ${clashed}. The instance takes this name as its device name there.`),
      input,
      err,
    ),
    el('div', 'pb-modal__footer', cancel, ok),
  );
  root.append(dialog);
  document.body.append(root);
  const dlg = trapDialog(root, dialog, close);
  function close() {
    closeName = null;
    dlg.hide();
    root.remove();
    if (opener?.isConnected) opener.focus();
  }
  closeName = close;
  function submit() {
    const problem = assignNameProblem(input.value, clashed);
    err.textContent = problem;
    err.hidden = !problem;
    if (problem) return input.focus();
    close();
    go(input.value.trim());
  }
  cancel.addEventListener('click', close);
  ok.addEventListener('click', submit);
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      submit();
    }
  });
  dlg.show();
  input.focus();
  input.select();
}

// A user who cannot open the target: the device would sign in and find no panel there for them. Answers true for
// Send anyway; Cancel, Escape and the Lock answer false.
function askAnyway(panel, row) {
  return new Promise((done) => {
    const opener = document.activeElement;
    const cancel = el('button', 'pb-btn pb-btn--outline', 'Cancel');
    cancel.type = 'button';
    const ok = el('button', 'pb-btn', icon('send'), ' Send anyway');
    ok.type = 'button';
    ok.id = 'assign-anyway';
    const root = el('div', 'pb-modal-backdrop sode__scrim');
    const dialog = el('div', 'pb-modal');
    dialog.setAttribute('role', 'dialog');
    dialog.setAttribute('aria-modal', 'true');
    dialog.setAttribute('aria-labelledby', 'assign-anyway-title');
    dialog.tabIndex = -1;
    dialog.append(
      el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', `${panel.name} is not shared with ${row.user}`), { id: 'assign-anyway-title' })),
      el(
        'div',
        'pb-modal__body',
        el('p', 'sode__hint', `${row.device} is signed in as ${row.user}, who cannot open ${panel.name}. It will open it and say there is no panel there to use.`),
      ),
      el('div', 'pb-modal__footer', cancel, ok),
    );
    root.append(dialog);
    document.body.append(root);
    const dlg = trapDialog(root, dialog, () => close(false));
    function close(go) {
      closeAnyway = null;
      dlg.hide();
      root.remove();
      if (opener?.isConnected) opener.focus();
      done(go);
    }
    closeAnyway = () => close(false);
    cancel.addEventListener('click', () => close(false));
    ok.addEventListener('click', () => close(true));
    dlg.show();
    cancel.focus();
  });
}

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

  const find = el('input', 'pb-input');
  find.type = 'search';
  find.id = 'assign-find';
  find.placeholder = 'Find an instance';
  find.autocomplete = 'off';
  find.setAttribute('aria-label', 'Find an instance');
  let cur = '';
  let shownRows = [];

  const ways = assignWays(panel);
  let way = ways[0];
  const intro = el('p', 'sode__hint');
  const sayIntro = () =>
    (intro.textContent =
      way === 'user'
        ? `The instance you pick opens ${panel.name} at once and shows its sign-in, where the person there types their own PIN. Its key for the panel it shows now keeps working.`
        : `The instance you pick opens ${panel.name} at once, with a key of its own for it. Its key for the panel it shows now keeps working.`);
  sayIntro();
  const choice =
    ways.length > 1
      ? segmented('assign-way', 'How the instance opens it', [
          ['client', 'Its own key'],
          ['user', 'Sign in'],
        ], (v) => {
          way = v;
          sayIntro();
          paint();
        })
      : null;
  choice?.set(way);

  const list = el('div', 'sode-ov__people');
  list.id = 'assign-list';
  let shown = '';
  // The page re-reads the instances every MON_POLL_MS; only a change is redrawn, so a button being pressed stays.
  function paint(force = true) {
    const all = assignRows(ctx.panels(), panel.id, Date.now(), way === 'user');
    const rows = rankAssign(all, find.value);
    cur = curRow(rows, cur);
    shownRows = rows;
    const now = JSON.stringify([rows, busy, cur]);
    if (!force && now === shown) return;
    shown = now;
    list.replaceChildren();
    if (!rows.length) return list.append(el('p', 'sode__hint', all.length ? 'No instance matches.' : 'No Sode panel page was open in the last 24 hours.'));
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
      if (r.notShared) who.append(el('span', null, el('span', 'pb-badge pb-badge--warn', `Not shared with ${r.user}`)));
      const item = el('div', 'sode-ov__cp', who, b);
      item.dataset.key = r.key;
      if (r.key === cur) item.setAttribute('aria-current', 'true');
      list.append(item);
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
      intro,
      ...(choice ? [choice.node] : []),
      find,
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

  // name is the instance's name on the target, chosen in the clash dialog; '' keeps its own.
  async function send(row, name = '') {
    if (!row.assignable || busy) return;
    const from = ctx.panels().find((p) => p.id === row.panel);
    if (!name && assignChangesMode(from, panel) && !(await confirmModeChange(from, panel))) return;
    if (!name && row.notShared && !(await askAnyway(panel, row))) return;
    if (busy || !root.isConnected) return;
    busy = true;
    paint();
    say('');
    let clash = null;
    try {
      const r = await ctx.api(`api/v1/panels/${panel.id}/assign`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ from: row.panel, instance: row.device, name, pin: pin.value, as: way }),
      });
      pin.value = '';
      say(`Sent ${r.instance} to ${panel.name}.`);
      await ctx.onDone();
    } catch (err) {
      clash = clashOf(err);
      say(clash === null ? err.message : '');
      if (pinNeeded(err.status, err.code)) {
        pinRow.hidden = false;
        pin.focus();
      }
    }
    busy = false;
    if (!root.isConnected) return;
    paint();
    if (clash !== null) askName(panel, name || row.device, clash, (chosen) => send(row, chosen));
  }

  list.addEventListener('click', (e) => {
    const b = e.target.closest('[data-instance]');
    const row = b && shownRows.find((r) => r.panel === b.dataset.from && r.device === b.dataset.instance);
    if (row) send(row);
  });

  find.addEventListener('input', () => paint());
  // The same keys as the panels list's Find: arrows or Ctrl/Cmd+J, K move the highlight, Enter assigns it.
  find.addEventListener('keydown', (e) => {
    const step = resultStep(e, shownRows.length > 0);
    if (step) {
      e.preventDefault();
      cur = stepRow(shownRows, cur, step);
      paint();
      list.querySelector('[aria-current="true"]')?.scrollIntoView({ block: 'nearest' });
    } else if (e.key === 'Enter' && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey) {
      e.preventDefault();
      const row = enterRow(shownRows, cur);
      if (row) send(row);
    }
  });

  paint();
  dlg.show();
  find.focus();
}
