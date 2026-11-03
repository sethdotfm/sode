// The Users section of the settings (#151): the user directory, which page
// types lock, and the auto-lock time. Saved on its own (the `users` message,
// guarded by the settings PIN like the rest), never with the main Save.
// Imported by settings.js before it builds its table of contents.

import { $, el, S, on, onMsg, emit, wsSend, sentence } from './core.js';

const form = $('settings-view');
const rows = el('div', 'sode__fields');
const err = el('div', 'pb-note pb-note--error');
err.setAttribute('role', 'alert');
err.hidden = true;
const ok = el('span', 'sode__hint');
ok.setAttribute('role', 'status');

const check = (id, label) => {
  const input = el('input', 'pb-check__input');
  input.type = 'checkbox';
  input.id = id;
  return { input, label: el('label', 'pb-check', input, el('span', 'pb-check__label', label)) };
};
const lockMain = check('usr-lock-main', 'Lock the main screen');
const lockGrid = check('usr-lock-grid', 'Lock Grid pages');
const lockLog = check('usr-lock-log', 'Lock the Communication Log');
const autoInput = el('input', 'pb-input');
autoInput.type = 'number';
autoInput.id = 'usr-auto';
autoInput.min = '0';
autoInput.max = '1440';
const add = el('button', 'pb-btn pb-btn--outline', 'Add user');
add.type = 'button';
const save = el('button', 'pb-btn pb-btn--ok', 'Save users');
save.type = 'button';

const group = el('fieldset', 'sode__group sode__users',
  el('legend', null, 'Users'),
  el('p', 'sode__hint', 'Optional. With users and a lock turned on, a page asks for a PIN before it can change anything; the name goes in history and the log. PINs are 4–12 digits and must differ. Entering the settings PIN (the admin password) on the lock screen lets that browser skip the locks. Changing or removing the settings PIN ends that for every browser.'),
  rows,
  add,
  el('div', 'sode__fields', lockMain.label, lockGrid.label, lockLog.label),
  el('div', 'sode__field',
    Object.assign(el('label', null, 'Lock after idle (minutes)'), { htmlFor: 'usr-auto' }),
    el('span', 'pb-number-wrap', autoInput),
    el('span', 'sode__hint', '0 = never. A browser that skips the locks is not locked.')),
  err,
  el('div', 'sode__names', ok, save));
// Enter in a field here saves the users, not the whole settings form.
group.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && e.target.tagName === 'INPUT') {
    e.preventDefault();
    save.click();
  }
});

// Next to Access, where the PIN is; at the end if that isn't found.
const access = [...form.querySelectorAll(':scope > fieldset')].find((f) => f.querySelector('legend')?.textContent === 'Access');
if (access) access.after(group);
else form.querySelector('#settings-err')?.parentElement?.before(group) ?? form.append(group);

let edited = false;
let seq = 0;
const touched = () => { edited = true; ok.textContent = ''; };

function addRow(u = { name: '', len: 0 }) {
  const name = el('input', 'pb-input');
  name.value = u.name;
  name.maxLength = 24;
  name.spellcheck = false;
  name.autocomplete = 'off';
  name.setAttribute('aria-label', 'User name');
  const pin = el('input', 'pb-input');
  pin.type = 'password';
  pin.inputMode = 'numeric';
  pin.maxLength = 12;
  pin.autocomplete = 'new-password';
  pin.setAttribute('aria-label', 'PIN');
  pin.placeholder = u.len ? 'unchanged' : '4–12 digits';
  const rm = el('button', 'pb-btn pb-btn--ghost pb-btn--sm', 'Remove');
  rm.type = 'button';
  const row = el('div', 'sode__users-row',
    el('div', 'sode__field', el('span', 'sode__label', 'Name'), name),
    el('div', 'sode__field', el('span', 'sode__label', 'PIN'), pin),
    rm);
  row.dataset.was = u.len ? u.name : '';
  rm.addEventListener('click', () => { row.remove(); touched(); });
  for (const f of [name, pin]) f.addEventListener('input', touched);
  rows.append(row);
  return name;
}

function fill() {
  rows.replaceChildren();
  for (const u of S.cfg?.users || []) addRow(u);
  lockMain.input.checked = !!S.cfg?.locks?.main;
  lockGrid.input.checked = !!S.cfg?.locks?.grid;
  lockLog.input.checked = !!S.cfg?.locks?.log;
  autoInput.value = String(S.cfg?.autoLockMinutes || 0);
  edited = false;
  err.hidden = true;
  save.disabled = false;
}

add.addEventListener('click', () => { addRow().focus(); touched(); });
for (const c of [lockMain, lockGrid, lockLog]) c.input.addEventListener('change', touched);
autoInput.addEventListener('input', touched);

function fail(text) {
  err.textContent = sentence(text);
  err.hidden = false;
}

save.addEventListener('click', () => {
  err.hidden = true;
  ok.textContent = '';
  const users = [...rows.children].map((row) => {
    const [name, pin] = row.querySelectorAll('input');
    return { name: name.value.trim(), was: row.dataset.was, pin: pin.value };
  });
  const mins = Number(autoInput.value);
  if (!Number.isInteger(mins) || mins < 0 || mins > 1440) return fail('Lock after idle must be 0 (never) to 1440 minutes.');
  if (users.some((u) => !u.name)) return fail('Every user needs a name.');
  if (users.some((u) => u.pin && !/^[0-9]{4,12}$/.test(u.pin))) return fail('A PIN is 4–12 digits.');
  if (users.some((u) => !u.pin && !u.was)) return fail('Give each new user a PIN.');
  const locks = { main: lockMain.input.checked, grid: lockGrid.input.checked, log: lockLog.input.checked };
  if ((locks.main || locks.grid || locks.log) && !users.length) return fail('Add a user before turning a lock on.');
  const id = ++seq;
  if (!wsSend({ t: 'users', id, usersReq: { users, locks, autoLockMinutes: mins } })) return fail('Not connected to Sode.');
  save.disabled = true;
});

onMsg('usersResult', (m) => {
  save.disabled = false;
  if (m.id !== seq) return;
  if (m.ok) {
    fill(); // the hello before this was skipped as an edit
    ok.textContent = 'Saved.';
    return;
  }
  if (m.locked) emit('locked'); // the settings PIN is wanted again
  fail(m.err);
});

// A snapshot refills the list unless it is being edited here.
on('hello', () => { if (!edited) fill(); });
fill();
