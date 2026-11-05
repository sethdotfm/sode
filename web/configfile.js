// Server configuration file (Settings > Access): Export… saves the live
// config.yaml, Import… replaces the whole configuration with a picked file.
// The block is in index.html; this wires it. Both are settings-PIN actions
// (the server refuses anything else, and then demandPin() brings the PIN
// prompt back). The server makes and reads the file; this only moves it.
// Imported by settings.js.

import { $, S, on, onMsg, wsSend, sentence } from './core.js';
import { demandPin, needsPin } from './pin.js';
import { showResult } from './plans.js';
import { fileName, fileProblem, sizeText } from './configfilerules.js';

const box = $('config-file');
const pins = $('config-pins');
const exportBtn = $('config-export');
const importBtn = $('config-import');
const pick = $('config-pick');
const confirmBox = $('config-confirm');
const confirmText = $('config-confirm-text');
const err = $('config-err');
const status = $('config-status');

let seq = 0;
let busy = null; // { id, kind: 'export' | 'import' }
let picked = null; // { name, text, bytes } waiting for the confirm

const say = (text) => { status.textContent = text; };
const fail = (text) => { err.textContent = text; err.hidden = !text; };

function setBusy(b) {
  busy = b;
  for (const x of [exportBtn, importBtn, pins]) x.disabled = !!b;
}

function reset() {
  picked = null;
  confirmBox.hidden = true;
  pick.value = '';
}

function start(kind) {
  if (needsPin()) { demandPin(); fail('Enter the settings PIN first.'); return 0; }
  fail('');
  setBusy({ id: ++seq, kind });
  return seq;
}

// A refusal for the PIN asks for it again; anything else is shown as it is.
function failed(m) {
  setBusy(null);
  say('');
  if (m.locked) demandPin();
  fail(sentence(m.err) || 'The server refused.');
}

// ---- export

exportBtn.addEventListener('click', () => {
  reset();
  const id = start('export');
  if (!id) return;
  say('Preparing the file…');
  if (!wsSend({ t: 'configExport', id, pins: pins.checked })) {
    setBusy(null);
    say('');
    fail('Not connected to Sode.');
  }
});

onMsg('configExportResult', (m) => {
  if (!busy || busy.kind !== 'export' || m.id !== busy.id) return;
  if (!m.ok) return failed(m);
  setBusy(null);
  const file = fileName({ now: new Date(), site: m.name });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(new Blob([m.yaml], { type: 'application/yaml' }));
  a.download = file;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(a.href), 10000);
  say(`Saved ${file}${pins.checked ? ', with PINs. Keep it secret.' : '.'}`);
});

// ---- import

importBtn.addEventListener('click', () => {
  reset();
  fail('');
  say('');
  if (needsPin()) { demandPin(); fail('Enter the settings PIN first.'); return; }
  pick.click();
});

pick.addEventListener('change', async () => {
  const f = pick.files?.[0];
  if (!f) return;
  const text = await f.text().catch(() => '');
  const bytes = new TextEncoder().encode(text).length;
  const problem = fileProblem(bytes, text);
  if (problem) { reset(); return fail(problem); }
  picked = { name: f.name, text, bytes };
  confirmText.textContent = `Replace the whole server configuration with ${f.name} (${sizeText(bytes)})? `
    + 'Every setting, user, lock, filter set, preset and profile is replaced by the file\'s, for everyone. '
    + 'A PIN the file leaves out stays as it is. The settings form closes and reloads. This cannot be undone.';
  confirmBox.hidden = false;
  $('config-keep').focus();
});

$('config-keep').addEventListener('click', () => { reset(); say('Import cancelled.'); importBtn.focus(); });

$('config-replace').addEventListener('click', () => {
  if (!picked) return;
  const id = start('import');
  if (!id) return;
  const { bytes, name } = picked;
  confirmBox.hidden = true;
  say(`Importing ${name}…`);
  // The begin message raises the server's read limit for the one message
  // after it (only for the settings PIN). The file goes out only once the
  // server has answered it: a refused begin must never be followed by a big
  // message, which would close the socket instead of showing the refusal.
  if (!wsSend({ t: 'configImportBegin', id, size: bytes })) {
    setBusy(null);
    say('');
    fail('Not connected to Sode.');
  }
});

onMsg('configImportBeginResult', (m) => {
  if (!busy || busy.kind !== 'import' || m.id !== busy.id) return;
  if (!m.ok) { reset(); return failed(m); }
  if (!picked || !wsSend({ t: 'configImport', id: busy.id, yaml: picked.text })) {
    reset();
    setBusy(null);
    say('');
    fail('Not connected to Sode.');
  }
});

onMsg('configImportResult', (m) => {
  if (!busy || busy.kind !== 'import' || m.id !== busy.id) return;
  if (!m.ok) { reset(); return failed(m); }
  const name = picked?.name || 'the file';
  reset();
  setBusy(null);
  say('');
  // The server has sent the new settings (hello). Close Settings so the open
  // form can't be saved back over them; a second Exit discards an unsaved
  // form's edits (they are for the old settings).
  const root = document.documentElement;
  for (let i = 0; i < 2 && root.dataset.view === 'settings'; i++) $('settings-exit').click();
  showResult('ok', 'Configuration imported', `Everyone now has the settings from ${name}.`);
});

// A dropped link loses the answer: say so rather than wait.
on('conn', () => {
  if (S.wsState === 'up' || !busy) return;
  const kind = busy.kind;
  setBusy(null);
  say('');
  fail(kind === 'import'
    ? 'The connection dropped before the answer: check Settings to see whether the import was applied.'
    : 'The connection dropped.');
});

// Settings closing or relocking leaves nothing half-done on screen.
on('access', () => { if (needsPin()) reset(); });
on('locked', () => reset());
box.hidden = false;
