// Settings > API: the switch and limits for Sode's API. Saved apart from the Settings form with the
// Advanced Settings PIN (the `apiSettings` message), even for a trusted user. Require HTTPS lives under Proxy.
// Imported by settings.js before it builds its table of contents.

import { $, el, S, on, onMsg, wsSend, sentence } from './core.js';
import { unlocked } from './pin.js';
import { askPin, reask } from './pinprompt.js';
import { apiProblem, apiSettingsReq } from './apirules.js';

const form = $('settings-view');

const check = (id, text, hint) => {
  const input = el('input', 'pb-check__input');
  input.type = 'checkbox';
  input.id = id;
  return { input, node: el('div', 'sode__field', el('label', 'pb-check', input, el('span', 'pb-check__label', text)), el('span', 'sode__hint', hint)) };
};
const num = (id, text, hint, min, max) => {
  const input = el('input', 'pb-input');
  input.type = 'number';
  input.id = id;
  input.min = String(min);
  input.max = String(max);
  return {
    input,
    node: el(
      'div',
      'sode__field',
      Object.assign(el('label', null, text), { htmlFor: id }),
      el('span', 'pb-number-wrap', input),
      el('span', 'sode__hint', hint),
    ),
  };
};

const enabled = check('api-enabled', 'API enabled', 'Off by default. While off, every API call is refused.');
const guest = check('api-guest', 'Guest API', 'Lets a call with no key use the Guest scopes (Manage users, Rights tab, Guest). The API switch gates it too.');
const logReads = check('api-log-reads', 'Log API reads', 'One Communication Log row for every read. Off keeps the log to commands, refusals and key events.');
const rate = num('api-rate', 'Rate limit (requests a minute per key)', '0 means none.', 0, 100000);
const cap = num('api-cap', 'Take cap (API takes in flight)', 'The router answers one message at a time, so a runaway client is held back here. 1-64.', 1, 64);

const err = el('div', 'pb-note pb-note--error');
err.setAttribute('role', 'alert');
err.hidden = true;
const note = el('span', 'sode__hint');
note.setAttribute('role', 'status');
note.setAttribute('aria-live', 'polite');
const save = el('button', 'pb-btn pb-btn--outline', 'Save API settings');
save.type = 'button';
save.id = 'api-save';
save.dataset.find = 'api-save';
save.dataset.label = 'Save API settings';

const group = el(
  'fieldset',
  'sode__group',
  el('legend', null, 'API'),
  enabled.node,
  guest.node,
  logReads.node,
  rate.node,
  cap.node,
  el('div', 'sode__field', save, note),
  err,
);
group.dataset.apart = '';
const usersGroup = [...form.querySelectorAll(':scope > fieldset')].find((f) => f.querySelector('legend')?.textContent === 'Users');
if (usersGroup) usersGroup.after(group);
else form.querySelector('#settings-err')?.parentElement?.before(group) ?? form.append(group);

let saved = { enabled: false, rateLimit: 0, takeCap: 1, guestEnabled: false, logReads: false };
let seq = 0;

function paint() {
  const a = S.cfg?.api;
  if (!a) return;
  saved = { enabled: !!a.enabled, rateLimit: a.rateLimit, takeCap: a.takeCap, guestEnabled: !!a.guestEnabled, logReads: !!a.logReads };
  enabled.input.checked = saved.enabled;
  guest.input.checked = saved.guestEnabled;
  logReads.input.checked = saved.logReads;
  rate.input.value = String(saved.rateLimit);
  cap.input.value = String(saved.takeCap);
}
on('hello', paint);
paint();

const edited = () => ({
  enabled: enabled.input.checked,
  rateLimit: Number(rate.input.value),
  takeCap: Number(cap.input.value),
  guestEnabled: guest.input.checked,
  logReads: logReads.input.checked,
});
const fail = (text) => {
  err.textContent = sentence(text);
  err.hidden = !text;
};

async function doSave() {
  fail('');
  note.textContent = '';
  const next = edited();
  const bad = apiProblem(next);
  if (bad) return fail(bad);
  if (S.pinSet && !unlocked() && !(await askPin('API settings need the Advanced Settings PIN.'))) return;
  const id = ++seq;
  if (!wsSend({ ...apiSettingsReq(saved, next), id })) return fail('Not connected to Sode.');
  save.disabled = true;
}
save.addEventListener('click', doSave);

onMsg('apiSettingsResult', async (m) => {
  if (m.id !== seq) return;
  save.disabled = false;
  if (m.ok) {
    note.textContent = 'Saved.';
    return;
  }
  if (m.locked && (await reask('API settings need the Advanced Settings PIN again.'))) return doSave();
  fail(m.err);
});
