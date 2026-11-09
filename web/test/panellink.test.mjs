// The one place that builds a panel's link text (panellink.js). Pure.
import test from 'node:test';
import assert from 'node:assert/strict';
import { linkFor } from '../panellink.js';

const base = 'https://sode.example/';

test('a panel links to its id URL under the Sode root', () => {
  assert.equal(linkFor({ id: 'p-1' }, { base }), 'https://sode.example/panels/p-1');
  assert.equal(linkFor({ id: 'p-1' }, { base: 'https://sode.example/sode/' }), 'https://sode.example/sode/panels/p-1');
});

test('kiosk adds the fragment that hides the header', () => {
  assert.equal(linkFor({ id: 'p-1' }, { base, kiosk: true }), 'https://sode.example/panels/p-1#kiosk=1');
});
