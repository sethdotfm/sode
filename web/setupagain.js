// Run setup again (Settings > Server, Administrator only; docs/agents/settings-saves.md). The setup steps over Settings,
// prefilled from the form; Finish stages the changed fields and the picked preset, then opens the one Save review.
import { $, S, on } from './core.js';
import { openAgain, closeAgain } from './setupwizard.js';
import { form, readSettings } from './settingsform.js';
import { saveSources } from './savesources.js';
import { stagePreset } from './accesspreset.js';
import { againAnswers, againStage } from './setupagainrules.js';
import { stageFirstRouter } from './settingsrouterdetail.js';

// The setup steps name the first router's settings by these keys; the one Save names them router/<uid>/<attribute>.
const FIRST = {
  routerTitle: 'title',
  router: 'address',
  levels: 'levels',
  levelNumbering: 'levelNumbering',
  numberOffset: 'numberOffset',
  fireMode: 'fireMode',
  salvoGroup: 'salvoGroup',
};

const open = $('setup-again-open');
let presets = [];

// Finish must not undo an edit staged before.
function valuesNow() {
  const out = readSettings();
  for (const src of saveSources()) Object.assign(out, src.now());
  const uid = S.routers[0]?.uid;
  for (const [k, attr] of Object.entries(FIRST)) out[k] = out[`router/${uid}/${attr}`];
  return out;
}

// Each write fires the events its own field listens to (the brand preview, the level numbering labels).
function write(key, value) {
  if (key in FIRST) return stageFirstRouter(FIRST[key], value);
  const node = form.querySelector(`[data-key="${key}"]`);
  if (!node) return;
  if (node.type === 'radio') node.checked = true;
  else node.value = String(value);
  for (const ev of ['input', 'change']) node.dispatchEvent(new Event(ev, { bubbles: true }));
}

function finish(d) {
  const { fields, preset } = againStage(d, valuesNow(), presets);
  for (const [k, v] of Object.entries(fields)) write(k, v);
  // No user is ticked: Run setup again changes no user.
  if (preset) stagePreset(preset, []);
  form.requestSubmit();
  open.focus();
}

open.addEventListener('click', () => openAgain(againAnswers(valuesNow(), presets), presets, { finish, cancel: () => open.focus() }));

on('usersListed', (m) => {
  presets = m.accessPresets || [];
  open.disabled = !presets.length;
});
// It writes Critical fields, so only an Administrator runs it.
on('access', (a) => {
  if (a.role !== 'administrator') closeAgain(false);
});
