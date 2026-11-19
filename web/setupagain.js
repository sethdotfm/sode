// Run setup again (Settings > Server, Administrator only; docs/agents/settings-saves.md). The setup steps over Settings,
// prefilled from the form; Finish stages the changed fields and the picked preset, then opens the one Save review.
import { $, on } from './core.js';
import { openAgain, closeAgain } from './setupwizard.js';
import { form, readSettings } from './settingsform.js';
import { saveSources } from './savesources.js';
import { stagePreset } from './accesspreset.js';
import { againAnswers, againStage } from './setupagainrules.js';

const open = $('setup-again-open');
let presets = [];

// The values with unsaved edits, so an edit staged before is not undone by Finish.
function valuesNow() {
  const out = readSettings();
  for (const src of saveSources()) Object.assign(out, src.now());
  return out;
}

// Each write fires the events its own field listens to (the brand preview, the level numbering labels).
function write(key, value) {
  const node = key === 'levelNumbering' ? $(`cfg-lvlnum-${value}`) : form.querySelector(`[data-key="${key}"]`);
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
