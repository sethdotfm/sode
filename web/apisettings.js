// Settings > API (index.html #api-group): the keyless API, its scopes and the limits. The API itself is always on.
// Saved apart from the Settings form (the `apiSettings` message). Require HTTPS lives under Proxy.

import { $, S, on, onMsg, wsSend, sentence, el } from './core.js';
import { unlocked } from './pin.js';
import { askPin, reask } from './pinprompt.js';
import { askRecheck } from './recheck.js';
import { apiProblem, apiRepaints, apiSettingsReq, SCOPES } from './apirules.js';

const guest = $('api-guest');
const scopeBox = $('api-scopes');
const logReads = $('api-log-reads');
const rate = $('api-rate');
const cap = $('api-cap');
const err = $('api-err');
const note = $('api-note');
const save = $('api-save');

for (const s of SCOPES) {
  const box = el('input', 'pb-check__input');
  box.type = 'checkbox';
  box.dataset.scope = s.id;
  scopeBox.append(el('label', 'pb-check', box, el('span', 'pb-check__label', s.text)));
}
const boxes = [...scopeBox.querySelectorAll('input[data-scope]')];
// Disabled until a usersList answer says what the scopes are, so a tick cannot replace a set the page never saw.
for (const b of boxes) b.disabled = true;

let saved = { rateLimit: 0, takeCap: 1, guestEnabled: false, logReads: false, guestScopes: [] };
let sent = saved;
let seq = 0;
let painted = false;

const edited = () => ({
  rateLimit: Number(rate.value),
  takeCap: Number(cap.value),
  guestEnabled: guest.checked,
  logReads: logReads.checked,
  guestScopes: boxes.filter((b) => b.checked).map((b) => b.dataset.scope),
});

const fill = () => {
  guest.checked = saved.guestEnabled;
  logReads.checked = saved.logReads;
  rate.value = String(saved.rateLimit);
  cap.value = String(saved.takeCap);
  for (const b of boxes) b.checked = saved.guestScopes.includes(b.dataset.scope);
};

// The hello has no keyless scopes (they ride the Administrator's usersList), so they come from saved.
function paint() {
  const a = S.cfg?.api;
  if (!a) return;
  const incoming = { rateLimit: a.rateLimit, takeCap: a.takeCap, guestEnabled: !!a.guestEnabled, logReads: !!a.logReads, guestScopes: saved.guestScopes };
  if (painted && !apiRepaints(saved, edited(), incoming)) return;
  painted = true;
  saved = incoming;
  fill();
}
on('hello', paint);
paint();

// users.js asks for the usersList after a sign-in and on Manage users, and passes each answer on.
on('usersListed', (m) => {
  const incoming = { ...saved, guestScopes: m.guestScopes || [] };
  const first = boxes[0]?.disabled;
  for (const b of boxes) b.disabled = false;
  if (!first && !apiRepaints(saved, edited(), incoming)) return;
  saved = incoming;
  fill();
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
  if (m.recheck && (await askRecheck('Saving the keyless API'))) return doSave();
  if (m.locked && (await reask('API settings need the Advanced Settings PIN again.'))) return doSave();
  fail(m.err);
});
