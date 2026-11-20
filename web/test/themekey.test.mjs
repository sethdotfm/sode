import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';

const web = new URL('../', import.meta.url);
const read = (f) => readFileSync(new URL(f, web), 'utf8');

test('one module binds the theme key, every page loads it, and each key list shows it once', () => {
  const binds = readdirSync(web).filter((f) => f.endsWith('.js') && /alt\+shift\+t'|KeyT/.test(read(f)));
  assert.deepEqual(binds, ['themekey.js']);
  for (const entry of ['main.js', 'log.js', 'panelpage.js']) assert.match(read(entry), /import '\.\/themekey\.js';/, entry);
  for (const page of ['index.html', 'panel.html']) assert.equal(read(page).split('data-key="alt+shift+t"').length - 1, 1, page);
});

test('a panel with a stored theme marks the page, and the key leaves it and a Kiosk panel alone', () => {
  assert.match(read('panelrun.js'), /toggleAttribute\('data-sode-panel-theme', t !== 'auto'\)/);
  const key = read('themekey.js');
  assert.match(key, /hasAttribute\('data-sode-kiosk'\)/);
  assert.match(key, /hasAttribute\('data-sode-panel-theme'\)/);
});
