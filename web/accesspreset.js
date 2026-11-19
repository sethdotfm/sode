// Stages the preset as unsaved changes; the one Save sends them (docs/agents/settings-saves.md).

import { $, el, on, emit } from './core.js';
import { trapDialog } from './widgets.js';
import { btn, check, segmented } from './formbits.js';
import { form } from './settingsform.js';
import { markPreset } from './settingssave.js';
import { stageUserFlags } from './users.js';
import { stageKeyless } from './apisettings.js';
import { presetTarget } from './accesspresetrules.js';

const open = $('cfg-apply-preset');
let presets = [];
let users = [];
let me = '';
let pick = '';

const root = el('div', 'pb-modal-backdrop sode__scrim');
root.hidden = true;
const dialog = el('div', 'pb-modal pb-modal--md');
dialog.setAttribute('role', 'dialog');
dialog.setAttribute('aria-modal', 'true');
dialog.setAttribute('aria-labelledby', 'preset-apply-title');
dialog.tabIndex = -1;
const pickBox = el('div', 'sode__field');
const userBox = el('div', 'sode__field', el('span', 'sode__label', 'Set Never sign out and No auto-lock for'));
const rows = el('div', 'sode__preset-users');
const tickAll = btn('pb-btn--outline pb-btn--sm', 'Tick all', () => {
  for (const b of rows.querySelectorAll('input')) b.checked = true;
});
userBox.append(tickAll, rows);
const applyBtn = btn('pb-btn--ok', 'Stage changes', apply);
dialog.append(
  el('div', 'pb-modal__header', Object.assign(el('span', 'pb-modal__title', 'Apply an access preset'), { id: 'preset-apply-title' })),
  el('div', 'pb-modal__body', pickBox, userBox),
  el('div', 'pb-modal__footer', btn('pb-btn--outline', 'Cancel', close), applyBtn),
);
root.append(dialog);
document.body.append(root);
const dlg = trapDialog(root, dialog, close);
let back = null;

function fillPick() {
  const seg = segmented(
    'preset-apply-pick',
    'Access preset',
    presets.map((p) => [p.key, p.name]),
    (v) => {
      pick = v;
      applyBtn.disabled = false;
    },
  );
  pickBox.replaceChildren(el('span', 'sode__label', 'Access preset'), seg.node);
  seg.set(pick);
}

function fillUsers() {
  rows.replaceChildren(
    ...users.map((u) => {
      const c = check(u.name);
      c.input.dataset.uid = u.id;
      if (u.id === me) c.label.append(el('span', 'pb-badge pb-badge--info', 'You'));
      return c.label;
    }),
  );
  userBox.hidden = !users.length;
}

function show() {
  back = document.activeElement;
  pick = '';
  applyBtn.disabled = true;
  fillPick();
  fillUsers();
  dlg.show();
  dialog.focus();
}

function close() {
  dlg.hide();
  if (back?.isConnected) back.focus();
}

const setCheck = (id, v) => ($(id).checked = !!v);

function apply() {
  const p = presets.find((x) => x.key === pick);
  if (!p) return;
  stagePreset(
    p,
    [...rows.querySelectorAll('input:checked')].map((b) => b.dataset.uid),
  );
  close();
}

// ticked: user ids whose flags change.
export function stagePreset(p, ticked) {
  const f = p.fields;
  setCheck('cfg-lock-main', f.locks.main);
  setCheck('cfg-lock-log', f.locks.log);
  setCheck('cfg-lock-panel', f.locks.panel);
  $('cfg-auto-lock').value = String(f.autoLockMinutes);
  $('cfg-guest-role').value = f.guestRole;
  setCheck('cfg-pin-waits', f.pinWaits);
  setCheck('cfg-public-panels', f.publicPanels);
  $('cfg-new-panel-acts').value = f.newPanelActs;
  setCheck('cfg-new-persistent', f.newUserDefaults.persistent);
  setCheck('cfg-new-no-auto-lock', f.newUserDefaults.noAutoLock);
  stageKeyless({ guestEnabled: f.api.guestEnabled, scopes: f.guestApiScopes });
  for (const id of ticked) stageUserFlags(id, p.users);
  form.dispatchEvent(new Event('change'));
  emit('saveSourceChanged');
  markPreset(p.name, presetTarget(p, ticked));
}

open.addEventListener('click', show);
on('usersListed', (m) => {
  presets = m.accessPresets || [];
  users = (m.users || []).map((u) => ({ id: u.id, name: u.name }));
  me = m.me || '';
  open.disabled = !presets.length;
});
// Every field it writes is Critical, so only an Administrator sees it.
on('access', (a) => {
  open.hidden = a.role !== 'administrator';
  if (open.hidden && !root.hidden) close();
});
