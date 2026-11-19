import { $, S, on, el } from './core.js';
import { addSaveSource } from './savesources.js';
import { diffRows, showValue } from './savereviewrules.js';
import { apiProblem, SCOPES } from './apirules.js';

const guest = $('api-guest');
const scopeBox = $('api-scopes');
const logReads = $('api-log-reads');
const rate = $('api-rate');
const cap = $('api-cap');

for (const s of SCOPES) {
  const box = el('input', 'pb-check__input');
  box.type = 'checkbox';
  box.dataset.scope = s.id;
  scopeBox.append(el('label', 'pb-check', box, el('span', 'pb-check__label', s.text)));
}
const boxes = [...scopeBox.querySelectorAll('input[data-scope]')];
// Disabled until a usersList answer says what the scopes are, so a tick cannot replace a set the page never saw.
for (const b of boxes) b.disabled = true;

// The page's scope order, so a set the server keeps in another order is no change.
const ordered = (list) => SCOPES.map((s) => s.id).filter((id) => list.includes(id));

let base = {}; // api, and guestApiScopes after a usersList gives them
let vers = {};
let scopes = null; // the last usersList's {list, version}

const apiNow = () => ({ rateLimit: Number(rate.value), takeCap: Number(cap.value), guestEnabled: guest.checked, logReads: logReads.checked });

function now() {
  if (!('api' in base)) return {};
  const out = { api: apiNow() };
  if ('guestApiScopes' in base) out.guestApiScopes = boxes.filter((b) => b.checked).map((b) => b.dataset.scope);
  return out;
}
const edited = (key) => diffRows({ [key]: base[key] }, { [key]: now()[key] }).length > 0;

function fillApi() {
  const a = S.cfg?.api;
  if (!a) return;
  base.api = { rateLimit: a.rateLimit, takeCap: a.takeCap, guestEnabled: !!a.guestEnabled, logReads: !!a.logReads };
  vers.api = S.versions?.api;
  guest.checked = base.api.guestEnabled;
  logReads.checked = base.api.logReads;
  rate.value = String(base.api.rateLimit);
  cap.value = String(base.api.takeCap);
}

function fillScopes() {
  if (!scopes) return;
  base.guestApiScopes = ordered(scopes.list);
  vers.guestApiScopes = scopes.version;
  for (const b of boxes) b.checked = base.guestApiScopes.includes(b.dataset.scope);
}

// A hello or list moves only a baseline with no edits on it, so another person's save shows as a conflict.
on('hello', () => {
  if (!('api' in base) || !edited('api')) fillApi();
});
fillApi();

on('usersListed', (m) => {
  scopes = { list: m.guestScopes || [], version: m.versions?.guestApiScopes };
  const first = boxes[0]?.disabled;
  for (const b of boxes) b.disabled = false;
  if (first || !edited('guestApiScopes')) fillScopes();
});

const LABELS = { rateLimit: 'api-rate', takeCap: 'api-cap', guestEnabled: 'api-guest', logReads: 'api-log-reads' };
function label(r) {
  if (r.key === 'guestApiScopes') return $('api-scopes-label').textContent;
  const f = $(LABELS[r.sub]);
  return (f.closest('.pb-check')?.querySelector('.pb-check__label') || document.querySelector(`label[for="${f.id}"]`)).textContent;
}

function show(r, v) {
  if (r.key === 'guestApiScopes') return v.length ? v.map((id) => SCOPES.find((s) => s.id === id)?.text || id).join(', ') : 'None';
  return showValue(v);
}

// Callers emit the change.
export function stageKeyless({ guestEnabled, scopes }) {
  if (guestEnabled !== undefined) guest.checked = guestEnabled;
  if (scopes) for (const b of boxes) b.checked = scopes.includes(b.dataset.scope);
}

addSaveSource({
  owns: (key) => key === 'api' || key === 'guestApiScopes',
  base: () => base,
  now,
  version: (key) => vers[key],
  label,
  show,
  problem: () => ('api' in base ? apiProblem(apiNow()) : ''),
  saved(key, value, r) {
    base[key] = value;
    if (r.version) vers[key] = r.version;
  },
  conflict(key, r) {
    vers[key] = r.version;
    base[key] = key === 'guestApiScopes' ? ordered(r.value || []) : r.value;
  },
  reset() {
    base = {};
    fillApi();
    fillScopes();
  },
});
