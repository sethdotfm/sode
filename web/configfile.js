// Settings > Server backup. Export and Import are settings-PIN actions: pinprompt.js asks first, and again with a retry if the server refuses.
// The server makes and reads the file; this only moves it.

import { $, S, on, emit, onMsg, wsSend, sentence } from './core.js';
import { needsPin, unlocked } from './pin.js';
import { askPin, reask } from './pinprompt.js';
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

const say = (text) => {
  status.textContent = text;
};
const fail = (text) => {
  err.textContent = text;
  err.hidden = !text;
};

function setBusy(b) {
  busy = b;
  for (const x of [exportBtn, importBtn, pins]) x.disabled = !!b;
}

function reset() {
  picked = null;
  confirmBox.hidden = true;
  pick.value = '';
}

async function start(kind) {
  fail('');
  const what = kind === 'export' ? 'Exporting' : 'Importing';
  if (S.pinSet && !unlocked() && !(await askPin(`${what} the configuration needs the Advanced Settings PIN.`))) return 0;
  setBusy({ id: ++seq, kind });
  return seq;
}

function failed(m, retry) {
  setBusy(null);
  say('');
  if (m.locked) {
    reask('The Advanced Settings PIN is needed again.').then((ok) => {
      if (ok) retry();
    });
    return;
  }
  fail(sentence(m.err) || 'The server refused.');
}

// ---- export

async function runExport() {
  reset();
  const id = await start('export');
  if (!id) return;
  say('Preparing the file…');
  if (!wsSend({ t: 'configExport', id, pins: pins.checked })) {
    setBusy(null);
    say('');
    fail('Not connected to Sode.');
  }
}
exportBtn.addEventListener('click', runExport);

onMsg('configExportResult', (m) => {
  if (!busy || busy.kind !== 'export' || m.id !== busy.id) return;
  if (!m.ok) return failed(m, runExport);
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
  pick.click(); // the PIN is asked for when the replace is confirmed
});

pick.addEventListener('change', async () => {
  const f = pick.files?.[0];
  if (!f) return;
  const text = await f.text().catch(() => '');
  const bytes = new TextEncoder().encode(text).length;
  const problem = fileProblem(bytes, text);
  if (problem) {
    reset();
    return fail(problem);
  }
  picked = { name: f.name, text, bytes };
  confirmText.textContent =
    `Replace the whole server configuration with ${f.name} (${sizeText(bytes)})? ` +
    "Every setting, user, lock, filter set, preset and profile is replaced by the file's, for everyone. " +
    'A PIN the file leaves out stays as it is. The settings form closes and reloads. This cannot be undone.';
  confirmBox.hidden = false;
  $('config-keep').focus();
});

$('config-keep').addEventListener('click', () => {
  reset();
  say('Import cancelled.');
  importBtn.focus();
});

async function runImport() {
  if (!picked) return;
  const id = await start('import');
  if (!id) return;
  if (!picked) {
    // a lock or a PIN change reset the block while the PIN popup was open
    setBusy(null);
    return;
  }
  const { bytes, name } = picked;
  confirmBox.hidden = true;
  say(`Importing ${name}…`);
  // The begin message raises the server's read limit for the next message. Send the file only after begin is answered:
  // a refused begin followed by a big message would close the socket instead of showing the refusal.
  if (!wsSend({ t: 'configImportBegin', id, size: bytes })) {
    setBusy(null);
    say('');
    fail('Not connected to Sode.');
  }
}
$('config-replace').addEventListener('click', runImport);

onMsg('configImportBeginResult', (m) => {
  if (!busy || busy.kind !== 'import' || m.id !== busy.id) return;
  if (!m.ok) {
    if (!m.locked) reset();
    return failed(m, runImport);
  }
  if (!picked || !wsSend({ t: 'configImport', id: busy.id, yaml: picked.text })) {
    reset();
    setBusy(null);
    say('');
    fail('Not connected to Sode.');
  }
});

onMsg('configImportResult', (m) => {
  if (!busy || busy.kind !== 'import' || m.id !== busy.id) return;
  if (!m.ok) {
    if (!m.locked) reset();
    return failed(m, runImport);
  }
  const name = picked?.name || 'the file';
  reset();
  setBusy(null);
  say('');
  // New settings arrived: close Settings (discarding the form) so it can't be saved back over them.
  emit('settingsClose', { discard: true });
  const kept = m.kept ? ` It also holds settings this release doesn't know, kept as they are: ${m.kept}.` : '';
  showResult('ok', 'Configuration imported', `Everyone now has the settings from ${name}.${kept}`);
});

// A dropped link loses the answer: say so rather than wait.
on('conn', () => {
  if (S.wsState === 'up' || !busy) return;
  const kind = busy.kind;
  setBusy(null);
  say('');
  fail(kind === 'import' ? 'The connection dropped before the answer: check Settings to see whether the import was applied.' : 'The connection dropped.');
});

on('access', () => {
  if (needsPin()) reset();
});
on('locked', () => reset());
box.hidden = false;
