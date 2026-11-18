// Settings > API (index.html #api-group): the switch and limits for Sode's API. Saved apart from the Settings form with the
// Advanced Settings PIN (the `apiSettings` message), even for a trusted user. Require HTTPS lives under Proxy.

import { $, S, on, onMsg, wsSend, sentence } from './core.js';
import { unlocked } from './pin.js';
import { askPin, reask } from './pinprompt.js';
import { apiProblem, apiRepaints, apiSettingsReq } from './apirules.js';

const enabled = $('api-enabled');
const guest = $('api-guest');
const logReads = $('api-log-reads');
const rate = $('api-rate');
const cap = $('api-cap');
const err = $('api-err');
const note = $('api-note');
const save = $('api-save');

let saved = { enabled: false, rateLimit: 0, takeCap: 1, guestEnabled: false, logReads: false };
let sent = saved;
let seq = 0;
let painted = false;

const edited = () => ({
  enabled: enabled.checked,
  rateLimit: Number(rate.value),
  takeCap: Number(cap.value),
  guestEnabled: guest.checked,
  logReads: logReads.checked,
});

function paint() {
  const a = S.cfg?.api;
  if (!a) return;
  const incoming = { enabled: !!a.enabled, rateLimit: a.rateLimit, takeCap: a.takeCap, guestEnabled: !!a.guestEnabled, logReads: !!a.logReads };
  if (painted && !apiRepaints(saved, edited(), incoming)) return;
  painted = true;
  saved = incoming;
  enabled.checked = saved.enabled;
  guest.checked = saved.guestEnabled;
  logReads.checked = saved.logReads;
  rate.value = String(saved.rateLimit);
  cap.value = String(saved.takeCap);
}
on('hello', paint);
paint();

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
  sent = next;
  save.disabled = true;
}
save.addEventListener('click', doSave);

onMsg('apiSettingsResult', async (m) => {
  if (m.id !== seq) return;
  save.disabled = false;
  if (m.ok) {
    saved = sent;
    note.textContent = 'Saved.';
    return;
  }
  if (m.locked && (await reask('API settings need the Advanced Settings PIN again.'))) return doSave();
  fail(m.err);
});
