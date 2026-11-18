// The Authentication mode a blank new panel starts with (Settings > Access > Panels). Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { newPanelActs } from '../panelsrules.js';

test('a blank new panel takes the new-panel mode', () => {
  for (const m of ['public', 'session', 'both', 'api']) assert.equal(newPanelActs({ newPanelActs: m, publicPanels: true }), m);
});

test('a Public default starts as User while Public panels are not allowed', () => {
  assert.equal(newPanelActs({ newPanelActs: 'public', publicPanels: false }), 'session');
  assert.equal(newPanelActs({ newPanelActs: 'api', publicPanels: false }), 'api');
});

test('before the hello, or from an older server, a new panel is Public as the server default', () => {
  assert.equal(newPanelActs(null), 'public');
  assert.equal(newPanelActs({}), 'public');
});
