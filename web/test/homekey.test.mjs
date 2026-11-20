import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { HOME_CHORD, homeKeyAct, homeStep } from '../homekeyrules.js';
import { chord } from '../keys.js';

const web = join(dirname(fileURLToPath(import.meta.url)), '..');
const read = (f) => readFileSync(join(web, f), 'utf8');

test('the Home key is the physical H with Alt and Shift, also where macOS Option types another character', () => {
  const key = (o) => chord({ key: 'H', code: 'KeyH', ...o });
  assert.equal(key({ altKey: true, shiftKey: true }), HOME_CHORD);
  assert.equal(key({ key: 'Ó', altKey: true, shiftKey: true }), HOME_CHORD);
  assert.notEqual(key({ ctrlKey: true }), HOME_CHORD);
  assert.notEqual(key({ ctrlKey: true, shiftKey: true }), HOME_CHORD);
  assert.notEqual(key({ altKey: true }), HOME_CHORD);
  assert.notEqual(key({ altKey: true, shiftKey: true, ctrlKey: true }), HOME_CHORD);
  assert.notEqual(key({ altKey: true, shiftKey: true, metaKey: true }), HOME_CHORD);
  assert.notEqual(chord({ key: 'Home', code: 'Home', altKey: true }), HOME_CHORD);
});

test('homeKeyAct: Home goes back to its plain view, every other page opens Home', () => {
  assert.equal(homeKeyAct({ page: 'main', refusal: '' }), 'here');
  for (const page of ['log', 'panels', 'editor', 'panel']) assert.equal(homeKeyAct({ page, refusal: '' }), 'go', page);
});

test('homeKeyAct: the Palette refusals do nothing on every page', () => {
  for (const refusal of ['lock', 'kiosk', 'setup'])
    for (const page of ['main', 'log', 'panels', 'editor', 'panel']) assert.equal(homeKeyAct({ page, refusal }), 'none', `${refusal} ${page}`);
});

// Runs the steps as homeview.js does, with a page that answers each step.
function press(page) {
  const tried = new Set();
  const ran = [];
  for (let i = 0; i < 10; i++) {
    const step = homeStep({ dialog: page.dialog, settings: page.settings, unsaved: page.unsaved, tried });
    ran.push(step);
    if (step === 'stop' || step === 'focus') return ran;
    tried.add(step === 'exitSettings' ? 'settings' : 'dialogs');
    page[step]();
  }
  throw new Error('no end');
}

test('homeStep: with nothing open the focus goes to Source', () => {
  assert.deepEqual(press({ dialog: false, settings: false, unsaved: false }), ['focus']);
});

test('homeStep: dialogs with no edits close at once, then Settings exits, then the focus goes to Source', () => {
  const page = { dialog: true, settings: true, unsaved: false };
  page.closeDialogs = () => (page.dialog = false);
  page.exitSettings = () => (page.settings = false);
  assert.deepEqual(press(page), ['closeDialogs', 'exitSettings', 'focus']);
});

test('homeStep: a dialog with edits gets its own Esc and stops on its warning; the second press discards', () => {
  let warned = false;
  const page = { dialog: true, settings: false, unsaved: true };
  page.escDialogs = () => {
    if (warned) page.dialog = page.unsaved = false;
    warned = true;
  };
  assert.deepEqual(press(page), ['escDialogs', 'stop']);
  assert.equal(page.dialog, true);
  assert.deepEqual(press(page), ['escDialogs', 'focus']);
});

test('homeStep: Settings with unsaved changes warns on the first press and closes on the second', () => {
  let warned = false;
  const page = { dialog: false, settings: true, unsaved: true };
  page.exitSettings = () => {
    if (warned) page.settings = page.unsaved = false;
    warned = true;
  };
  assert.deepEqual(press(page), ['exitSettings', 'stop']);
  assert.deepEqual(press(page), ['exitSettings', 'focus']);
});

test('one module binds the Home key, every page loads it, and each key list shows it once', () => {
  const binds = readdirSync(web).filter((f) => f.endsWith('.js') && !f.endsWith('rules.js') && /alt\+shift\+h'|HOME_CHORD/.test(read(f)));
  assert.deepEqual(binds, ['homekey.js']);
  for (const entry of ['main.js', 'log.js', 'panelpage.js']) assert.match(read(entry), /import '\.\/homekey\.js';/, entry);
  assert.match(read('main.js'), /import '\.\/homeview\.js';/);
  for (const page of ['index.html', 'panel.html']) assert.equal(read(page).split('>Alt+Shift+h</kbd>').length - 1, 1, page);
});
